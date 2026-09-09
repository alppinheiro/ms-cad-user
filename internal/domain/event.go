package domain

import (
	"time"

	"github.com/google/uuid"
)

// Eventos publicados no Kafka.
const (
	EventUserCreated = "user.created"
	SchemaVersion    = 1
)

// Envelope é o contrato da mensagem do Kafka. Todo evento carrega metadados
// mínimos para rastreabilidade (idempotência por event_id, versionamento e
// auditoria) + o payload de negócio.
type Envelope struct {
	EventID       string    `json:"event_id"`
	EventType     string    `json:"event_type"`
	SchemaVersion int       `json:"schema_version"`
	OccurredAt    time.Time `json:"occurred_at"`
	Payload       User      `json:"payload"`
}

// NewUserCreatedEnvelope monta o envelope padrão de criação de usuário.
func NewUserCreatedEnvelope(u User) Envelope {
	return Envelope{
		EventID:       uuid.NewString(),
		EventType:     EventUserCreated,
		SchemaVersion: SchemaVersion,
		OccurredAt:    time.Now().UTC(),
		Payload:       u,
	}
}

// NewID gera um identificador único universal (v4).
func NewID() string {
	return uuid.NewString()
}
