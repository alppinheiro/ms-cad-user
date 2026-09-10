---
mode: agent
description: Cria e atualiza o board Kanban do ms-cad-user no Miro (via MCP oficial)
---
Crie um novo board no Miro chamado "ms-cad-user — Roadmap de Estudos".

Monte um Kanban com 4 frames (colunas), da esquerda para a direita:
1) Backlog  2) Doing  3) Review  4) Done

No frame Backlog, crie 3 cards com título + descrição contendo o critério de pronto:

A) API REST de consulta (cmd/api)
   - GET /v1/users com filtros (uf, cidade, sexo, faixa de dataNascimento) e paginação;
   - POST /v1/users publica user.created no Kafka e responde 202 Accepted.

B) DLQ real no worker (cad-user.dlq)
   - Lote que esgota WORKER_RETRY_MAX deve ser publicado em cad-user.dlq com payload + motivo;
   - a métrica "mortos" deve zerar após a mudança.

C) Bateria de consultas Mongo com/sem índice (.explain)
   - Ampliar scripts/mongo/consultas_estudo.js com consultas + explain("executionStats");
   - documentar em docs/MONGO_QUERIES.md.

Ao final, retorne o link do board e a lista dos itens criados (com IDs).
Nas próximas interações, exija que a URL do board seja informada no prompt.
