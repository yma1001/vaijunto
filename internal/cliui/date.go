package cliui

import (
	"fmt"
	"strings"
	"time"
)

// Layouts de data. O fio do protocolo e a persistência continuam em ISO
// (YYYY-MM-DD). Só a CLI humana lê e mostra DD/MM/AAAA.
const (
	ISODateLayout = "2006-01-02"
	BRDateLayout  = "02/01/2006"
)

// ParseBRDate converte entrada DD/MM/AAAA em YYYY-MM-DD.
// Usa time.Parse e rejeita calendário inválido (ex.: 31/02/2026) com
// round-trip do valor formatado — o parser do Go aceitaria overflow.
func ParseBRDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("data vazia: use DD/MM/AAAA (exemplo: 16/09/2026)")
	}
	t, err := time.Parse(BRDateLayout, s)
	if err != nil {
		return "", fmt.Errorf("data inválida %q: use DD/MM/AAAA (exemplo: 16/09/2026)", s)
	}
	if t.Format(BRDateLayout) != s {
		return "", fmt.Errorf("data inválida %q: esse dia não existe no calendário", s)
	}
	return t.Format(ISODateLayout), nil
}

// FormatBRDate converte YYYY-MM-DD (protocolo) em DD/MM/AAAA para exibição.
func FormatBRDate(iso string) string {
	iso = strings.TrimSpace(iso)
	t, err := time.Parse(ISODateLayout, iso)
	if err != nil {
		return iso
	}
	return t.Format(BRDateLayout)
}

// FormatCreatedAt mostra o createdAt do protocolo (UTC RFC3339 simplificado)
// como data/hora brasileira para o usuário.
func FormatCreatedAt(rfc3339 string) string {
	s := strings.TrimSpace(rfc3339)
	if s == "" {
		return ""
	}
	layouts := []string{
		"2006-01-02T15:04:05Z",
		time.RFC3339,
		time.RFC3339Nano,
	}
	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t.UTC().Format("02/01/2006 15:04")
		}
	}
	return s
}
