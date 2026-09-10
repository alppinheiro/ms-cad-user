# Consultas no Mongo Express — sintaxe e limitações

Guia curto para usar a **caixa de Query** do Mongo Express (`http://localhost:8081`)
na coleção `cad_user.users`.

## ⚠️ Regra de ouro: Mongo Express NÃO entende Extended JSON (`$date`)

O Mongo Express usa o parser `mongodb-query-parser`, que **não suporta EJSON**.
Se você digitar:

```json
{ "dataNascimento": { "$gte": { "$date": "1980-01-01T00:00:00Z" } } }
```

o `$date` é interpretado como um **objeto literal** → **"No documents found."**
(teste realizado: retornou 0 resultados).

## ✅ Sintaxe que FUNCIONA: `ISODate("...")`

```json
{
  "dataNascimento": {
    "$gte": ISODate("1980-01-01T00:00:00Z"),
    "$lte": ISODate("1990-12-31T23:59:59Z")
  }
}
```

Resultado esperado com a base atual: **355.502 documentos** (validado via mongosh).

## Tabela de equivalência entre ferramentas

| Ferramenta | Formato aceito |
|---|---|
| **Mongo Express** | `ISODate("1980-01-01T00:00:00Z")` (função JS) — **NÃO** aceita `{"$date": ...}` |
| **mongosh** | `ISODate("...")` **e** Extended JSON `{"$date": "..."}` |
| **driver Go / API** | `time.Time` (o driver converte para BSON Date) |

> Curiosidade: no Mongo Express a data é **exibida** como
> `Wed May 06 1987 00:00:00 GMT+0000 (Coordinated Universal Time)` porque a UI
> converte o BSON Date para JavaScript Date e chama `.toString()`. Isso é apenas
> formatação — o valor armazenado é `Date` (ISODate), confirmado por
> `$type: "date"` em 100% dos documentos.

## Exemplos prontos para colar na Query do Mongo Express

```javascript
// Faixa de data de nascimento (1980 a 1990)
{ "dataNascimento": { "$gte": ISODate("1980-01-01T00:00:00Z"), "$lte": ISODate("1990-12-31T23:59:59Z") } }

// Por UF + cidade
{ "endereco.uf": "SP", "endereco.cidade": "São Paulo" }

// Sexo feminino com WhatsApp no telefone principal
{ "sexo": "F", "telefones": { "$elemMatch": { "tipo": "CELULAR", "whatsapp": true } } }

// Nome começando com "Maria" (regex)
{ "nome": { "$regex": "^Maria", "$options": "i" } }

// CPF específico (usa índice único)
{ "cpf": "52998224725" }
```

## Limitações a lembrar

- **Não há UI de aggregation** (pipeline) no Mongo Express — para `$group`,
  `$unwind`, `$lookup` use `mongosh`/scripts (`scripts/mongo/consultas_estudo.js`).
- A lista da coleção **não mostra o total filtrado** de forma destacada; para
  contagem exata use `db.users.countDocuments({...})` no mongosh.
- Consultas muito pesadas sem índice podem estourar o timeout da UI; prefira
  filtros que usem os índices criados (`uniq_cpf`, `idx_uf_cidade`,
  `idx_sexo_nascimento`, `idx_created_at`, `idx_email`).

## Alternativas recomendadas

1. **Mongo Express**: consultas simples com `ISODate(...)` (como acima);
2. **mongosh** (`make mongo-shell`): consultas completas + agregações;
3. **Scripts versionados** em `scripts/mongo/*.js` executados por
   `docker exec -i mscaduser-mongodb mongosh --quiet cad_user < scripts/mongo/arquivo.js`.
