package cliui

import (
	"fmt"
	"strings"
	"unicode"
)

// ParseBRLToCents converte texto de preço humano em centavos (int64).
// Aceita 15, 15,50 e 15.50. No máximo duas casas. Sem float64.
// Um trecho = um valor: não interpreta lista separada por vírgula.
func ParseBRLToCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("preço vazio")
	}
	for _, r := range s {
		if r == '-' {
			return 0, fmt.Errorf("preço não pode ser negativo")
		}
		if r == '+' || unicode.IsSpace(r) {
			return 0, fmt.Errorf("preço inválido %q", s)
		}
	}

	commaCount := strings.Count(s, ",")
	dotCount := strings.Count(s, ".")
	if commaCount+dotCount > 1 {
		return 0, fmt.Errorf("informe um único preço por trecho (use 15, 15,50 ou 15.50)")
	}

	sep := ""
	switch {
	case commaCount == 1:
		sep = ","
	case dotCount == 1:
		sep = "."
	}

	whole, frac := s, ""
	if sep != "" {
		parts := strings.SplitN(s, sep, 2)
		whole, frac = parts[0], parts[1]
		if frac == "" {
			return 0, fmt.Errorf("preço inválido %q", s)
		}
	}
	if whole == "" || !allDigits(whole) {
		return 0, fmt.Errorf("preço inválido %q", s)
	}
	if frac != "" && (!allDigits(frac) || len(frac) > 2) {
		return 0, fmt.Errorf("preço inválido %q: no máximo duas casas decimais", s)
	}

	reais, err := parseUint64Text(whole)
	if err != nil {
		return 0, fmt.Errorf("preço inválido %q", s)
	}
	if reais > uint64((1<<63-1)/100) {
		return 0, fmt.Errorf("preço inválido %q: valor grande demais", s)
	}
	cents := int64(reais) * 100
	if frac != "" {
		switch len(frac) {
		case 1:
			cents += int64(frac[0]-'0') * 10
		case 2:
			cents += int64(frac[0]-'0')*10 + int64(frac[1]-'0')
		}
	}
	return cents, nil
}

// FormatBRL formata centavos como "R$ 15,00". Nunca usa float64.
func FormatBRL(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%sR$ %d,%02d", sign, cents/100, cents%100)
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseUint64Text(s string) (uint64, error) {
	var n uint64
	for _, r := range s {
		d := uint64(r - '0')
		if n > (^uint64(0)-d)/10 {
			return 0, fmt.Errorf("overflow")
		}
		n = n*10 + d
	}
	return n, nil
}
