package cliui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/yma1001/vaijunto/internal/domain"
	"github.com/yma1001/vaijunto/internal/protocol"
)

// StatusPT traduz os status do protocolo para rótulos em português.
// O fio continua ACTIVE/CANCELLED/CONFIRMED.
func StatusPT(status string) string {
	switch status {
	case domain.RideActive:
		return "ATIVA"
	case domain.RideCancelled:
		return "CANCELADA"
	case domain.ResConfirmed:
		return "CONFIRMADA"
	default:
		return status
	}
}

// FormatRide imprime a carona para o motorista (rota, vagas, passageiros por trecho).
func FormatRide(index int, r protocol.RideView, passengers *protocol.ListRidePassengersResult) string {
	var b strings.Builder
	if index > 0 {
		fmt.Fprintf(&b, "Carona [%d]\n", index)
	} else {
		b.WriteString("Carona\n")
	}
	fmt.Fprintf(&b, "  ID: %s\n", r.RideID)
	fmt.Fprintf(&b, "  Status: %s\n", StatusPT(r.Status))
	fmt.Fprintf(&b, "  Rota: %s\n", FormatRoute(r.Cities))
	fmt.Fprintf(&b, "  Data: %s\n", FormatBRDate(r.DepartureDate))
	fmt.Fprintf(&b, "  Horário: %s\n", r.DepartureTime)
	fmt.Fprintf(&b, "  Capacidade: %d\n", r.Capacity)
	b.WriteString("  Trechos:\n")
	for i, seg := range r.Segments {
		fmt.Fprintf(&b, "    %d. %s → %s\n", i+1, seg.Origin, seg.Destination)
		fmt.Fprintf(&b, "       Preço: %s\n", FormatBRL(seg.Price))
		fmt.Fprintf(&b, "       Vagas: %d\n", seg.AvailableSeats)
	}
	if passengers != nil {
		b.WriteString("  Passageiros confirmados:\n")
		if len(passengers.Segments) == 0 {
			b.WriteString("    (nenhum)\n")
		}
		for _, seg := range passengers.Segments {
			fmt.Fprintf(&b, "    %s → %s\n", seg.Origin, seg.Destination)
			if len(seg.Passengers) == 0 {
				b.WriteString("      (nenhum)\n")
				continue
			}
			for _, p := range seg.Passengers {
				fmt.Fprintf(&b, "      - %s  reserva: %s\n", p.Username, p.ReservationID)
			}
		}
	}
	return b.String()
}

// FormatItinerary imprime uma opção da busca para o passageiro escolher pelo número.
func FormatItinerary(index int, it protocol.ItineraryView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Itinerário [%d]\n", index)
	origin, dest := "", ""
	if len(it.Legs) > 0 {
		origin = it.Legs[0].Origin
		dest = it.Legs[len(it.Legs)-1].Destination
	}
	fmt.Fprintf(&b, "  Origem: %s\n", origin)
	fmt.Fprintf(&b, "  Destino: %s\n", dest)
	fmt.Fprintf(&b, "  Rota: %s\n", FormatRoute(itineraryCities(it)))
	if len(it.Legs) > 0 {
		fmt.Fprintf(&b, "  Data: %s\n", FormatBRDate(it.Legs[0].DepartureDate))
		fmt.Fprintf(&b, "  Horário: %s\n", it.Legs[0].DepartureTime)
	}
	fmt.Fprintf(&b, "  Preço total: %s\n", FormatBRL(it.TotalPrice))
	fmt.Fprintf(&b, "  Baldeações: %d\n", it.Transfers)
	b.WriteString("  Trechos:\n")
	for i, leg := range it.Legs {
		fmt.Fprintf(&b, "    %d. %s → %s\n", i+1, leg.Origin, leg.Destination)
		fmt.Fprintf(&b, "       Carona: %s\n", leg.RideID)
		fmt.Fprintf(&b, "       Data: %s  Horário: %s\n", FormatBRDate(leg.DepartureDate), leg.DepartureTime)
		fmt.Fprintf(&b, "       Preço: %s\n", FormatBRL(leg.Price))
		fmt.Fprintf(&b, "       Vagas: %d\n", leg.AvailableSeats)
	}
	return b.String()
}

// FormatReservation imprime uma reserva confirmada ou cancelada, sem JSON cru.
func FormatReservation(index int, r protocol.ReservationView) string {
	var b strings.Builder
	if index > 0 {
		fmt.Fprintf(&b, "Reserva [%d]\n", index)
	} else {
		b.WriteString("Reserva\n")
	}
	fmt.Fprintf(&b, "  ID: %s\n", r.ReservationID)
	fmt.Fprintf(&b, "  Status: %s\n", StatusPT(r.Status))
	fmt.Fprintf(&b, "  Rota: %s\n", FormatRoute(reservationCities(r)))
	if len(r.Legs) > 0 {
		fmt.Fprintf(&b, "  Data: %s\n", FormatBRDate(r.Legs[0].DepartureDate))
		fmt.Fprintf(&b, "  Horário: %s\n", r.Legs[0].DepartureTime)
	}
	fmt.Fprintf(&b, "  Preço total: %s\n", FormatBRL(r.TotalPrice))
	b.WriteString("  Trechos:\n")
	for i, leg := range r.Legs {
		fmt.Fprintf(&b, "    %d. %s → %s\n", i+1, leg.Origin, leg.Destination)
		fmt.Fprintf(&b, "       Carona: %s\n", leg.RideID)
		fmt.Fprintf(&b, "       Data: %s  Horário: %s\n", FormatBRDate(leg.DepartureDate), leg.DepartureTime)
		fmt.Fprintf(&b, "       Preço: %s\n", FormatBRL(leg.Price))
	}
	fmt.Fprintf(&b, "  Criada em: %s\n", FormatCreatedAt(r.CreatedAt))
	return b.String()
}

// itineraryCities reconstitui a sequência origem→…→destino para o cabeçalho da busca.
func itineraryCities(it protocol.ItineraryView) []string {
	if len(it.Legs) == 0 {
		return nil
	}
	out := []string{it.Legs[0].Origin}
	for _, leg := range it.Legs {
		out = append(out, leg.Destination)
	}
	return out
}

// reservationCities reconstitui a rota da reserva para o cabeçalho da listagem.
func reservationCities(r protocol.ReservationView) []string {
	if len(r.Legs) == 0 {
		return nil
	}
	out := []string{r.Legs[0].Origin}
	for _, leg := range r.Legs {
		out = append(out, leg.Destination)
	}
	return out
}

// ResolveListChoice interpreta o número 1-based da lista ou um ID real.
func ResolveListChoice(input string, ids []string) (int, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return -1, fmt.Errorf("informe o número da lista")
	}
	if n, err := strconv.Atoi(input); err == nil {
		if n < 1 || n > len(ids) {
			return -1, fmt.Errorf("número fora da lista (1 a %d)", len(ids))
		}
		return n - 1, nil
	}
	for i, id := range ids {
		if id == input {
			return i, nil
		}
	}
	return -1, fmt.Errorf("não encontrei esse item na lista; use o número exibido")
}

// ValidateRegisterInput cobre as checagens da CLI antes de chamar REGISTER.
func ValidateRegisterInput(user, pass, confirm string) error {
	if strings.TrimSpace(user) == "" {
		return fmt.Errorf("informe o usuário")
	}
	if pass == "" {
		return fmt.Errorf("informe a senha")
	}
	if confirm != pass {
		return fmt.Errorf("a confirmação da senha não confere")
	}
	return nil
}

// FriendlyError traduz falhas de protocolo/rede para a CLI.
func FriendlyError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "username already exists"):
		return "este usuário já existe"
	case strings.Contains(msg, "role must be DRIVER or PASSENGER"):
		return "papel inválido; use DRIVER ou PASSENGER"
	case strings.Contains(msg, "username and password required"):
		return "usuário e senha são obrigatórios"
	case strings.Contains(msg, "invalid credentials"):
		return "usuário ou senha incorretos"
	case isCommsMessage(msg):
		return "falha de comunicação com o servidor"
	default:
		return msg
	}
}

// isCommsMessage reconhece erros de socket para a CLI mostrar "falha de comunicação".
func isCommsMessage(msg string) bool {
	needles := []string{
		"connection refused",
		"i/o timeout",
		"EOF",
		"broken pipe",
		"dial tcp",
		"use of closed network",
		"wsarecv",
		"wsasend",
	}
	for _, n := range needles {
		if strings.Contains(msg, n) {
			return true
		}
	}
	return false
}
