// Command worker consome eventos user.created do Kafka (consumer group) e
// persiste em lote no MongoDB com upsert por CPF (idempotente). O worker roda
// no docker-compose (restart: unless-stopped) ou via `go run ./cmd/worker`.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/IBM/sarama"

	"ms-cad-user/internal/config"
	"ms-cad-user/internal/domain"
	kafkainfra "ms-cad-user/internal/kafka"
	"ms-cad-user/internal/mongodb"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ---- MongoDB -----------------------------------------------------------
	// 1) Conecta e faz PING (fail-fast: banco fora = worker não sobe);
	// 2) Garante os ÍNDICES (cria se não existirem — idempotente). Fazemos isso
	//    ANTES de consumir para nunca gravar mensagem sem o índice único de CPF
	//    (sem ele, a idempotência não teria como ser garantida pelo banco).
	mongoClient, err := mongodb.Connect(ctx, cfg.MongoURI)
	if err != nil {
		slog.Error("não foi possível conectar ao MongoDB", "erro", err)
		os.Exit(1)
	}
	defer mongoClient.Disconnect(context.Background())

	if err := mongodb.EnsureIndexes(ctx, mongoClient, cfg.MongoDatabase); err != nil {
		slog.Error("não foi possível garantir índices do MongoDB", "erro", err)
		os.Exit(1)
	}
	slog.Info("MongoDB conectado e índices garantidos",
		"uri", cfg.MongoURI, "db", cfg.MongoDatabase, "colecao", cfg.MongoCollection)

	// ---- Contadores de ponta a ponta ----------------------------------------
	// atomic.Int64 porque são lidos/escritos por MÚLTIPLAS goroutines
	// simultaneamente: a do processamento (process) incrementa e a de métricas
	// (metricsLoop) lê. Sem atomic teríamos data race.
	var consumed, inserted, updated, dead atomic.Int64

	// process é o callback chamado pelo handler para CADA lote de mensagens.
	// Recebe mensagens CRUAS do Kafka e devolve o que o Mongo gravou.
	process := func(ctx context.Context, msgs []*sarama.ConsumerMessage) error {
		users := make([]domain.User, 0, len(msgs))
		for _, m := range msgs {
			// Passo 1: desserializa o envelope JSON (contrato do evento).
			var env domain.Envelope
			if err := json.Unmarshal(m.Value, &env); err != nil {
				// Mensagem que não é JSON do nosso contrato: não tem como
				// processar. Contabiliza como "morta" e segue (roadmap: DLQ).
				dead.Add(1)
				slog.Error("mensagem inválida (json)", "erro", err, "offset", m.Offset)
				continue
			}
			// Passo 2: valida as regras de negócio ANTES de tocar no banco.
			// Dado inválido não pode poluir o Mongo (ex.: CPF falso).
			if err := env.Payload.Validate(); err != nil {
				dead.Add(1)
				slog.Error("payload de usuário inválido (futuro: DLQ)",
					"erro", err, "event_id", env.EventID)
				continue
			}
			// Passo 3: payload válido entra no lote que será gravado em bulk.
			users = append(users, env.Payload)
		}

		// Passo 4: grava o lote inteiro de uma vez (bulk upsert por CPF).
		// Se der erro, retornamos para o handler aplicar o retry com backoff.
		stats, err := mongodb.UpsertUsers(ctx, mongoClient, cfg.MongoDatabase, users)
		// Contadores de auditoria: consumidos = tentados; inseridos/atualizados
		// mostram quantos eram novos vs reentregas (idempotência na prática).
		consumed.Add(int64(len(users)))
		inserted.Add(stats.Inserted)
		updated.Add(stats.Matched + stats.Modified)
		if err != nil {
			return err
		}
		return nil
	}

	handler := kafkainfra.NewBatchHandler(cfg.WorkerBatchSize, cfg.WorkerFlushInterval, cfg.WorkerRetryMax, process)

	// ---- Consumer group ------------------------------------------------------
	// Cria o consumer group (nome vem da env KAFKA_CONSUMER_GROUP). Se subirmos
	// uma SEGUNDA instância do worker, o Kafka divide as partições entre as
	// duas (escalabilidade horizontal sem mudança de código).
	cg, err := sarama.NewConsumerGroup(cfg.KafkaBrokers, cfg.ConsumerGroup, kafkainfra.ConsumerGroupConfig())
	if err != nil {
		slog.Error("não foi possível criar o consumer group", "erro", err)
		os.Exit(1)
	}
	defer cg.Close()

	// Drena o canal de ERROS do consumer group. Quando Return.Errors=true (ver
	// ConsumerGroupConfig), o Sarama exige que alguém consuma esse canal — se
	// não drenarmos, o cliente pode bloquear. Aqui apenas logamos.
	go func() {
		for err := range cg.Errors() {
			slog.Error("erro do consumer group", "erro", err)
		}
	}()

	slog.Info("worker iniciado",
		"group", cfg.ConsumerGroup, "topic", cfg.Topic,
		"brokers", cfg.KafkaBrokers,
		"batch", cfg.WorkerBatchSize, "flush", cfg.WorkerFlushInterval)

	metricsCtx, metricsStop := context.WithCancel(ctx)
	go metricsLoop(metricsCtx, &consumed, &inserted, &updated, &dead)
	defer metricsStop()

	// Loop de consumo: cg.Consume BLOQUEIA processando mensagens até ocorrer um
	// rebalance ou erro. Quando retorna sem erro, significa que houve
	// REBALANCE (partições redistribuídas) e devemos chamar Consume de novo
	// para continuar — por isso o loop. O sleep de 2s dá respiro antes de
	// tentar o próximo ciclo. Ao receber SIGTERM o contexto é cancelado e
	// saímos do loop para o shutdown gracioso.
	for {
		err := cg.Consume(ctx, []string{cfg.Topic}, handler)
		if err != nil {
			slog.Error("ciclo de consumo falhou; tentando novamente", "erro", err)
		}
		if ctx.Err() != nil {
			break
		}
		time.Sleep(2 * time.Second) // aguarda nova atribuição após rebalance
	}

	// Pequena espera antes de encerrar para o AUTO-COMMIT de offset (intervalo
	// de 1s) conseguir gravar o último offset processado — reduz reprocessamento
	// na próxima subida (que é seguro, por causa do upsert idempotente).
	time.Sleep(time.Second) // deixa o auto-commit gravar o último offset
	slog.Info("worker encerrado",
		"consumidos", consumed.Load(),
		"inseridos", inserted.Load(),
		"atualizados", updated.Load(),
		"mortos", dead.Load(),
	)
}

// metricsLoop loga a evolução do consumo a cada 3 segundos. Separado em
// função própria para manter o main enxuto; recebe ponteiros atômicos para os
// contadores compartilhados com a goroutine de processamento.
func metricsLoop(ctx context.Context, consumed, inserted, updated, dead *atomic.Int64) {
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			slog.Info("progresso do worker",
				"consumidos", consumed.Load(),
				"inseridos", inserted.Load(),
				"atualizados", updated.Load(),
				"mortos", dead.Load(),
			)
		case <-ctx.Done():
			return
		}
	}
}
