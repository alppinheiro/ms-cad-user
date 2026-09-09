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
	var consumed, inserted, updated, dead atomic.Int64

	process := func(ctx context.Context, msgs []*sarama.ConsumerMessage) error {
		users := make([]domain.User, 0, len(msgs))
		for _, m := range msgs {
			var env domain.Envelope
			if err := json.Unmarshal(m.Value, &env); err != nil {
				dead.Add(1)
				slog.Error("mensagem inválida (json)", "erro", err, "offset", m.Offset)
				continue
			}
			if err := env.Payload.Validate(); err != nil {
				dead.Add(1)
				slog.Error("payload de usuário inválido (futuro: DLQ)",
					"erro", err, "event_id", env.EventID)
				continue
			}
			users = append(users, env.Payload)
		}

		stats, err := mongodb.UpsertUsers(ctx, mongoClient, cfg.MongoDatabase, users)
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
	cg, err := sarama.NewConsumerGroup(cfg.KafkaBrokers, cfg.ConsumerGroup, kafkainfra.ConsumerGroupConfig())
	if err != nil {
		slog.Error("não foi possível criar o consumer group", "erro", err)
		os.Exit(1)
	}
	defer cg.Close()

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

	time.Sleep(time.Second) // deixa o auto-commit gravar o último offset
	slog.Info("worker encerrado",
		"consumidos", consumed.Load(),
		"inseridos", inserted.Load(),
		"atualizados", updated.Load(),
		"mortos", dead.Load(),
	)
}

// metricsLoop loga a evolução do consumo a cada 3 segundos.
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
