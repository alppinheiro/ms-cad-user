package domain

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// Generator produz usuários sintéticos realistas (pt-BR) de forma
// determinística por seed — reexecutar com a mesma seed gera os MESMOS dados,
// o que torna a carga reproduzível e idempotente por CPF/email.
type Generator struct {
	rng       *rand.Rand
	usedCPF   map[string]struct{}
	usedEmail map[string]struct{}
	origin    string
	n         int64
}

// NewGenerator cria um gerador com a seed e a origem informada.
func NewGenerator(seed int64, origin string) *Generator {
	return &Generator{
		rng:       rand.New(rand.NewSource(seed)),
		usedCPF:   make(map[string]struct{}),
		usedEmail: make(map[string]struct{}),
		origin:    origin,
	}
}

// Count retorna quantos usuários já foram gerados.
func (g *Generator) Count() int64 { return g.n }

// Next gera o próximo usuário, garantindo CPF e email únicos na instância.
func (g *Generator) Next() User {
	uf := ufs[g.rng.Intn(len(ufs))]
	cs := citiesByUF[uf]
	c := cs[g.rng.Intn(len(cs))]

	sexo := SexoMasculino
	if g.rng.Intn(2) == 1 {
		sexo = SexoFeminino
	}
	firstNames := firstNamesM
	if sexo == SexoFeminino {
		firstNames = firstNamesF
	}
	nome := firstNames[g.rng.Intn(len(firstNames))]
	sobrenome := lastNames[g.rng.Intn(len(lastNames))]

	created := time.Now().UTC()
	u := User{
		UserID:         randomUUID(g.rng),
		Nome:           nome,
		Sobrenome:      sobrenome,
		Email:          g.uniqueEmail(nome, sobrenome),
		CPF:            g.uniqueCPF(),
		RG:             randomRG(g.rng),
		Sexo:           sexo,
		DataNascimento: randomBirthDate(g.rng),
		Telefones:      randomTelefones(g.rng, c.ddd),
		Endereco:       randomEndereco(g.rng, uf, c),
		Status:         StatusAtivo,
		Origem:         g.origin,
		CreatedAt:      created,
		UpdatedAt:      created,
	}
	g.n++
	return u
}

// uniqueCPF gera um CPF válido e ainda não utilizado nesta execução.
func (g *Generator) uniqueCPF() string {
	for {
		cpf := randomCPF(g.rng)
		if _, ok := g.usedCPF[cpf]; ok {
			continue
		}
		g.usedCPF[cpf] = struct{}{}
		return cpf
	}
}

// uniqueEmail monta nome.sobrenome+suporte numérico em domínios comuns.
func (g *Generator) uniqueEmail(nome, sobrenome string) string {
	base := slug(nome) + "." + slug(sobrenome)
	for attempts := 0; attempts < 100; attempts++ {
		suffix := fmt.Sprintf("%04d", g.rng.Intn(10000))
		email := fmt.Sprintf("%s%s@%s", base, suffix, emailDomains[g.rng.Intn(len(emailDomains))])
		if _, ok := g.usedEmail[email]; ok {
			continue
		}
		g.usedEmail[email] = struct{}{}
		return email
	}
	// Extremamente improvável: esgota com um marcador único no fim.
	return fmt.Sprintf("%s.%d@%s", base, g.n, emailDomains[0])
}

func randomRG(rng *rand.Rand) string {
	return fmt.Sprintf("%02d.%03d.%03d-%d",
		rng.Intn(90), rng.Intn(1000), rng.Intn(1000), rng.Intn(10))
}

// randomBirthDate sorteia uma data de nascimento para idades entre 18 e 80 anos.
func randomBirthDate(rng *rand.Rand) time.Time {
	now := time.Now().UTC()
	age := 18 + rng.Intn(63)
	year := now.Year() - age
	month := 1 + rng.Intn(12)
	day := 1 + rng.Intn(28)
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func randomTelefones(rng *rand.Rand, ddd int) []Telefone {
	dddStr := fmt.Sprintf("%02d", ddd)
	phones := []Telefone{{
		DDD:       dddStr,
		Numero:    fmt.Sprintf("9%04d-%04d", rng.Intn(10000), rng.Intn(10000)),
		Tipo:      "CELULAR",
		WhatsApp:  rng.Intn(10) < 9,
		Principal: true,
	}}
	if rng.Intn(100) < 60 { // 60% têm segundo telefone
		phones = append(phones, Telefone{
			DDD:      dddStr,
			Numero:   fmt.Sprintf("%04d-%04d", 3+rng.Intn(3), rng.Intn(10000)),
			Tipo:     "RESIDENCIAL",
			WhatsApp: false,
		})
	}
	return phones
}

func randomEndereco(rng *rand.Rand, uf string, c cidade) Endereco {
	complemento := ""
	if rng.Intn(100) < 25 {
		complemento = fmt.Sprintf("Apto %d", 100+rng.Intn(900))
	}
	prefix := c.cepMin + rng.Intn(c.cepMax-c.cepMin+1)
	cep := fmt.Sprintf("%05d-%03d", prefix, rng.Intn(1000))
	return Endereco{
		Logradouro:  fmt.Sprintf("%s %s", streetKinds[rng.Intn(len(streetKinds))], streets[rng.Intn(len(streets))]),
		Numero:      fmt.Sprintf("%d", 1+rng.Intn(9000)),
		Complemento: complemento,
		Bairro:      bairros[rng.Intn(len(bairros))],
		CEP:         cep,
		Cidade:      c.nome,
		UF:          uf,
	}
}

// randomUUID gera um UUID v4 determinístico a partir do RNG do gerador, para
// que a mesma seed produza exatamente o mesmo dataset (idempotente por CPF).
func randomUUID(rng *rand.Rand) string {
	b := make([]byte, 16)
	if _, err := rng.Read(b); err != nil {
		panic(fmt.Sprintf("rng.Read falhou: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // versão 4
	b[8] = (b[8] & 0x3f) | 0x80 // variante RFC 4122
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// slug normaliza nome para composição de email (sem acentos/espaços).
func slug(s string) string {
	replacer := strings.NewReplacer(
		"á", "a", "à", "a", "ã", "a", "â", "a", "ä", "a",
		"é", "e", "è", "e", "ê", "e", "ë", "e",
		"í", "i", "ì", "i", "î", "i", "ï", "i",
		"ó", "o", "ò", "o", "õ", "o", "ô", "o", "ö", "o",
		"ú", "u", "ù", "u", "û", "u", "ü", "u",
		"ç", "c",
	)
	s = replacer.Replace(s)
	s = strings.ToLower(s)
	return strings.ReplaceAll(s, " ", "")
}
