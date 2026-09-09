COMPOSE ?= docker-compose

# Defaults (sobrescritos por .env quando presente)
-include .env
LOAD_TOTAL ?= 500000
LOAD_RATE ?= 2000
SEED ?= 42
# Serviços exibidos por `make logs` (padrão: todos os workers). Ex.: make logs SVC=worker
SVC ?= worker worker-2 worker-3
TOTAL ?= $(LOAD_TOTAL)
RATE ?= $(LOAD_RATE)
KAFKA_BROKERS ?= localhost:9095
MONGODB_URI ?= mongodb://localhost:27017

.PHONY: help fmt build vet test check up infra worker down ps logs logs-once \
	load run-worker mongo-shell topics consume reset

help:
	@echo "Targets disponíveis (ms-cad-user):"
	@echo "  make fmt          - formata o código (gofmt)"
	@echo "  make build        - compila todos os pacotes Go"
	@echo "  make vet          - roda go vet"
	@echo "  make test         - testes unitários com cobertura"
	@echo "  make check        - fmt + build + vet + test"
	@echo "  make up           - sobe a stack completa (infra + worker) com build"
	@echo "  make infra        - sobe apenas Mongo + Kafka + GUIs + kafka-init"
	@echo "  make worker       - (re)constrói e sobe só o worker"
	@echo "  make down         - derruba a stack (mantém volumes)"
	@echo "  make ps / logs    - status / logs em tempo real (SVC= para filtrar)"
	@echo "  make logs-once    - mostra o último estado dos logs e encerra"
	@echo "  make load         - roda o generator no host (TOTAL= RATE= SEED=)"
	@echo "  make run-worker   - roda o worker no host (go run)"
	@echo "  make mongo-shell  - mongosh dentro do container"
	@echo "  make topics       - lista os tópicos do Kafka"
	@echo "  make consume      - mostra 5 mensagens do tópico"
	@echo "  make reset        - derruba a stack apagando volumes (-v)"

fmt:
	gofmt -w .

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./... -cover

check: fmt build vet test

# ---------------------------------------------------------------------------
# Docker Compose
# ---------------------------------------------------------------------------
up:
	$(COMPOSE) up -d --build

infra:
	$(COMPOSE) up -d mongodb mongo-express kafka kafka-init kafka-ui

worker:
	$(COMPOSE) up -d --build worker

down:
	$(COMPOSE) down

ps:
	$(COMPOSE) ps

logs:
	$(COMPOSE) logs -f --tail=100 $(SVC)

logs-once:
	$(COMPOSE) logs --tail=20 $(SVC)

reset:
	$(COMPOSE) down -v

# ---------------------------------------------------------------------------
# Carga / execução no host (go run) — usa as portas publicadas do compose
# ---------------------------------------------------------------------------
load:
	KAFKA_BROKERS=$(KAFKA_BROKERS) \
	go run ./cmd/generator -total=$(TOTAL) -rate=$(RATE) -seed=$(SEED)

run-worker:
	KAFKA_BROKERS=$(KAFKA_BROKERS) MONGODB_URI=$(MONGODB_URI) \
	go run ./cmd/worker

# ---------------------------------------------------------------------------
# Ferramentas de inspeção
# ---------------------------------------------------------------------------
mongo-shell:
	$(COMPOSE) exec mongodb mongosh $(MONGO_DATABASE)

topics:
	$(COMPOSE) exec kafka /opt/kafka/bin/kafka-topics.sh \
		--bootstrap-server localhost:9092 --list

consume:
	$(COMPOSE) exec kafka /opt/kafka/bin/kafka-console-consumer.sh \
		--bootstrap-server localhost:9092 --topic $(KAFKA_TOPIC) \
		--from-beginning --max-messages 5
