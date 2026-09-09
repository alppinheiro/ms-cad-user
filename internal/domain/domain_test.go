package domain

import (
	"testing"
)

func TestIsValidCPF(t *testing.T) {
	// CPFs conhecidos (válidos gerados por ferramentas públicas).
	valid := []string{
		"529.982.247-25",
		"111.444.777-35",
		"52998224725",
		"123.456.789-09",
		"987.654.321-00",
	}
	for _, cpf := range valid {
		if !IsValidCPF(cpf) {
			t.Errorf("esperava CPF válido: %s", cpf)
		}
	}

	invalid := []string{
		"111.111.111-11", // sequência repetida
		"000.000.000-00",
		"123.456.789-00", // dígito verificador errado
		"52998224726",    // dv errado
		"12345",          // tamanho inválido
		"abcdefghijk",
		"",
	}
	for _, cpf := range invalid {
		if IsValidCPF(cpf) {
			t.Errorf("esperava CPF inválido: %s", cpf)
		}
	}
}

func TestRandomCPFAlwaysValid(t *testing.T) {
	g := NewGenerator(42, "test")
	seen := make(map[string]struct{})
	for i := 0; i < 2000; i++ {
		cpf := g.uniqueCPF()
		if !IsValidCPF(cpf) {
			t.Fatalf("CPF gerado é inválido: %s", cpf)
		}
		if _, ok := seen[cpf]; ok {
			t.Fatalf("CPF duplicado: %s", cpf)
		}
		seen[cpf] = struct{}{}
	}
}

func TestGeneratorDeterministicAndValid(t *testing.T) {
	a := NewGenerator(7, "test")
	b := NewGenerator(7, "test")

	for i := 0; i < 500; i++ {
		ua := a.Next()
		ub := b.Next()
		if ua.UserID != ub.UserID || ua.CPF != ub.CPF || ua.Email != ub.Email {
			t.Fatalf("gerador não é determinístico na iteração %d", i)
		}
		if err := ua.Validate(); err != nil {
			t.Fatalf("usuário gerado é inválido: %v", err)
		}
	}
	if a.Count() != 500 {
		t.Fatalf("contador inesperado: %d", a.Count())
	}
}

func TestGeneratedEmailsUnique(t *testing.T) {
	g := NewGenerator(99, "test")
	emails := make(map[string]struct{})
	for i := 0; i < 500; i++ {
		emails[g.Next().Email] = struct{}{}
	}
	if len(emails) != 500 {
		t.Fatalf("esperava 500 emails únicos, obtive %d", len(emails))
	}
}
