package kafka

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/IBM/sarama"
)

// ===========================================================================
// CONSUMIDOR EM LOTE (consumer group)
// ---------------------------------------------------------------------------
// O consumo é feito com o padrão de consumer group do Kafka (escalável:
// rodar N instâncias divide as partições entre elas) e em LOTE para conseguir
// acompanhar milhares de mensagens/segundo — gravar 1 documento por vez no
// Mongo não sustentaria a vazão do generator.
// Garantia usada: at-least-once (o Kafka pode reentregar) + idempotência no
// Mongo por CPF → reentrega vira update, nunca duplicata.
// ===========================================================================

// BatchProcessor processa um lote de mensagens do Kafka.
type BatchProcessor func(ctx context.Context, msgs []*sarama.ConsumerMessage) error

// BatchHandler consome mensagens em lotes (tamanho e/ou tempo), invocando o
// processador (ex.: bulk write no Mongo) e só então marcando os offsets.
// Comitamos via auto-commit do Sarama; a idempotência por CPF garante que uma
// reentrega pós-crash não duplica usuários no banco (at-least-once + upsert).
type BatchHandler struct {
	batchSize     int
	flushInterval time.Duration
	retryMax      int
	process       BatchProcessor

	consumed atomic.Int64 // mensagens lidas do Kafka
	batches  atomic.Int64 // lotes processados com sucesso
	dead     atomic.Int64 // mensagens que esgotaram retries
}

// NewBatchHandler cria o handler consumidor com os parâmetros de lote.
// Os ifs de guarda garantem valores utilizáveis mesmo se a config vier zerada
// (defesa contra env errada — ex.: WORKER_BATCH_SIZE=0).
func NewBatchHandler(batchSize int, flushInterval time.Duration, retryMax int, process BatchProcessor) *BatchHandler {
	if batchSize <= 0 {
		batchSize = 1000
	}
	if flushInterval <= 0 {
		flushInterval = time.Second
	}
	if retryMax <= 0 {
		retryMax = 1
	}
	return &BatchHandler{
		batchSize:     batchSize,
		flushInterval: flushInterval,
		retryMax:      retryMax,
		process:       process,
	}
}

// Setup prepara a sessão do consumer group.
func (h *BatchHandler) Setup(sarama.ConsumerGroupSession) error { return nil }

// Cleanup finaliza a sessão.
func (h *BatchHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim lê as mensagens de UMA partição acumulando em lotes.
// Três gatilhos disparam o flush do lote:
//  1. `len(buf) >= batchSize`  → throughput: encheu o lote, grava;
//  2. `<-ticker.C`             → latência: passou o flushInterval e sobrou
//     lote pequeno, grava mesmo assim (evita mensagem "encalhada" esperando
//     o lote encher — crítico no fim da carga);
//  3. `sess.Context().Done()`  → rebalance/shutdown: grava o que ficou e sai
//     para a sessão encerrar de forma limpa.
//
// O consumo de uma partição é sequencial (uma goroutine por partição no
// Sarama) — é isso que preserva a ordem por chave dentro da partição.
func (h *BatchHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	buf := make([]*sarama.ConsumerMessage, 0, h.batchSize)
	ticker := time.NewTicker(h.flushInterval)
	defer ticker.Stop()

	flush := func() error {
		if len(buf) == 0 {
			return nil
		}
		msgs := buf
		buf = make([]*sarama.ConsumerMessage, 0, h.batchSize)
		return h.processBatch(sess, msgs)
	}

	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return flush()
			}
			buf = append(buf, msg)
			if len(buf) >= h.batchSize {
				if err := flush(); err != nil {
					return err
				}
			}
		case <-ticker.C:
			if err := flush(); err != nil {
				return err
			}
		case <-sess.Context().Done():
			return flush()
		}
	}
}

// processBatch executa o processador com retries exponenciais.
//
// Retry com backoff (200ms * 2 a cada tentativa): erros de Mongo/infra costumam
// ser transitórios e o backoff dá tempo de o serviço se recuperar — retry
// imediato em loop só sobrecarregaria o banco.
// Ao final do processamento com sucesso, chamamos sess.MarkMessage para cada
// mensagem: isso marca o OFFSET como processado. O commit de fato ocorre via
// AUTO-COMMIT do Sarama (1s) — não precisamos commitar manualmente porque a
// idempotência por CPF torna seguro reprocessar o que eventualmente ficar
// sem commit (at-least-once + upsert).
// Esgotou os retries? Não travamos o consumer group (poison-pill travaria
// TODAS as partições): contabilizamos como "morto" e seguimos. O item DLQ do
// roadmap prevê publicar esses lotes em cad-user.dlq para análise posterior.
func (h *BatchHandler) processBatch(sess sarama.ConsumerGroupSession, msgs []*sarama.ConsumerMessage) error {
	h.consumed.Add(int64(len(msgs)))

	var err error
	delay := 200 * time.Millisecond
	for attempt := 1; attempt <= h.retryMax; attempt++ {
		err = h.process(sess.Context(), msgs)
		if err == nil {
			for _, m := range msgs {
				sess.MarkMessage(m, "")
			}
			h.batches.Add(1)
			return nil
		}
		if attempt < h.retryMax {
			slog.Warn("lote falhou; nova tentativa",
				"tentativa", attempt, "de", h.retryMax, "msgs", len(msgs), "erro", err)
			select {
			case <-time.After(delay):
			case <-sess.Context().Done():
				return err
			}
			delay *= 2
		}
	}

	// Esgotou os retries: registra como "dead letter" (futuro: publicar no
	// tópico cad-user.dlq) e segue — evita poison-pill travando o grupo.
	slog.Error("lote descartado após retries (futuro: DLQ)",
		"msgs", len(msgs), "erro", err)
	h.dead.Add(int64(len(msgs)))
	return nil
}

// Stats retorna (consumidas, lotes_ok, mortas).
func (h *BatchHandler) Stats() (consumed, batches, dead int64) {
	return h.consumed.Load(), h.batches.Load(), h.dead.Load()
}

// ConsumerGroupConfig devolve a configuração padrão do consumer group.
// Escolhas:
//   - Offsets.Initial = OffsetOldest: na PRIMEIRA vez (sem offset commitado) o
//     grupo começa do início do tópico — se subirmos o worker depois do
//     generator, ele processa o histórico em vez de pular as mensagens;
//     execuções seguintes retomam do último offset commitado;
//   - Return.Errors = true: expõe erros internos no canal cg.Errors() (o main
//     precisa drenar esse canal para logs);
//   - AutoCommit 1s: compromisso entre durabilidade (commits frequentes) e
//     custo (commits demais geram carga no __consumer_offsets);
//   - MaxProcessingTime = 30s: teto de segurança para um lote não travar a
//     sessão além do tempo que o Kafka considera o consumidor "vivo".
func ConsumerGroupConfig() *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.Consumer.Return.Errors = true
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	cfg.Consumer.Offsets.AutoCommit.Enable = true
	cfg.Consumer.Offsets.AutoCommit.Interval = time.Second
	cfg.Consumer.MaxProcessingTime = 30 * time.Second
	return cfg
}
