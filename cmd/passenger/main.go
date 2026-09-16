package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/yma1001/vaijunto/internal/client"
	"github.com/yma1001/vaijunto/internal/cliui"
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

	var lastSearch []protocol.ItineraryView
	var lastReservations []protocol.ReservationView

	for {
		if c.Role == "" {
			fmt.Println("1) entrar  2) criar conta  3) PING  0) sair")
			switch read("> ") {
			case "1":
				u := read("usuário: ")
				p := read("senha: ")
				if err := c.Login(u, p); err != nil {
					fmt.Println(cliui.FriendlyError(err))
					continue
				}
				if c.Role != protocol.RolePassenger {
					fmt.Println("esta conta não é PASSENGER; use o cliente motorista")
					c.Role = ""
					continue
				}
				fmt.Println("ok, autenticado como", c.Name)
			case "2":
				cliui.RegisterInteractive(c, read, protocol.RolePassenger)
			case "3":
				if err := c.Ping(); err != nil {
					fmt.Println(cliui.FriendlyError(err))
				} else {
					fmt.Println("PONG")
				}
			case "0":
				return
			default:
				fmt.Println("opção inválida")
			}
			continue
		}

		fmt.Println("1) buscar itinerários  2) confirmar (número da última busca)")
		fmt.Println("3) minhas reservas  4) cancelar reserva  5) ping  6) logout  0) sair")
		switch read("> ") {
		case "1":
			orig := read("origem: ")
			dest := read("destino: ")
			iso, err := cliui.ParseBRDate(read("data (DD/MM/AAAA): "))
			if err != nil {
				fmt.Println(err)
				continue
			}
			var out protocol.SearchItinerariesResult
			if err := c.MustOK(protocol.OpSearchItineraries, protocol.SearchItinerariesData{
				Origin: orig, Destination: dest, Date: iso,
			}, &out); err != nil {
				fmt.Println(cliui.FriendlyError(err))
				continue
			}
			lastSearch = out.Itineraries
			if len(lastSearch) == 0 {
				fmt.Println("nenhum itinerário encontrado")
				continue
			}
			for i, it := range lastSearch {
				fmt.Print(cliui.FormatItinerary(i+1, it))
			}
		case "2":
			if len(lastSearch) == 0 {
				fmt.Println("faça uma busca primeiro")
				continue
			}
			ids := make([]string, len(lastSearch))
			for i := range lastSearch {
				ids[i] = strconvIndex(i + 1)
			}
			idx, err := cliui.ResolveListChoice(read("número do itinerário: "), ids)
			if err != nil {
				fmt.Println(err)
				continue
			}
			it := lastSearch[idx]
			legs := make([]protocol.ConfirmLeg, 0, len(it.Legs))
			for _, l := range it.Legs {
				legs = append(legs, protocol.ConfirmLeg{RideID: l.RideID, Origin: l.Origin, Destination: l.Destination})
			}
			var out protocol.ConfirmReservationResult
			if err := c.MustOK(protocol.OpConfirmReservation, protocol.ConfirmReservationData{Legs: legs}, &out); err != nil {
				fmt.Println(cliui.FriendlyError(err))
				continue
			}
			fmt.Print(cliui.FormatReservation(0, out.Reservation))
		case "3":
			list, err := loadReservations(c)
			if err != nil {
				fmt.Println(cliui.FriendlyError(err))
				continue
			}
			lastReservations = list
			printReservations(list)
		case "4":
			list, err := ensureReservations(c, lastReservations)
			if err != nil {
				fmt.Println(cliui.FriendlyError(err))
				continue
			}
			lastReservations = list
			printReservations(list)
			ids := make([]string, len(list))
			for i, r := range list {
				ids[i] = r.ReservationID
			}
			idx, err := cliui.ResolveListChoice(read("número da reserva: "), ids)
			if err != nil {
				fmt.Println(err)
				continue
			}
			fmt.Println("ID:", list[idx].ReservationID)
			var out protocol.CancelReservationResult
			if err := c.MustOK(protocol.OpCancelReservation, protocol.CancelReservationData{ReservationID: list[idx].ReservationID}, &out); err != nil {
				fmt.Println(cliui.FriendlyError(err))
				continue
			}
			fmt.Print(cliui.FormatReservation(idx+1, out.Reservation))
		case "5":
			if err := c.Ping(); err != nil {
				fmt.Println(cliui.FriendlyError(err))
			} else {
				fmt.Println("PONG")
			}
		case "6":
			_ = c.MustOK(protocol.OpLogout, map[string]any{}, nil)
			return
		case "0":
			return
		default:
			fmt.Println("opção inválida")
		}
	}
}

func loadReservations(c *client.Client) ([]protocol.ReservationView, error) {
	var out protocol.ListReservationsResult
	if err := c.MustOK(protocol.OpListReservations, map[string]any{}, &out); err != nil {
		return nil, err
	}
	sort.Slice(out.Reservations, func(i, j int) bool {
		return out.Reservations[i].ReservationID < out.Reservations[j].ReservationID
	})
	return out.Reservations, nil
}

func ensureReservations(c *client.Client, last []protocol.ReservationView) ([]protocol.ReservationView, error) {
	if len(last) > 0 {
		return last, nil
	}
	return loadReservations(c)
}

func printReservations(list []protocol.ReservationView) {
	if len(list) == 0 {
		fmt.Println("nenhuma reserva")
		return
	}
	for i, r := range list {
		fmt.Print(cliui.FormatReservation(i+1, r))
	}
}

func strconvIndex(n int) string {
	return fmt.Sprintf("%d", n)
}
