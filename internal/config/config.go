// Package config centraliza a leitura de configuração a partir de VARIÁVEIS DE
// AMBIENTE (padrão 12-factor). Por que assim?
//
//  1. Um único binário roda em contextos diferentes sem recompilar:
//     - dentro do docker-compose usa a rede interna (KAFKA_BROKERS=kafka:9092,
//     MONGODB_URI=mongodb://mongodb:27017);
//     - no host (go run / make load) usa as portas publicadas
//     (KAFKA_BROKERS=localhost:9095, MONGODB_URI=mongodb://localhost:27017).
//  2. Nada de segredo/ambiente fica "chumbado" no código ou versionado
//     (o arquivo .env é ignorado pelo git; só o .env.example vai para o repo).
//
// A leitura é feita UMA vez na inicialização (Load) — simples e suficiente para
// estes binários (não usamos biblioteca de config nem hot-reload propositalmente).
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config reúne toda a configuração lida do ambiente, agrupada por domínio:
//   - Kafka: endereços, tópicos e consumer group usados por produtor/consumidor;
//   - MongoDB: onde o worker persiste os usuários;
//   - Worker: parâmetros de lote/retry do consumo.
//
// Manter tudo em um struct único facilita injetar a configuração nas funções e
// escrever testes (basta montar a struct sem depender de env real).
type Config struct {
	// Kafka
	KafkaBrokers  []string
	Topic         string
	TopicDLQ      string
	ConsumerGroup string

	// MongoDB
	MongoURI        string
	MongoDatabase   string
	MongoCollection string

	// Worker (consumer)
	WorkerBatchSize     int
	WorkerFlushInterval time.Duration
	WorkerRetryMax      int
}

// Load lê as variáveis de ambiente aplicando defaults sensatos. Os defaults
// apontam para o CONTEXTO DE HOST (local de desenvolvimento), pois são os
// valores usados por `make load` / `go run`; dentro do compose o arquivo .env
// + as envs do serviço sobrescrevem com a rede interna.
func Load() Config {
	return Config{
		KafkaBrokers:        splitCSV(env("KAFKA_BROKERS", "localhost:9095")),
		Topic:               env("KAFKA_TOPIC", "cad-user.created"),
		TopicDLQ:            env("KAFKA_TOPIC_DLQ", "cad-user.dlq"),
		ConsumerGroup:       env("KAFKA_CONSUMER_GROUP", "cad-user-worker"),
		MongoURI:            env("MONGODB_URI", "mongodb://localhost:27017"),
		MongoDatabase:       env("MONGO_DATABASE", "cad_user"),
		MongoCollection:     env("MONGO_COLLECTION", "users"),
		WorkerBatchSize:     envInt("WORKER_BATCH_SIZE", 1000),
		WorkerFlushInterval: envDur("WORKER_FLUSH_INTERVAL", time.Second),
		WorkerRetryMax:      envInt("WORKER_RETRY_MAX", 5),
	}
}

// env retorna o valor da variável de ambiente ou o default quando ela está
// vazia. O TrimSpace evita que espaços acidentais (comuns ao editar .env)
// quebrem a configuração silenciosamente.
func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// envInt lê um inteiro POSITIVO da env. Qualquer valor ausente/inválido cai no
// default — validação na origem evita "0" ou negativo escorregando para dentro
// de loops/lotes (ex.: batchSize <= 0 quebraria o consumer).
func envInt(key string, def int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil || v <= 0 {
		return def
	}
	return v
}

// envDur lê uma DURAÇÃO no formato aceito por time.ParseDuration (ex.: "1s",
// "250ms"). Valores inválidos ou não-positivos usam o default.
func envDur(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}

// splitCSV separa uma lista de brokers "host1:9092,host2:9092" em uma fatia,
// descartando entradas vazias — assim o Kafka recebe uma lista válida mesmo
// com vírgulas sobrando no .env.
func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
