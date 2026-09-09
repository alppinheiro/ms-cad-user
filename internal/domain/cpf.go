package domain

import (
	"math/rand"
	"strings"
)

// onlyDigits mantém apenas os caracteres numéricos da entrada.
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
// verificadores. Retorna false para sequências repetidas (ex.: 111.111.111-11).
func IsValidCPF(cpf string) bool {
	d := onlyDigits(cpf)
	if len(d) != 11 {
		return false
	}

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

	d1 := cpfCheckDigit(d[:9])
	if d1 != int(d[9]-'0') {
		return false
	}
	d2 := cpfCheckDigit(d[:10])
	return d2 == int(d[10]-'0')
}

// cpfCheckDigit calcula um dígito verificador do CPF para os primeiros n dígitos.
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
// diferente do segundo para evitar sequências repetidas.
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

func isRepeatedCPF(d string) bool {
	for i := 1; i < len(d); i++ {
		if d[i] != d[0] {
			return false
		}
	}
	return true
}
