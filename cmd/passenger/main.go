// Cliente passageiro (CLI). Busca itinerários, confirma, lista e cancela reservas.
// A busca não segura vaga: o CONFIRM reenvia as legs e o servidor revalida.
package main

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/yma1001/vaijunto/internal/client"
	"github.com/yma1001/vaijunto/internal/cliui"
	"github.com/yma1001/vaijunto/internal/config"
	"github.com/yma1001/vaijunto/internal/protocol"
)

// main conecta em SERVER_HOST:SERVER_PORT e entra no menu do passageiro.
func main() {
	cfg := config.Load()
	fmt.Printf("VAIJUNTO — cliente PASSAGEIRO\nServidor: %s\n\n", cfg.ServerAddr())
	c, err := client.Dial(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "conexão recusada: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()
	run(c, os.Stdin, os.Stdout)
}

// run é o loop dos menus do passageiro. lastSearch/lastReservations são cache de tela, não o Store.
func run(c *client.Client, in io.Reader, out io.Writer) {
	prompt := cliui.NewPrompter(in, out)
	read := prompt.Read

	var lastSearch []protocol.ItineraryView
	var lastReservations []protocol.ReservationView

	for {
		if prompt.EOF {
			return
		}
		if c.Role == "" {
			fmt.Fprintln(out, "1) entrar  2) criar conta  3) PING  0) sair")
			choice := read("> ")
			if prompt.EOF {
				return
			}
			switch choice {
			case "1":
				u := read("usuário: ")
				p := read("senha: ")
				if err := c.Login(u, p); err != nil {
					fmt.Fprintln(out, cliui.FriendlyError(err))
					continue
				}
				if c.Role != protocol.RolePassenger {
					fmt.Fprintln(out, "esta conta não é PASSENGER; use o cliente motorista")
					c.Role = ""
					continue
				}
				fmt.Fprintln(out, "ok, autenticado como", c.Name)
			case "2":
				cliui.RegisterInteractive(c, read, protocol.RolePassenger)
			case "3":
				if err := c.Ping(); err != nil {
					fmt.Fprintln(out, cliui.FriendlyError(err))
				} else {
					fmt.Fprintln(out, "PONG")
				}
			case "0":
				return
			default:
				fmt.Fprintln(out, "opção inválida")
			}
			continue
		}

		fmt.Fprintln(out, "1) buscar itinerários  2) confirmar (número da última busca)")
		fmt.Fprintln(out, "3) minhas reservas  4) cancelar reserva  5) ping  6) logout  0) sair")
		choice := read("> ")
		if prompt.EOF {
			return
		}
		switch choice {
		case "1":
			orig := read("origem: ")
			dest := read("destino: ")
			iso, err := cliui.ParseBRDate(read("data (DD/MM/AAAA): "))
			if err != nil {
				fmt.Fprintln(out, err)
				continue
			}
			var searchOut protocol.SearchItinerariesResult
			if err := c.MustOK(protocol.OpSearchItineraries, protocol.SearchItinerariesData{
				Origin: orig, Destination: dest, Date: iso,
			}, &searchOut); err != nil {
				fmt.Fprintln(out, cliui.FriendlyError(err))
				continue
			}
			lastSearch = searchOut.Itineraries
			if len(lastSearch) == 0 {
				fmt.Fprintln(out, "nenhum itinerário encontrado")
				continue
			}
			for i, it := range lastSearch {
				fmt.Fprint(out, cliui.FormatItinerary(i+1, it))
			}
		case "2":
			if len(lastSearch) == 0 {
				fmt.Fprintln(out, "faça uma busca primeiro")
				continue
			}
			ids := make([]string, len(lastSearch))
			for i := range lastSearch {
				ids[i] = strconvIndex(i + 1)
			}
			idx, err := cliui.ResolveListChoice(read("número do itinerário: "), ids)
			if err != nil {
				fmt.Fprintln(out, err)
				continue
			}
			it := lastSearch[idx]
			legs := make([]protocol.ConfirmLeg, 0, len(it.Legs))
			for _, l := range it.Legs {
				legs = append(legs, protocol.ConfirmLeg{RideID: l.RideID, Origin: l.Origin, Destination: l.Destination})
			}
			var conf protocol.ConfirmReservationResult
			if err := c.MustOK(protocol.OpConfirmReservation, protocol.ConfirmReservationData{Legs: legs}, &conf); err != nil {
				fmt.Fprintln(out, cliui.FriendlyError(err))
				continue
			}
			fmt.Fprint(out, cliui.FormatReservation(0, conf.Reservation))
		case "3":
			list, err := loadReservations(c)
			if err != nil {
				fmt.Fprintln(out, cliui.FriendlyError(err))
				continue
			}
			lastReservations = list
			printReservations(out, list)
		case "4":
			list, err := ensureReservations(c, lastReservations)
			if err != nil {
				fmt.Fprintln(out, cliui.FriendlyError(err))
				continue
			}
			lastReservations = list
			printReservations(out, list)
			ids := make([]string, len(list))
			for i, r := range list {
				ids[i] = r.ReservationID
			}
			idx, err := cliui.ResolveListChoice(read("número da reserva: "), ids)
			if err != nil {
				fmt.Fprintln(out, err)
				continue
			}
			fmt.Fprintln(out, "ID:", list[idx].ReservationID)
			var cancelOut protocol.CancelReservationResult
			if err := c.MustOK(protocol.OpCancelReservation, protocol.CancelReservationData{ReservationID: list[idx].ReservationID}, &cancelOut); err != nil {
				fmt.Fprintln(out, cliui.FriendlyError(err))
				continue
			}
			fmt.Fprint(out, cliui.FormatReservation(idx+1, cancelOut.Reservation))
		case "5":
			if err := c.Ping(); err != nil {
				fmt.Fprintln(out, cliui.FriendlyError(err))
			} else {
				fmt.Fprintln(out, "PONG")
			}
		case "6":
			if err := c.Logout(); err != nil {
				fmt.Fprintln(out, cliui.FriendlyError(err))
			}
			lastSearch = nil
			lastReservations = nil
		case "0":
			return
		default:
			fmt.Fprintln(out, "opção inválida")
		}
	}
}

// loadReservations pede LIST_RESERVATIONS ao servidor (o que vale depois de um restart).
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

// ensureReservations usa o cache da tela se existir; senão consulta o servidor.
func ensureReservations(c *client.Client, last []protocol.ReservationView) ([]protocol.ReservationView, error) {
	if len(last) > 0 {
		return last, nil
	}
	return loadReservations(c)
}

// printReservations lista as reservas com data BR e preço em reais.
func printReservations(w io.Writer, list []protocol.ReservationView) {
	if len(list) == 0 {
		fmt.Fprintln(w, "nenhuma reserva")
		return
	}
	for i, r := range list {
		fmt.Fprint(w, cliui.FormatReservation(i+1, r))
	}
}

// strconvIndex vira o índice 1-based da busca numa string para o seletor da CLI.
func strconvIndex(n int) string {
	return fmt.Sprintf("%d", n)
}
