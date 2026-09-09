# Trabalho Multiagente — ms-cad-user

Guia para paralelizar o desenvolvimento usando **subagentes/custom agents** do
VS Code (Copilot). Os papéis estão em `.github/agents/*.agent.md`.

## Regra de ouro

Dois agentes **nunca** editam os mesmos arquivos ao mesmo tempo. Em caso de
dúvida sobre fronteiras, consulte a tabela abaixo. Fases 0–2 foram feitas de
forma sequencial (coupling alto); a paralelização brilha da **Fase 3 em diante**.

## Divisão de responsabilidades (fronteiras de arquivo)

| Agente | Arquivos | Com quem conversa (contratos) |
|---|---|---|
| `cad-user-infra` | docker-compose.yml (inclui réplicas `worker`/`worker-2`/`worker-3`), Dockerfile, Makefile (`logs`/`logs-once`/`lag`), .env*, healthchecks | Nenhum código Go; portas; nº de partições define máx. de workers |
| `cad-user-domain` | internal/domain/* (+ testes) | Contrato: structs + envelope (quem publica/consome usa) |
| `cad-user-kafka` | internal/kafka/*, cmd/generator | Consome `domain.Envelope`/`domain.User`; grupo com 3 membros; use `make lag` |
| `cad-user-mongo` | internal/mongodb/*, cmd/worker, scripts/mongo/* | Consome `domain.User`; índice único em `cpf`; Mongo single-node é o gargalo |
| `cad-user-reviewer` | leitura de qualquer arquivo | Não edita; devolve parecer |

## Sugestões de sprint paralelizáveis

1. **Sprint A (Fase 3)**:
   - `cad-user-mongo`: ampliar `scripts/mongo/consultas_estudo.js` (mais agregações,
     índices, `explain`) — arquivo próprio, zero conflito.
   - `cad-user-domain`: adicionar campos/dados sintéticos (ex.: `nacionalidade`,
     segundo endereço) + testes — **só se** o contrato for versionado (schema_version).
   - `cad-user-reviewer`: revisar o pipeline recém-construído em paralelo.
2. **Sprint B (Fase 4 — evolução)**:
   - `cad-user-kafka`: DLQ real (`cad-user.dlq` producer no worker) e observabilidade
     do consumer (contadores Prometheus).
   - `cad-user-mongo`: API REST de consulta (novo `cmd/api`) lendo o Mongo com
     paginação/índices.
   - `cad-user-infra`: Grafana/Prometheus e profiles no compose.

## Como disparar

1. No VS Code: abra o seletor de agentes (ícone na barra de chat do Copilot) e
   escolha o papel (ex.: `cad-user-mongo`).
2. Para o padrão orquestrador, crie um agente coordenador que chame os outros
   pela ferramenta `agent` (veja doc oficial de subagents do VS Code).
3. Cada agente roda `make check` / `go test ./...` antes de terminar.

## Definição de pronto (DoD) global

- [ ] `gofmt` limpo; `go build ./...` e `go vet ./...` sem erros.
- [ ] `go test ./...` verde (domínio ≥ ~80% cobertura).
- [ ] Fluxo validado ponta a ponta ao menos uma vez: `make load` pequeno
      (ex.: TOTAL=1000) e `db.users.countDocuments()` bate com o total.
- [ ] `make lag` com LAG=0 após a carga de teste (workers parados/drenados).
- [ ] Sem alteração fora da fronteira do papel (revise o diff).
