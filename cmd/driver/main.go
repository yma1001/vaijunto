// Cliente motorista (CLI). Publica carona, lista, vê passageiros por trecho e cancela.
// Fala o mesmo protocolo TCP do passageiro; o papel DRIVER restringe as operações.
package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/yma1001/vaijunto/internal/client"
	"github.com/yma1001/vaijunto/internal/cliui"
	"github.com/yma1001/vaijunto/internal/config"
	"github.com/yma1001/vaijunto/internal/protocol"
)

// main conecta em SERVER_HOST:SERVER_PORT e entra no menu do motorista.
func main() {
	cfg := config.Load()
	fmt.Printf("VAIJUNTO — cliente MOTORISTA\nServidor: %s\n\n", cfg.ServerAddr())
	c, err := client.Dial(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "conexão recusada: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()
	run(c, os.Stdin, os.Stdout)
}

// run é o loop dos menus (entrada/cadastro e, depois do LOGIN, as operações de motorista).
func run(c *client.Client, in io.Reader, out io.Writer) {
	prompt := cliui.NewPrompter(in, out)
	read := prompt.Read

	var lastRides []protocol.RideView

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
				if c.Role != protocol.RoleDriver {
					fmt.Fprintln(out, "esta conta não é DRIVER; use o cliente passageiro")
					c.Role = ""
					continue
				}
				fmt.Fprintln(out, "ok, autenticado como", c.Name)
			case "2":
				cliui.RegisterInteractive(c, read, protocol.RoleDriver)
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

		fmt.Fprintln(out, "1) publicar carona  2) minhas caronas  3) passageiros da carona")
		fmt.Fprintln(out, "4) cancelar carona  5) ping  6) logout  0) sair")
		choice := read("> ")
		if prompt.EOF {
			return
		}
		switch choice {
		case "1":
			fmt.Fprint(out, cliui.CitiesPrompt)
			cities, err := cliui.ParseCities(read(""))
			if err != nil {
				fmt.Fprintln(out, err)
				continue
			}
			iso, err := cliui.ParseBRDate(read("data (DD/MM/AAAA): "))
			if err != nil {
				fmt.Fprintln(out, err)
				continue
			}
			hora := read("horário (HH:MM): ")
			if _, err := time.Parse("15:04", hora); err != nil {
				fmt.Fprintln(out, "horário inválido: use HH:MM (exemplo: 08:00)")
				continue
			}
			cap, err := strconv.Atoi(read("assentos: "))
			if err != nil || cap < 1 {
				fmt.Fprintln(out, "informe um número de assentos maior que zero")
				continue
			}
			prices := make([]int64, 0, len(cities)-1)
			okPrices := true
			for i := 0; i < len(cities)-1; i++ {
				raw := read(fmt.Sprintf("Preço %s → %s (R$): ", cities[i], cities[i+1]))
				cents, err := cliui.ParseBRLToCents(raw)
				if err != nil {
					fmt.Fprintln(out, err)
					okPrices = false
					break
				}
				prices = append(prices, cents)
			}
			if !okPrices {
				continue
			}
			var ride protocol.RideView
			if err := c.MustOK(protocol.OpPublishRide, protocol.PublishRideData{
				Cities: cities, DepartureDate: iso, DepartureTime: hora, Capacity: cap, SegmentPrices: prices,
			}, &ride); err != nil {
				fmt.Fprintln(out, cliui.FriendlyError(err))
				continue
			}
			fmt.Fprint(out, cliui.FormatRide(0, ride, nil))
		case "2":
			rides, err := loadDriverRides(c)
			if err != nil {
				fmt.Fprintln(out, cliui.FriendlyError(err))
				continue
			}
			lastRides = rides
			printDriverRides(out, c, rides)
		case "3":
			rides, err := ensureRides(c, lastRides)
			if err != nil {
				fmt.Fprintln(out, cliui.FriendlyError(err))
				continue
			}
			lastRides = rides
			printDriverRides(out, c, rides)
			idx, err := pickRide(read("número da carona: "), rides)
			if err != nil {
				fmt.Fprintln(out, err)
				continue
			}
			pass, err := loadPassengers(c, rides[idx].RideID)
			if err != nil {
				fmt.Fprintln(out, cliui.FriendlyError(err))
				continue
			}
			fmt.Fprint(out, cliui.FormatRide(idx+1, rides[idx], pass))
		case "4":
			rides, err := ensureRides(c, lastRides)
			if err != nil {
				fmt.Fprintln(out, cliui.FriendlyError(err))
				continue
			}
			lastRides = rides
			printDriverRides(out, c, rides)
			idx, err := pickRide(read("número da carona: "), rides)
			if err != nil {
				fmt.Fprintln(out, err)
				continue
			}
			fmt.Fprintln(out, "ID:", rides[idx].RideID)
			var cancelled protocol.RideView
			if err := c.MustOK(protocol.OpCancelRide, protocol.CancelRideData{RideID: rides[idx].RideID}, &cancelled); err != nil {
				fmt.Fprintln(out, cliui.FriendlyError(err))
				continue
			}
			fmt.Fprint(out, cliui.FormatRide(idx+1, cancelled, nil))
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
			lastRides = nil
		case "0":
			return
		default:
			fmt.Fprintln(out, "opção inválida")
		}
	}
}

// loadDriverRides pede LIST_DRIVER_RIDES ao servidor (não usa cache local como fonte de verdade).
func loadDriverRides(c *client.Client) ([]protocol.RideView, error) {
	var out protocol.ListDriverRidesResult
	if err := c.MustOK(protocol.OpListDriverRides, map[string]any{}, &out); err != nil {
		return nil, err
	}
	sort.Slice(out.Rides, func(i, j int) bool { return out.Rides[i].RideID < out.Rides[j].RideID })
	return out.Rides, nil
}

// ensureRides reaproveita a última listagem se o motorista acabou de ver as caronas.
func ensureRides(c *client.Client, last []protocol.RideView) ([]protocol.RideView, error) {
	if len(last) > 0 {
		return last, nil
	}
	return loadDriverRides(c)
}

// printDriverRides mostra cada carona com os passageiros confirmados por trecho.
func printDriverRides(w io.Writer, c *client.Client, rides []protocol.RideView) {
	if len(rides) == 0 {
		fmt.Fprintln(w, "nenhuma carona publicada")
		return
	}
	for i, r := range rides {
		pass, _ := loadPassengers(c, r.RideID)
		fmt.Fprint(w, cliui.FormatRide(i+1, r, pass))
	}
}

// loadPassengers pede os confirmados daquela carona, agrupados por trecho.
func loadPassengers(c *client.Client, rideID string) (*protocol.ListRidePassengersResult, error) {
	var out protocol.ListRidePassengersResult
	if err := c.MustOK(protocol.OpListRidePassengers, protocol.ListRidePassengersData{RideID: rideID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// pickRide interpreta o número (ou o rideId) que o motorista digitou na lista.
func pickRide(input string, rides []protocol.RideView) (int, error) {
	if len(rides) == 0 {
		return -1, fmt.Errorf("nenhuma carona na lista")
	}
	ids := make([]string, len(rides))
	for i, r := range rides {
		ids[i] = r.RideID
	}
	return cliui.ResolveListChoice(input, ids)
}
