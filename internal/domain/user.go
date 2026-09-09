package domain

import (
	"fmt"
	"time"
)

// Constantes de domínio.
const (
	StatusAtivo     = "ATIVO"
	SexoMasculino   = "M"
	SexoFeminino    = "F"
	CollectionUsers = "users" // nome da coleção Mongo (padrão)
)

// User é o agregado principal: dados pessoais + contatos + endereço.
// As tags `json` definem o contrato do evento no Kafka; as tags `bson`
// definem a forma do documento no MongoDB.
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
type Telefone struct {
	DDD       string `json:"ddd" bson:"ddd"`
	Numero    string `json:"numero" bson:"numero"`
	Tipo      string `json:"tipo" bson:"tipo"`
	WhatsApp  bool   `json:"whatsapp" bson:"whatsapp"`
	Principal bool   `json:"principal" bson:"principal"`
}

// Endereco representa o endereço residencial do usuário.
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
