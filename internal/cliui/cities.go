package cliui

import (
	"fmt"
	"strings"
)

const CitiesPrompt = "Cidades da rota, separadas por vírgula (mínimo 2)\nexemplo: Salvador, Feira de Santana, Jequié, Vitória da Conquista\n> "

// ParseCities interpreta a lista digitada pelo motorista.
// Não há limite de 3 cidades: só o mínimo de 2. Itens vazios são rejeitados
// (não são ignorados), depois de TrimSpace.
func ParseCities(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("informe no mínimo 2 cidades, separadas por vírgula")
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil, fmt.Errorf("cidade vazia na lista; não deixe itens em branco entre vírgulas")
		}
		out = append(out, p)
	}
	if len(out) < 2 {
		return nil, fmt.Errorf("informe no mínimo 2 cidades (você informou %d)", len(out))
	}
	return out, nil
}

// FormatRoute junta cidades com a seta usada na CLI.
func FormatRoute(cities []string) string {
	return strings.Join(cities, " → ")
}

// RouteFromLegs monta a rota completa de um itinerário/reserva a partir dos trechos.
func RouteFromLegs(origins, destinations []string) []string {
	if len(origins) == 0 || len(origins) != len(destinations) {
		return nil
	}
	out := []string{origins[0]}
	for i := range destinations {
		out = append(out, destinations[i])
	}
	return out
}
