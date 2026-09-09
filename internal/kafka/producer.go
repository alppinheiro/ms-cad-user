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
	successes atomic.Int64
	failures  atomic.Int64
}

// NewProducer configura e abre um produtor assíncrono idempotente.
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
	p := &Producer{ap: ap, topic: topic}
	go p.drain()
	return p, nil
}

// drain contabiliza confirmações e erros até o produtor ser fechado.
func (p *Producer) drain() {
	for {
		select {
		case _, ok := <-p.ap.Successes():
			if !ok {
				return
			}
			p.successes.Add(1)
		case err, ok := <-p.ap.Errors():
			if !ok {
				return
			}
			p.failures.Add(1)
			slog.Error("falha ao publicar mensagem", "erro", err.Err, "topico", err.Msg.Topic)
		}
	}
}

// Publish envia key/value para o tópico do produtor (bloqueia sob backpressure).
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

// Close aguarda o flush de todas as mensagens pendentes e fecha os canais.
func (p *Producer) Close() error {
	return p.ap.Close()
}
