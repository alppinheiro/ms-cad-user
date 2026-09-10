# PoC — Miro MCP (board + atividades + acompanhamento)

Objetivo: usar o **MCP oficial do Miro** no VS Code para criar um board Kanban
com as atividades do projeto e acompanhar o andamento (Backlog → Doing → Review
→ Done), permitindo que agentes trabalhem em paralelo.

## 1) O que já está pronto neste repositório

- **`.vscode/mcp.json`** com o servidor remoto oficial:
  ```json
  { "servers": { "miro": { "type": "http", "url": "https://mcp.miro.com/" } } }
  ```
  (endpoint obtido da documentação oficial do Miro; autenticação é OAuth 2.1
  gerenciada pelo próprio VS Code.)

## 2) Como conectar (passo a passo)

1. Abra o projeto no VS Code e localize o arquivo `.vscode/mcp.json`
   (ou rode `MCP: Open Workspace Folder Configuration` no Command Palette);
2. Clique em **Start** (code lens em cima do servidor) e confirme o **Trust**;
3. O VS Code abre o **OAuth no navegador** → faça login no Miro e escolha o
   **time** onde o board ficará. ⚠️ A conexão é **1 time por vez**: se autorizar
   outro time depois, a conexão anterior é substituída;
4. Confirme com `MCP: List Servers` → `miro` → **Show Output** (se houver erro,
   veja `docs` de troubleshooting do Miro);
5. No **Chat (modo Agent)**, as tools do Miro ficam disponíveis (ex.:
   `board_create`, `canvas_*`, `comment_*`).

> Alternativa sem JSON: instalar pelo **MCP registry** do VS Code
> (Extensions → seção **MCP SERVERS** → buscar "Miro" → Install). O resultado é
> o mesmo.

## 3) Limites e cuidados (importante!)

| Item | Valor/observação |
|---|---|
| Limite diário de tool calls | Free **100**, Starter **500**, Business **2.000**, Enterprise **10.000** (reset 00:00 UTC) |
| Webhooks/eventos | ❌ o MCP **não** tem; só a **REST API** do Miro tem |
| Board por interação | Sempre **cole a URL do board** no prompt ("prompt hygiene") |
| Time | Só 1 time autorizado por vez |
| Tools em transição | As novas **Canvas tools** (`canvas_*`, SVG) substituem `layout_*`, `diagram_*`, `context_*`, `doc_*` (deprecação em **14/09/2026**) |

Tools úteis hoje: `board_create`, `board_search_boards`,
`canvas_get_canvas_composer_skill` (chamar 1x antes de escrever),
`canvas_create_from_svg`, `canvas_update_from_svg`, `canvas_read_as_svg`,
`comment_list_comments`, `comment_reply`, `comment_resolve` (e tabelas com
visualização Kanban via canvas).

## 4) Prompt pronto (copie e cole no Chat em modo Agent)

```text
Crie um novo board no Miro chamado "ms-cad-user — Roadmap de Estudos".

Monte um Kanban com 4 frames (colunas), da esquerda para a direita:
1) Backlog  2) Doing  3) Review  4) Done

No frame Backlog, crie 3 cards com título + descrição contendo o critério de pronto:

A) API REST de consulta (cmd/api)
   - GET /v1/users com filtros (uf, cidade, sexo, faixa de dataNascimento) e paginação;
   - POST /v1/users publica user.created no Kafka e responde 202 Accepted.

B) DLQ real no worker (cad-user.dlq)
   - Lote que esgota WORKER_RETRY_MAX deve ser publicado em cad-user.dlq com payload + motivo;
   - métrica "mortos" deve zerar após a mudança.

C) Bateria de consultas Mongo com/sem índice (.explain)
   - Ampliar scripts/mongo/consultas_estudo.js com consultas + explain("executionStats");
   - documentar em docs/MONGO_QUERIES.md.

Ao final, retorne o link do board e a lista dos itens criados (com IDs).
```

### Para acompanhar o andamento (prompts seguintes)

```text
Board: <COLE A URL DO BOARD AQUI>

Mova o card "API REST de consulta (cmd/api)" do frame Doing para Review e
adicione um comentário com o resumo do que foi feito e o link do PR/commit.
```

```text
Board: <COLE A URL DO BOARD AQUI>

Liste os cards de cada frame e me diga quantos estão em Backlog, Doing, Review e Done.
```

## 5) Por que isso ajuda no trabalho paralelo dos agentes

- **1 card = 1 unidade de trabalho** → cada agente "pega" um card diferente;
- O agente marca `Doing` ao começar, comenta o progresso e move para `Review`;
- Um agente revisor move para `Done`;
- Para evitar conflito de arquivos no repositório, combine com
  **git worktrees** por card (ver `docs/AGENTES.md`).

## 6) Limitações da PoC e próximo passo (Fase 2)

O MCP oficial é ótimo para **criar/ler/comentar** e para a experiência visual,
mas é **mediado por IA**, tem **limite diário** e **não tem webhooks**. Se
quisermos automação determinística (ex.: mover card dispara agente), o caminho é
o **nosso MCP server em Go sobre a REST API v2 do Miro** (planejado no roadmap):
`POST /v2/boards`, `POST /v2/boards/{id}/frames`, `POST /v2/boards/{id}/cards`,
`PATCH /v2/boards/{id}/items/{id}` (mover entre frames) e **webhooks**.
