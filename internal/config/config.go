// Package config carrega a configuração da aplicação a partir de variáveis de
// ambiente (padrão 12-factor). Os mesmos binários funcionam no docker-compose
// (rede interna) e no host (portas publicadas) — basta variar as envs.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config reúne toda a configuração lida do ambiente.
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

// Load lê as variáveis de ambiente aplicando defaults sensatos.
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

// env retorna o valor da variável ou o default quando vazia.
func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil || v <= 0 {
		return def
	}
	return v
}

func envDur(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
}

// splitCSV separa uma lista de brokers "a:9092,b:9092" em fatia.
func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
