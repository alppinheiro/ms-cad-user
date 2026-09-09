package domain

import (
	"time"

	"github.com/google/uuid"
)

// Eventos publicados no Kafka. Manter o nome do evento explícito (e não usar o
// nome do tópico) permite que um mesmo tópico transporte tipos diferentes no
// futuro e que o consumidor faça roteamento por event_type.
const (
	EventUserCreated = "user.created"
	SchemaVersion    = 1 // incrementar quando o payload mudar de forma incompatível
)

// Envelope é o CONTRATO da mensagem do Kafka. Cada evento carrega metadados
// mínimos + payload de negócio. Por que esses campos?
//   - event_id (uuid): idempotência e rastreabilidade — se a mesma mensagem for
//     entregue 2x (at-least-once), sabemos identificar a duplicata;
//   - event_type: qual evento aconteceu (rota no consumidor);
//   - schema_version: evolução segura do contrato (quem consome sabe qual
//     versão está lendo e pode migrar);
//   - occurred_at (UTC): quando o evento aconteceu de fato (auditoria) — não
//     confundir com o momento em que o worker persistiu (updatedAt no Mongo).
//
// Payload é o User completo. JSON foi a serialização escolhida para facilitar
// a leitura no Kafka UI (estudo); em produção, Avro/Protobuf + Schema Registry
// dariam contrato forte e payload menor (roadmap).
type Envelope struct {
	EventID       string    `json:"event_id"`
	EventType     string    `json:"event_type"`
	SchemaVersion int       `json:"schema_version"`
	OccurredAt    time.Time `json:"occurred_at"`
	Payload       User      `json:"payload"`
}

// NewUserCreatedEnvelope monta o envelope padrão de criação de usuário.
// O event_id usa uuid.NewString() (aleatório criptográfico) porque é ÚNICO por
// EVENTO — diferente do userId (que é determinístico por seed no generator
// apenas para tornar a carga reproduzível).
func NewUserCreatedEnvelope(u User) Envelope {
	return Envelope{
		EventID:       uuid.NewString(),
		EventType:     EventUserCreated,
		SchemaVersion: SchemaVersion,
		OccurredAt:    time.Now().UTC(),
		Payload:       u,
	}
}

// NewID gera um identificador único universal (v4). Mantido como utilitário do
// pacote para uso futuro (ex.: API REST gerando userId no cadastro online).
func NewID() string {
	return uuid.NewString()
}
