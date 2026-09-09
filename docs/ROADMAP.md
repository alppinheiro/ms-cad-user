# Roadmap — ms-cad-user

Documento vivo de planejamento. Objetivo: estudar um pipeline assíncrono de
cadastro de usuários (Go + Kafka + MongoDB) com práticas de mercado e evoluir o
projeto em fases pequenas e verificáveis.

## ✅ Concluído (Fase 0–2 + base da Fase 3)

- [x] Infra docker-compose: MongoDB 7, Kafka 3.9 (KRaft, 6 partições), Mongo
      Express (`:8081`), Kafka UI (`:9085`), worker com restart automático.
- [x] `cmd/generator`: carga em massa (500k usuários @ ~2.000 msg/s, 0 erros),
      dados sintéticos pt-BR realistas e determinísticos por seed.
- [x] `cmd/worker`: consumer group → batch → Mongo `bulkWrite` upsert por CPF
      (idempotência; validado com 505k docs, lag 0, `mortos=0`).
- [x] **Escala horizontal**: tópico com 6 partições + 3 workers no mesmo group
      (`worker`, `worker-2`, `worker-3`) — rebalance 2/2/2 comprovado com carga
      real e consumo paralelo.
- [x] **Benchmark produtor × consumidor**: produtor confirmou ~6.000 msg/s sem
      erro; consumer sustentou ~5.6–6.3k msg/s; `make lag`/`make logs-once`
      criados para inspeção rápida.
- [x] **Volume de estudo**: base carregada com várias seeds (total no Mongo já
      em ~2M de documentos) para Fase 3.
- [x] Comentários didáticos em todo o código Go + `docs/ROADMAP.md` no repo.
- [x] Domínio Go com CPF válido (algoritmo oficial), envelope de evento
      versionado, testes com ~82% de cobertura no domínio.
- [x] Publicado no GitHub (`alppinheiro/ms-cad-user`) e ambiente local OK.
- [x] Caderno de consultas Mongo inicial (`scripts/mongo/consultas_estudo.js`).

## 🔜 Próximas fases (itens para não esquecer)

### Fase 3 — Estudo de consultas Mongo (em andamento)
- [ ] Rodar/bateria de consultas com **volume real (~2M docs já carregados)** e
      registrar tempos de resposta.
- [ ] Comparar consultas **com e sem índice** usando
      `.explain("executionStats")` e documentar no README/docs.
- [ ] Ampliar `scripts/mongo/consultas_estudo.js`: mais agregações, índices
      parciais/TTL, busca textual, `$lookup`, projeções e paginação.
- [ ] Criar `docs/MONGO_QUERIES.md` com os exemplos + explicação de cada plano.

### Fase 4 — Evolução do pipeline
- [ ] **API REST de cadastro/consulta** (novo `cmd/api`): `POST /v1/users`
      (publica no Kafka e responde 202) + `GET` com filtros/paginação lendo o
      Mongo — usar os índices já criados.
- [ ] **DLQ real**: publicar lotes que esgotaram retries no tópico
      `cad-user.dlq` (hoje apenas contabilizamos `mortos`).
- [ ] **Redis para dedupe/cache** (avaliado e deixado de fora por design —
      reavaliar quando houver taxa alta de reentrega ou necessidade de estudo).
- [ ] **Observabilidade**: métricas Prometheus (produtor/consumidor/Mongo),
      Grafana com dashboards, traces OpenTelemetry (Jaeger) por `event_id`.

### Fase 5 — Qualidade/engenharia
- [ ] Testes de integração com Testcontainers (Kafka + Mongo reais) e CI no
      GitHub Actions (check + integration + smoke).
- [ ] `.golangci.yml` e lint no CI.
- [ ] Multiagente: sprints paralelos usando `.github/agents/*` e `docs/AGENTES.md`
      (ex.: agentes `cad-user-mongo` e `cad-user-kafka` em frentes independentes).

## 📌 Ideias futuras / pendências menores
- Autoscaler por lag (tema estudado no workers-kafka; portar/adaptar aqui).
- Compressão/mensagens menores (medir ganho de throughput no produtor).
- Rastreabilidade ponta a ponta: header `event_id` + correlação em logs.
- Validar consistência: total de documentos vs CPFs distintos vs offsets do Kafka.
