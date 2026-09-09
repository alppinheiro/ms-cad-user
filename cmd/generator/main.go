// Command generator publica em massa eventos user.created no Kafka.
// Uso:  make load   (ou go run ./cmd/generator -total 500000 -rate 2000 -seed 42)
// Os dados são sintéticos porém realistas (pt-BR) e determinísticos por seed.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ms-cad-user/internal/config"
	"ms-cad-user/internal/domain"
	kafkainfra "ms-cad-user/internal/kafka"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	cfg := config.Load()

	var (
		total   = flag.Int64("total", envInt64("LOAD_TOTAL", 500000), "quantidade de usuários a gerar")
		rate    = flag.Int64("rate", envInt64("LOAD_RATE", 2000), "mensagens por segundo (0 = sem limite)")
		seed    = flag.Int64("seed", 42, "seed para reprodutibilidade (mesmo seed => mesmos dados)")
		origin  = flag.String("origin", "loader", "origem registrada nos documentos")
		topic   = flag.String("topic", "", "tópico de destino (default: KAFKA_TOPIC)")
		brokers = flag.String("brokers", "", "brokers Kafka (default: KAFKA_BROKERS)")
	)
	flag.Parse()
	if *total <= 0 {
		slog.Error("informe -total maior que zero")
		flag.Usage()
		os.Exit(2)
	}

	if *topic == "" {
		*topic = cfg.Topic
	}
	brokerList := cfg.KafkaBrokers
	if *brokers != "" {
		brokerList = strings.Split(*brokers, ",")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	prod, err := kafkainfra.NewProducer(brokerList, *topic)
	if err != nil {
		slog.Error("não foi possível abrir o produtor", "erro", err)
		os.Exit(1)
	}

	gen := domain.NewGenerator(*seed, *origin)
	start := time.Now()
	doneMetrics := make(chan struct{})

	// Métricas ao vivo a cada segundo.
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		last := int64(0)
		for {
			select {
			case <-tick.C:
				ok, fail := prod.Counters()
				rateNow := ok + fail - last
				last = ok + fail
				slog.Info("publicando...",
					"confirmadas", ok,
					"erros", fail,
					"rate_1s", rateNow,
					"decorrido_s", int(time.Since(start).Seconds()),
				)
			case <-doneMetrics:
				return
			}
		}
	}()

	step := time.Duration(0)
	if *rate > 0 {
		step = time.Second / time.Duration(*rate)
	}
	enviadas := int64(0)
	interrompido := false

loop:
	for enviadas < *total {
		select {
		case <-ctx.Done():
			interrompido = true
			break loop
		default:
		}

		u := gen.Next()
		env := domain.NewUserCreatedEnvelope(u)
		payload, err := json.Marshal(env)
		if err != nil {
			slog.Error("erro ao serializar evento", "erro", err)
			continue
		}
		// Key = CPF: particionamento hash garante mesma partição por usuário.
		prod.Publish([]byte(u.CPF), payload)
		enviadas++

		if step > 0 {
			next := start.Add(time.Duration(enviadas) * step)
			if wait := time.Until(next); wait > 0 {
				time.Sleep(wait)
			}
		}
	}

	close(doneMetrics)
	if err := prod.Close(); err != nil { // aguarda flush completo
		slog.Error("erro ao fechar produtor", "erro", err)
		os.Exit(1)
	}

	ok, fail := prod.Counters()
	elapsed := time.Since(start)
	slog.Info("resumo do generator",
		"enviadas", enviadas,
		"geradas", gen.Count(),
		"confirmadas", ok,
		"erros", fail,
		"duracao", elapsed.Round(time.Millisecond),
		"throughput_msgs_s", int(float64(ok)/elapsed.Seconds()),
	)
	if interrompido {
		slog.Warn("execução interrompida pelo usuário")
	}
	if fail > 0 || ok != enviadas {
		slog.Error("entrega incompleta: verifique o Kafka acima")
		os.Exit(1)
	}
	slog.Info("carga concluída com sucesso")
}

// envInt64 lê default numérico de variável de ambiente.
func envInt64(key string, def int64) int64 {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return def
}
