// Package mongodb é a camada de persistência do worker. Os usuários são
// gravados via bulk upsert por CPF (índice único) — é isso que torna o fluxo
// idempotente: reentregas do Kafka ou reexecução do generator apenas atualizam
// o documento existente, sem duplicar registros.
package mongodb

import (
	"context"
	"fmt"
	"time"

	"ms-cad-user/internal/domain"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Connect abre o cliente e valida conectividade com um ping.
// O driver do Mongo conecta de forma "lazy" (a primeira operação é que
// realmente conecta), então o PING aqui é o fail-fast: se o banco estiver
// fora/URI errada, o worker morre no boot com mensagem clara — em vez de
// subir "vivo" e só descobrir o problema quando chegar a primeira mensagem.
func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("mongo ping: %w", err)
	}
	return client, nil
}

// EnsureIndexes cria os índices da coleção users (idempotente — rodar várias
// vezes é seguro). A lista reflete os PADRÕES DE CONSULTA que queremos estudar:
//
//   - uniq_cpf (ÚNICO): é a ÂNCORA DA IDEMPOTÊNCIA — o upsert filtra por cpf e
//     o Mongo garante por construção que não existem 2 docs com o mesmo CPF;
//   - uniq_userid (ÚNICO): integridade do identificador lógico do usuário;
//   - idx_uf_cidade (composto): acelera "usuários de SP/São Paulo" e o
//     $group por UF (filtros + agregação juntos usam um só índice);
//   - idx_sexo_nascimento (composto): consultas "sexo + faixa etária";
//   - idx_created_at: ordenação/filtro por data de criação (auditoria);
//   - idx_email: busca por email (não-único de propósito — email não é chave
//     de negócio aqui e o generator garante unicidade na prática).
func EnsureIndexes(ctx context.Context, client *mongo.Client, dbName string) error {
	coll := client.Database(dbName).Collection(domain.CollectionUsers)
	models := []mongo.IndexModel{
		{ // CPF é a chave natural do cadastro → único.
			Keys:    bson.D{{Key: "cpf", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_cpf"),
		},
		{ // userId (UUID) também é único.
			Keys:    bson.D{{Key: "userId", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_userid"),
		},
		{ // Composto UF → cidade (prefixo de index cobre filtros só por UF).
			Keys:    bson.D{{Key: "endereco.uf", Value: 1}, {Key: "endereco.cidade", Value: 1}},
			Options: options.Index().SetName("idx_uf_cidade"),
		},
		{ // Composto sexo + data de nascimento (consultas demográficas).
			Keys:    bson.D{{Key: "sexo", Value: 1}, {Key: "dataNascimento", Value: 1}},
			Options: options.Index().SetName("idx_sexo_nascimento"),
		},
		{ // Filtros/ordenações por data de inclusão.
			Keys:    bson.D{{Key: "createdAt", Value: 1}},
			Options: options.Index().SetName("idx_created_at"),
		},
		{ // Busca por email.
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetName("idx_email"),
		},
	}
	if _, err := coll.Indexes().CreateMany(ctx, models); err != nil {
		return fmt.Errorf("criar índices: %w", err)
	}
	return nil
}

// BulkStats resume o resultado de um bulk write.
type BulkStats struct {
	Attempted int64
	Inserted  int64 // UpsertedCount (novos documentos)
	Matched   int64 // documentos já existentes
	Modified  int64 // documentos de fato alterados
}

// UpsertUsers grava um lote de usuários com upsert pela chave natural `cpf`.
//
// Por que UPSERT por CPF (e não insert simples)?
//   - Reentregas do Kafka (at-least-once) e reexecuções do generator chegam
//     como "a mesma pessoa de novo": se fizéssemos insert, criaríamos
//     DUPLICATAS; com upsert, a segunda entrega vira um UPDATE no documento
//     existente → resultado sempre consistente e idempotente.
//
// ordered:false: mesmo que um documento do lote falhe, o Mongo continua
// processando os demais (máxima vazão — importante em lotes de 1.000). Os
// contadores do resultado (BulkStats) permitem medir insert vs update.
func UpsertUsers(ctx context.Context, client *mongo.Client, dbName string, users []domain.User) (BulkStats, error) {
	if len(users) == 0 {
		return BulkStats{}, nil
	}

	coll := client.Database(dbName).Collection(domain.CollectionUsers)

	// `now` é único por lote: garante que todos os documentos de um mesmo batch
	// recebam o mesmo instante de atualização (coerência para queries de tempo).
	now := time.Now().UTC()
	models := make([]mongo.WriteModel, 0, len(users))
	for _, u := range users {
		// Converte o usuário em OPERAÇÕES Mongo (uma por usuário):
		//   $set          → campos que SEMPRE são gravados (dados atuais);
		//   $setOnInsert  → campos gravados SOMENTE na criação do documento.
		// Por que separar? Porque createdAt/origem são HISTÓRICO: se uma
		// reentrega chega depois, não queremos "apagar" quando o registro
		// nasceu originalmente — apenas atualizar os dados e o updatedAt.
		u.UpdatedAt = now // cada processamento renova o carimbo de atualização
		set := bson.M{
			"userId":         u.UserID,
			"nome":           u.Nome,
			"sobrenome":      u.Sobrenome,
			"email":          u.Email,
			"cpf":            u.CPF,
			"rg":             u.RG,
			"sexo":           u.Sexo,
			"dataNascimento": u.DataNascimento,
			"telefones":      u.Telefones,
			"endereco":       u.Endereco,
			"status":         u.Status,
			"updatedAt":      u.UpdatedAt,
		}
		setOnInsert := bson.M{
			"_id":       u.UserID, // _id é definido UMA vez, no insert
			"origem":    u.Origem,
			"createdAt": u.CreatedAt,
		}

		// Filtro pelo cpf (chave natural + índice único). SetUpsert(true):
		// se não existir → insert; se existir → update. Nunca duplicata.
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"cpf": u.CPF}).
			SetUpdate(bson.M{"$set": set, "$setOnInsert": setOnInsert}).
			SetUpsert(true))
	}

	// ordered:false = processa o lote inteiro mesmo com falhas pontuais
	// (máxima vazão). Em caso de erro, devolvemos o erro para o retry do
	// handler; como o upsert é idempotente, o retry é seguro.
	res, err := coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return BulkStats{Attempted: int64(len(models))}, fmt.Errorf("bulk upsert users: %w", err)
	}
	return BulkStats{
		Attempted: int64(len(models)),
		Inserted:  int64(res.UpsertedCount),
		Matched:   int64(res.MatchedCount),
		Modified:  int64(res.ModifiedCount),
	}, nil
}
