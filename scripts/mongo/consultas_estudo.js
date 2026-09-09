// ============================================================================
//  Caderno de consultas — MongoDB (Fase 3 de estudo)
//  Execute dentro do container após popular a base (make load):
//      make mongo-shell   # mongosh cad_user
//      load("consultas_estudo.js")
//  Ou rode direto:  docker-compose exec mongodb mongosh cad_user \
//      --file /scripts/mongo/consultas_estudo.js  (após montar o volume)
// ============================================================================

db = db.getSiblingDB("cad_user");

print("\n=== 1. TOTAL DE DOCUMENTOS ===");
db.users.countDocuments({});

print("\n=== 2. BUSCA POR CPF (usa índice uniq_cpf) ===");
db.users.find({ cpf: "52998224725" }).pretty();

print("\n=== 3. USUÁRIOS POR UF (índice idx_uf_cidade) ===");
db.users.find({ "endereco.uf": "SP" }).limit(5).pretty();

print("\n=== 4. AGRUPAMENTO POR UF (top 10 estados) ===");
db.users.aggregate([
  { $group: { _id: "$endereco.uf", total: { $sum: 1 } } },
  { $sort: { total: -1 } },
  { $limit: 10 },
]);

print("\n=== 5. DISTRIBUIÇÃO POR SEXO E UF ===");
db.users.aggregate([
  { $group: { _id: { uf: "$endereco.uf", sexo: "$sexo" }, total: { $sum: 1 } } },
  { $sort: { "_id.uf": 1 } },
]);

print("\n=== 6. FAIXA ETÁRIA (idades de 18 a 80) ===");
db.users.aggregate([
  {
    $project: {
      faixa: {
        $switch: {
          branches: [
            { case: { $lte: ["$dataNascimento", ISODate("1980-01-01T00:00:00Z")] }, then: "45+" },
            { case: { $lte: ["$dataNascimento", ISODate("1995-01-01T00:00:00Z")] }, then: "30-44" },
            { case: { $lte: ["$dataNascimento", ISODate("2005-01-01T00:00:00Z")] }, then: "18-29" },
          ],
          default: "outros",
        },
      },
    },
  },
  { $group: { _id: "$faixa", total: { $sum: 1 } } },
  { $sort: { total: -1 } },
]);

print("\n=== 7. USUÁRIOS COM WHATSAPP POR CIDADE ===");
db.users.aggregate([
  { $match: { "telefones.whatsapp": true } },
  { $group: { _id: { cidade: "$endereco.cidade", uf: "$endereco.uf" }, total: { $sum: 1 } } },
  { $sort: { total: -1 } },
  { $limit: 10 },
]);

print("\n=== 8. TELEFONES PRINCIPAIS (desnormalização — array) ===");
db.users.aggregate([
  { $unwind: "$telefones" },
  { $match: { "telefones.principal": true } },
  { $project: { _id: 0, nome: 1, cpf: 1, ddd: "$telefones.ddd", numero: "$telefones.numero" } },
  { $limit: 5 },
]);

print("\n=== 9. BUSCA TEXTUAL POR NOME/CIDADE (cria índice texto) ===");
db.users.createIndex({ nome: "text", sobrenome: "text", "endereco.cidade": "text" });
db.users.find({ $text: { $search: "Maria Santos" } }).limit(5).pretty();

print("\n=== 10. QUAIS ÍNDICES EXISTEM ===");
db.users.getIndexes().forEach((i) => printjson(i));

print("\n=== 11. EXPLICAR UM PLANO (usa idx_uf_cidade) ===");
db.users.find({ "endereco.uf": "SP", "endereco.cidade": "São Paulo" })
  .explain("executionStats").executionStats.totalDocsExamined;

print("\n=== 12. CONTAGEM POR ORIGEM (auditoria) ===");
db.users.aggregate([{ $group: { _id: "$origem", total: { $sum: 1 } } }]);

print("\n=== 13. ESTATÍSTICA DE COLEÇÃO ===");
db.users.stats();
