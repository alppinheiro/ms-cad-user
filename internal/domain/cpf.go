package domain

import (
	"math/rand"
	"strings"
)

// Este arquivo implementa a regra oficial do CPF brasileiro:
//
//	CPF = 9 dígitos base + 2 dígitos verificadores (DV).
//	O DV é calculado com módulo 11 sobre a soma ponderada dos dígitos
//	(pesos 10..2 para o 1º DV e 11..2 para o 2º DV). Se o resto for
//	< 2 o DV é 0; senão DV = 11 - resto.
//
// Colocamos VALIDAÇÃO e GERAÇÃO juntas aqui porque são duas faces da mesma
// regra: o generator só produz CPFs que passam em IsValidCPF — isso garante
// que o dado que chega ao Mongo é coerente e que os estudos de query não são
// contaminados por "lixo sintático".

// onlyDigits mantém apenas os caracteres numéricos da entrada. Usamos
// strings.Builder porque é a forma eficiente de concatenar em loop em Go
// (evita criar uma string nova a cada caractere).
func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// IsValidCPF valida um CPF (com ou sem máscara) usando o algoritmo dos dígitos
// verificadores. Etapas:
//  1. normaliza para 11 dígitos (aceita "529.982.247-25" e "52998224725");
//  2. rejeita sequências repetidas (111.111.111-11 passaria no módulo 11,
//     então a Receita exige o descarte explícito);
//  3. calcula os dois DV e compara com os dígitos informados.
func IsValidCPF(cpf string) bool {
	d := onlyDigits(cpf)
	if len(d) != 11 {
		return false
	}

	// Sequências repetidas (ex.: 000.000.000-00) são inválidas por regra.
	allEqual := true
	for i := 1; i < len(d); i++ {
		if d[i] != d[0] {
			allEqual = false
			break
		}
	}
	if allEqual {
		return false
	}

	// 1º dígito verificador usa os 9 primeiros dígitos; o 2º usa os 10
	// primeiros (incluindo o 1º DV já validado). Compare com o CPF dado.
	d1 := cpfCheckDigit(d[:9])
	if d1 != int(d[9]-'0') {
		return false
	}
	d2 := cpfCheckDigit(d[:10])
	return d2 == int(d[10]-'0')
}

// cpfCheckDigit calcula um dígito verificador do CPF para os primeiros n
// dígitos. Regra: soma cada dígito multiplicado por um peso decrescente
// (len(part)+1 ... 2); resto = soma % 11; DV = 0 se resto < 2, senão 11-resto.
func cpfCheckDigit(part string) int {
	sum := 0
	weight := len(part) + 1
	for _, r := range part {
		sum += int(r-'0') * weight
		weight--
	}
	rest := sum % 11
	if rest < 2 {
		return 0
	}
	return 11 - rest
}

// randomCPF gera um CPF válido (somente dígitos), com o primeiro dígito sempre
// diferente do segundo para evitar sequências repetidas. Estratégia:
//  1. sorteia os 9 dígitos base com o RNG fornecido (determinístico por seed);
//  2. ajusta o 2º dígito se for igual ao 1º (evita 111.111.111-xx);
//  3. calcula os dois DV com a mesma função usada na validação.
func randomCPF(rng *rand.Rand) string {
	base := make([]byte, 9)
	for i := range base {
		base[i] = byte('0' + rng.Intn(10))
	}
	// Garante que o CPF não seja uma sequência repetida.
	if base[1] == base[0] {
		base[1] = byte('0' + (base[0]-'0'+1)%10)
	}

	d := make([]byte, 11)
	copy(d, base)
	d[9] = byte('0' + cpfCheckDigit(string(d[:9])))
	d[10] = byte('0' + cpfCheckDigit(string(d[:10])))

	// Se ainda assim o dígito verificador repetir a sequência inteira,
	// ajusta o terceiro dígito e recalcula (caso extremamente raro).
	if isRepeatedCPF(string(d)) {
		d[2] = byte('0' + (d[0]-'0'+2)%10)
		d[9] = byte('0' + cpfCheckDigit(string(d[:9])))
		d[10] = byte('0' + cpfCheckDigit(string(d[:10])))
	}
	return string(d)
}

// isRepeatedCPF retorna true se todos os 11 dígitos forem iguais.
func isRepeatedCPF(d string) bool {
	for i := 1; i < len(d); i++ {
		if d[i] != d[0] {
			return false
		}
	}
	return true
}
