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

// EnsureIndexes cria os índices da coleção users (idempotente). O índice
// único em cpf é a âncora da idempotência do fluxo Kafka -> Mongo.
func EnsureIndexes(ctx context.Context, client *mongo.Client, dbName string) error {
	coll := client.Database(dbName).Collection(domain.CollectionUsers)
	models := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "cpf", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_cpf"),
		},
		{
			Keys:    bson.D{{Key: "userId", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_userid"),
		},
		{
			Keys:    bson.D{{Key: "endereco.uf", Value: 1}, {Key: "endereco.cidade", Value: 1}},
			Options: options.Index().SetName("idx_uf_cidade"),
		},
		{
			Keys:    bson.D{{Key: "sexo", Value: 1}, {Key: "dataNascimento", Value: 1}},
			Options: options.Index().SetName("idx_sexo_nascimento"),
		},
		{
			Keys:    bson.D{{Key: "createdAt", Value: 1}},
			Options: options.Index().SetName("idx_created_at"),
		},
		{
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

// UpsertUsers grava um lote de usuários com upsert pela chave natural `cpf`
// (ordered:false para máxima vazão). Reentregas viram updates sem duplicação.
func UpsertUsers(ctx context.Context, client *mongo.Client, dbName string, users []domain.User) (BulkStats, error) {
	if len(users) == 0 {
		return BulkStats{}, nil
	}

	coll := client.Database(dbName).Collection(domain.CollectionUsers)
	now := time.Now().UTC()
	models := make([]mongo.WriteModel, 0, len(users))
	for _, u := range users {
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
			"_id":       u.UserID,
			"origem":    u.Origem,
			"createdAt": u.CreatedAt,
		}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"cpf": u.CPF}).
			SetUpdate(bson.M{"$set": set, "$setOnInsert": setOnInsert}).
			SetUpsert(true))
	}

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
