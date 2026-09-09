// Package kafka encapsula produção e consumo com IBM/sarama (cliente de
// mercado). O produtor é assíncrono, idempotente (acks=all, retries) e
// particiona por chave (CPF) para garantir ordem por usuário.
package kafka

import (
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/IBM/sarama"
)

// Producer é um wrapper do AsyncProducer do Sarama com contadores atômicos.
type Producer struct {
	ap        sarama.AsyncProducer
	topic     string
	done      chan struct{}
	successes atomic.Int64
	failures  atomic.Int64
}

// NewProducer configura e abre um produtor assíncrono idempotente. Cada opção
// abaixo foi escolhida pensando em throughput ALTO com durabilidade:
//
//   - RequiredAcks = WaitForAll (acks=all): o broker só confirma depois de
//     gravar no líder — troca um pouco de latência por zero perda em failover
//     (equivalente ao que sistemas de produção usam por padrão);
//   - Idempotent = true: o PRODUTOR não injeta duplicata quando há retry
//     (protocolo KIP-98). Exige acks=all e no máx. 1 requisição em voo
//     (MaxOpenRequests=1) — por isso ambos aparecem juntos;
//   - Retry.Max/Backoff: reenvio automático com backoff; combinado com a
//     idempotência, um timeout de rede não gera mensagem duplicada;
//   - Return.Successes = true: recebemos confirmação por mensagem (usada para
//     medir throughput real e saber quando tudo foi entregue — essencial no
//     generator para garantir "carga 100% confirmada");
//   - Compression = Snappy: menos bytes na rede/disco com CPU baixa (gzip
//     comprime mais, porém custa mais CPU — para mensagens pequenas Snappy é o
//     melhor custo/benefício);
//   - Flush.Messages/Frequency: acumula até 500 msgs ou 250ms antes de enviar
//     → muito mais eficiente que 1 request por mensagem;
//   - Partitioner = NewHashPartitioner: usa o HASH da CHAVE (CPF) para escolher
//     a partição → o mesmo CPF cai sempre na mesma partição (ordem garantida
//     por usuário e reprocessamento previsível).
func NewProducer(brokers []string, topic string) (*Producer, error) {
	cfg := sarama.NewConfig()

	// Garantias de entrega "de mercado": idempotência de produtor requer
	// acks=all, retries e no máximo 1 requisição em voo.
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Idempotent = true
	cfg.Net.MaxOpenRequests = 1
	cfg.Producer.Retry.Max = 10
	cfg.Producer.Retry.Backoff = 300 * time.Millisecond

	// Throughput: lote a cada 500 mensagens ou 250ms + confirmações contadas.
	cfg.Producer.Return.Successes = true
	cfg.Producer.Compression = sarama.CompressionSnappy
	cfg.Producer.Flush.Messages = 500
	cfg.Producer.Flush.Frequency = 250 * time.Millisecond

	// Hash por chave (CPF) => mesma partição para o mesmo usuário.
	cfg.Producer.Partitioner = sarama.NewHashPartitioner

	ap, err := sarama.NewAsyncProducer(brokers, cfg)
	if err != nil {
		return nil, fmt.Errorf("sarama async producer: %w", err)
	}
	p := &Producer{ap: ap, topic: topic, done: make(chan struct{})}
	go p.drain()
	return p, nil
}

// drain contabiliza confirmações e erros até o produtor ser fechado.
// Importante: drena CADA canal até fechar (range), senão o encerramento de um
// canal (ex.: Errors) faria o dreno terminar antes de contar os Successes.
func (p *Producer) drain() {
	defer close(p.done)
	for range p.ap.Successes() {
		p.successes.Add(1)
	}
	for err := range p.ap.Errors() {
		p.failures.Add(1)
		slog.Error("falha ao publicar mensagem", "erro", err.Err, "topico", err.Msg.Topic)
	}
}

// Publish envia key/value para o tópico do produtor. O envio é feito pelo canal
// Input() do AsyncProducer: se o buffer interno estiver cheio (Kafka mais lento
// que o produtor), esta chamada BLOQUEIA — isso é proposital, pois cria
// backpressure natural: o gerador não "atropela" o broker nem estoura memória
// (em vez de descartar mensagens silenciosamente).
func (p *Producer) Publish(key, value []byte) {
	p.ap.Input() <- &sarama.ProducerMessage{
		Topic: p.topic,
		Key:   sarama.ByteEncoder(key),
		Value: sarama.ByteEncoder(value),
	}
}

// Counters retorna (publicadas com sucesso, falhas).
func (p *Producer) Counters() (successes, failures int64) {
	return p.successes.Load(), p.failures.Load()
}

// Close aguarda o flush de todas as mensagens pendentes, o encerramento dos
// canais e o fim do dreno de contadores. A ordem importa:
//  1. ap.Close() → Sarama para de aceitar mensagens e aguarda o envio/flush;
//  2. <-p.done  → só depois o dreno terminou de contar Successes/Errors.
//
// Sem o passo 2, o generator leria a contagem final ANTES de o dreno acabar
// (race que causava "confirmadas < enviadas" mesmo com entrega 100% ok).
func (p *Producer) Close() error {
	err := p.ap.Close()
	<-p.done
	return err
}
