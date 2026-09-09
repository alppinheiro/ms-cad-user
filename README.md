# ms-cad-user

Projeto de estudo **profissional** de um pipeline de cadastro de usuários em alta vazão:

```
cmd/generator ──(Sarama AsyncProducer)──▶ Apache Kafka ──▶ cmd/worker ──▶ MongoDB
(genera N usuários pt-BR,     topic cad-user.created      (consumer group,    (bulk upsert
 determinístico por seed)         6 partições              batch 1k)           por CPF)
```

**Objetivo de estudo:** gerar milhares de usuários por segundo → Kafka → worker →
MongoDB, e depois praticar consultas no Mongo com volume realista.

## Stack

| Camada | Tecnologia |
|---|---|
| Linguagem | Go 1.26 |
| Kafka | `apache/kafka:3.9.0` (KRaft, single-node, 6 partições) + **IBM/sarama** |
| MongoDB | `mongo:7.0` (coleção `users`, bulk upsert `ordered:false`) |
| GUIs | Mongo Express (`:8081`) e Kafka UI (`:9085`) |
| Orquestração | docker-compose + Colima (macOS) |

## Estrutura

```
ms-cad-user/
├── cmd/
│   ├── generator/main.go     # carga em massa → Kafka  (make load)
│   └── worker/main.go        # consumer group → Mongo (docker-compose)
├── internal/
│   ├── domain/               # Usuario, CPF (algoritmo real), envelope, dados sintéticos pt-BR
│   ├── config/               # configuração 12-factor via env
│   ├── kafka/                # produtor idempotente + consumer em lote (sarama)
│   └── mongodb/              # conexão, índices, upsert por CPF
├── scripts/mongo/            # caderno de consultas de estudo
├── docker-compose.yml
├── .env.example
├── Dockerfile                # multi-target (TARGET=cmd/xxx)
└── Makefile
```

## Modelo do documento (coleção `users`)

```jsonc
{
  "_id": "uuid",                    // idempotente, definido no $setOnInsert
  "userId": "uuid",                 // id gerado pela mesma seed (determinístico)
  "nome": "Maria", "sobrenome": "Silva",
  "email": "maria.silva1234@gmail.com",
  "cpf": "52998224725",             // índice ÚNICO (chave de idempotência)
  "rg": "12.345.678-9",
  "sexo": "F",
  "dataNascimento": ISODate(...),   // 18–80 anos
  "telefones": [{ "ddd": "11", "numero": "99888-7777", "tipo": "CELULAR",
                  "whatsapp": true, "principal": true }],
  "endereco": { "logradouro": "...", "numero": "123", "complemento": "",
                "bairro": "...", "cep": "01310-100", "cidade": "São Paulo", "uf": "SP" },
  "status": "ATIVO",
  "origem": "loader",
  "createdAt": ISODate(...), "updatedAt": ISODate(...)
}
```

**Índices criados automaticamente pelo worker:**
`uniq_cpf` (único), `uniq_userid` (único), `idx_uf_cidade`, `idx_sexo_nascimento`,
`idx_created_at`, `idx_email`.

## Quick Start

```bash
# 1) ambiente Docker (Colima)
colima start          # (se ainda não estiver rodando)

# 2) subir infra + worker
cd ms-cad-user
cp .env.example .env   # já existe um .env pronto
make up                # ou: docker-compose up -d --build

# 3) conferir as GUIs
#    Mongo Express: http://localhost:8081   (admin/admin)
#    Kafka UI:      http://localhost:9085

# 4) gerar uma carga (host) — ex.: 1.000 usuários a 500/s
make load TOTAL=1000 RATE=500

# 5) inspecionar
docker-compose logs -f worker   # throughput do worker
make mongo-shell                # count({}) dentro do mongosh
make topics / make consume
```

Para a carga completa de estudo (500 mil): `make load` (usa `LOAD_TOTAL`/`LOAD_RATE` do `.env`).
Reexecutar com a mesma seed não duplica: o upsert por CPF atualiza os registros.

## Decisões de arquitetura

- **at-least-once + idempotência**: produtor Sarama idempotente (`acks=all`,
  `retries`, `max.in.flight=1`) + consumo em lote com auto-commit + **upsert no
  Mongo por CPF** (índice único). Reentrega/reexecução = update, nunca duplicata.
- **Sem Redis nesta fase**: a gravação no Mongo é o objetivo final e o upsert
  com índice único já garante a correção; Redis (dedupe por `event_id`) fica
  como evolução futura.
- **Particionamento por chave (CPF)**: ordem por usuário preservada por partição.
- **Consumer em lote**: `WORKER_BATCH_SIZE=1000` ou `WORKER_FLUSH_INTERVAL=1s` →
  `bulkWrite ordered:false` para acompanhar milhares de mensagens/segundo.
- **Falhas**: batch com retry exponencial (`WORKER_RETRY_MAX`); ao esgotar, a
  mensagem é registrada como *dead* (futuro: tópico `cad-user.dlq`).

## Comandos úteis (Makefile)

| Alvo | Ação |
|---|---|
| `make up / infra / worker` | sobe stack completa / só infra / só worker |
| `make down / reset` | derruba / derruba **apagando volumes** |
| `make load TOTAL=10000 RATE=2000` | gera carga no host |
| `make run-worker` | roda o worker no host (fora do container) |
| `make mongo-shell / topics / consume` | inspeções |
| `make check` | gofmt + build + vet + test |

> Uso no host (`go run`): as envs do `.env` apontam para as portas publicadas
> (`KAFKA_BROKERS=localhost:9095`, `MONGODB_URI=mongodb://localhost:27017`).
> Dentro do compose, o worker usa a rede interna (`kafka:9092`, `mongodb:27017`).

## Próximas fases (estudo)

1. **Fase 3 — Estudo de consultas**: popular 500k usuários e usar
   `scripts/mongo/consultas_estudo.js` (filtros, agregações, `explain`).
2. **Fase 4 — Evoluções**: API REST de cadastro/consulta, DLQ real, Redis para
   dedup, observabilidade (Prometheus/Grafana/OpenTelemetry), testes de
   integração com Testcontainers, CI.
