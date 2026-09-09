// Package domain contém o CORAÇÃO do negócio, sem dependência de infraestrutura
// (nada de Kafka/Mongo aqui) — é o que permite testar a regra de negócio de
// forma rápida e isolada. Responsabilidades:
//
//   - Modelos: User, Telefone, Endereco (contrato do evento e do documento);
//   - Regras: validação de CPF (algoritmo oficial) e validação do cadastro;
//   - Contrato do Kafka: Envelope de evento versionado;
//   - Dados sintéticos pt-BR determinísticos (Generator) para a carga de estudo.
//
// As tags `json` definem o contrato da mensagem no Kafka; as tags `bson`
// definem a forma como o documento é gravado no MongoDB. Por isso as mesmas
// structs servem produtor, consumidor e persistência sem conversão manual.
package domain

import (
	"fmt"
	"time"
)

// Constantes de domínio. São valores "fechados" de negócio (enums em Go não
// existem nativamente), usados para evitar strings soltas ("magic values")
// espalhadas pelo código:
//   - StatusAtivo: status inicial de todo cadastro;
//   - SexoMasculino/SexoFeminino: domínio pequeno e estável, então string é ok
//     para o estudo (em produção com muitos valores usaríamos enum/tabela);
//   - CollectionUsers: nome padrão da coleção Mongo (sobrescrevível via env).
const (
	StatusAtivo     = "ATIVO"
	SexoMasculino   = "M"
	SexoFeminino    = "F"
	CollectionUsers = "users" // nome da coleção Mongo (padrão)
)

// User é o agregado principal: dados pessoais + contatos + endereço.
// Escolhas de modelagem e por quê:
//   - CPF/RG como string: CPF tem zeros à esquerda e máscara, então NUNCA usar
//     número inteiro (perderia o zero inicial); guardamos só dígitos para o
//     índice único e a validação ficarem simples.
//   - Sexo como string "M"/"F": domínio pequeno e estável (ver constantes).
//   - DataNascimento como time.Time (UTC): permite queries de faixa etária
//     reais no Mongo (ISODate) e comparações corretas entre fusos.
//   - CreatedAt/UpdatedAt: auditoria — "quando entrou no sistema" e "quando foi
//     reprocessado pela última vez" (importante para estudar reprocessamento).
//   - Telefones como array embutido: modelagem "documento" natural do Mongo
//     (evita join/relacionamento para um dado que só é lido junto do usuário).
type User struct {
	UserID         string     `json:"userId" bson:"userId"`
	Nome           string     `json:"nome" bson:"nome"`
	Sobrenome      string     `json:"sobrenome" bson:"sobrenome"`
	Email          string     `json:"email" bson:"email"`
	CPF            string     `json:"cpf" bson:"cpf"`
	RG             string     `json:"rg" bson:"rg"`
	Sexo           string     `json:"sexo" bson:"sexo"`
	DataNascimento time.Time  `json:"dataNascimento" bson:"dataNascimento"`
	Telefones      []Telefone `json:"telefones" bson:"telefones"`
	Endereco       Endereco   `json:"endereco" bson:"endereco"`
	Status         string     `json:"status" bson:"status"`
	Origem         string     `json:"origem" bson:"origem"`
	CreatedAt      time.Time  `json:"createdAt" bson:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt" bson:"updatedAt"`
}

// Telefone representa um contato telefônico do usuário.
// `Principal` indica o telefone preferencial (evita depender da posição no
// array) e `WhatsApp` habilita queries do tipo "quantos têm WhatsApp".
type Telefone struct {
	DDD       string `json:"ddd" bson:"ddd"`
	Numero    string `json:"numero" bson:"numero"`
	Tipo      string `json:"tipo" bson:"tipo"`
	WhatsApp  bool   `json:"whatsapp" bson:"whatsapp"`
	Principal bool   `json:"principal" bson:"principal"`
}

// Endereco representa o endereço residencial do usuário.
// Modelado como documento EMBUTIDO (não referência) porque acompanha o usuário
// e é consultado junto com ele; UF/Cidade separados em campos facilitam os
// índices compostos e agregações de estudo (ex.: group by UF).
type Endereco struct {
	Logradouro  string `json:"logradouro" bson:"logradouro"`
	Numero      string `json:"numero" bson:"numero"`
	Complemento string `json:"complemento" bson:"complemento"`
	Bairro      string `json:"bairro" bson:"bairro"`
	CEP         string `json:"cep" bson:"cep"`
	Cidade      string `json:"cidade" bson:"cidade"`
	UF          string `json:"uf" bson:"uf"`
}

// Validate aplica as regras básicas de negócio do cadastro.
// Usamos um switch com early-return porque: (1) a primeira falha já é
// informativa o bastante para o estudo; (2) evita acumular mensagens quando só
// queremos decidir "processa ou descarta". É chamada no WORKER (consumer) —
// mensagem com payload inválido não pode poluir o banco; quem produziu um dado
// inválido deverá ser tratado via DLQ (roadmap). Validação de CPF usa o
// algoritmo oficial em cpf.go.
func (u User) Validate() error {
	switch {
	case u.Nome == "":
		return fmt.Errorf("nome é obrigatório")
	case u.Sobrenome == "":
		return fmt.Errorf("sobrenome é obrigatório")
	case u.Email == "":
		return fmt.Errorf("email é obrigatório")
	case !IsValidCPF(u.CPF):
		return fmt.Errorf("cpf inválido: %s", u.CPF)
	case u.RG == "":
		return fmt.Errorf("rg é obrigatório")
	case u.Sexo != SexoMasculino && u.Sexo != SexoFeminino:
		return fmt.Errorf("sexo inválido: %s", u.Sexo)
	case u.DataNascimento.IsZero():
		return fmt.Errorf("data de nascimento é obrigatória")
	case len(u.Telefones) == 0:
		return fmt.Errorf("pelo menos um telefone é obrigatório")
	case u.Endereco.CEP == "":
		return fmt.Errorf("cep é obrigatório")
	case u.Endereco.Cidade == "":
		return fmt.Errorf("cidade é obrigatória")
	case u.Endereco.UF == "":
		return fmt.Errorf("uf é obrigatória")
	}
	return nil
}
