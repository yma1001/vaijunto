package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/yma1001/vaijunto/internal/client"
	"github.com/yma1001/vaijunto/internal/config"
	"github.com/yma1001/vaijunto/internal/protocol"
)

func main() {
	cfg := config.Load()
	fmt.Printf("VAIJUNTO — cliente PASSAGEIRO\nServidor: %s\n\n", cfg.ServerAddr())
	c, err := client.Dial(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "conexão recusada: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	in := bufio.NewScanner(os.Stdin)
	read := func(prompt string) string {
		fmt.Print(prompt)
		if !in.Scan() {
			return ""
		}
		return strings.TrimSpace(in.Text())
	}

	var last []protocol.ItineraryView

	for {
		if c.Role == "" {
			fmt.Println("1) LOGIN  2) PING  0) sair")
			switch read("> ") {
			case "1":
				u := read("usuário: ")
				p := read("senha: ")
				if err := c.Login(u, p); err != nil {
					fmt.Println("erro:", err)
					continue
				}
				if c.Role != protocol.RolePassenger {
					fmt.Println("esta conta não é PASSENGER; use o cliente motorista")
					c.Role = ""
					continue
				}
				fmt.Println("ok, autenticado como", c.Name)
			case "2":
				if err := c.Ping(); err != nil {
					fmt.Println("erro:", err)
				} else {
					fmt.Println("PONG")
				}
			default:
				return
			}
			continue
		}

		fmt.Println("1) buscar itinerários  2) confirmar (índice da última busca)")
		fmt.Println("3) minhas reservas  4) cancelar reserva  5) ping  6) logout  0) sair")
		switch read("> ") {
		case "1":
			orig := read("origem: ")
			dest := read("destino: ")
			date := read("data YYYY-MM-DD: ")
			var out protocol.SearchItinerariesResult
			if err := c.MustOK(protocol.OpSearchItineraries, protocol.SearchItinerariesData{
				Origin: orig, Destination: dest, Date: date,
			}, &out); err != nil {
				fmt.Println("erro:", err)
				continue
			}
			last = out.Itineraries
			if len(last) == 0 {
				fmt.Println("nenhum itinerário encontrado")
				continue
			}
			for i, it := range last {
				fmt.Printf("[%d] preço=%d centavos  baldeações=%d  trechos=%d\n", i, it.TotalPrice, it.Transfers, len(it.Legs))
				for _, l := range it.Legs {
					fmt.Printf("    %s → %s  ride=%s  vagas=%d  preço=%d\n", l.Origin, l.Destination, l.RideID, l.AvailableSeats, l.Price)
				}
			}
		case "2":
			if len(last) == 0 {
				fmt.Println("faça uma busca primeiro")
				continue
			}
			idx, _ := strconv.Atoi(read("índice: "))
			if idx < 0 || idx >= len(last) {
				fmt.Println("índice inválido")
				continue
			}
			it := last[idx]
			legs := make([]protocol.ConfirmLeg, 0, len(it.Legs))
			for _, l := range it.Legs {
				legs = append(legs, protocol.ConfirmLeg{RideID: l.RideID, Origin: l.Origin, Destination: l.Destination})
			}
			var out protocol.ConfirmReservationResult
			if err := c.MustOK(protocol.OpConfirmReservation, protocol.ConfirmReservationData{Legs: legs}, &out); err != nil {
				fmt.Println("erro:", err)
				continue
			}
			printJSON(out)
		case "3":
			var out protocol.ListReservationsResult
			if err := c.MustOK(protocol.OpListReservations, map[string]any{}, &out); err != nil {
				fmt.Println("erro:", err)
				continue
			}
			printJSON(out)
		case "4":
			id := read("reservationId: ")
			var out protocol.CancelReservationResult
			if err := c.MustOK(protocol.OpCancelReservation, protocol.CancelReservationData{ReservationID: id}, &out); err != nil {
				fmt.Println("erro:", err)
				continue
			}
			printJSON(out)
		case "5":
			if err := c.Ping(); err != nil {
				fmt.Println("erro:", err)
			} else {
				fmt.Println("PONG")
			}
		case "6":
			_ = c.MustOK(protocol.OpLogout, map[string]any{}, nil)
			return
		default:
			return
		}
	}
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}
