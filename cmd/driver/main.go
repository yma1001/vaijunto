package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yma1001/vaijunto/internal/client"
	"github.com/yma1001/vaijunto/internal/cliui"
	"github.com/yma1001/vaijunto/internal/config"
	"github.com/yma1001/vaijunto/internal/protocol"
)

func main() {
	cfg := config.Load()
	fmt.Printf("VAIJUNTO — cliente MOTORISTA\nServidor: %s\n\n", cfg.ServerAddr())
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

	var lastRides []protocol.RideView

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
				if c.Role != protocol.RoleDriver {
					fmt.Println("esta conta não é DRIVER; use o cliente passageiro")
					c.Role = ""
					continue
				}
				fmt.Println("ok, autenticado como", c.Name)
			case "2":
				cliui.RegisterInteractive(c, read, protocol.RoleDriver)
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

		fmt.Println("1) publicar carona  2) minhas caronas  3) passageiros da carona")
		fmt.Println("4) cancelar carona  5) ping  6) logout  0) sair")
		switch read("> ") {
		case "1":
			fmt.Print(cliui.CitiesPrompt)
			cities, err := cliui.ParseCities(read(""))
			if err != nil {
				fmt.Println(err)
				continue
			}
			iso, err := cliui.ParseBRDate(read("data (DD/MM/AAAA): "))
			if err != nil {
				fmt.Println(err)
				continue
			}
			hora := read("horário (HH:MM): ")
			if _, err := time.Parse("15:04", hora); err != nil {
				fmt.Println("horário inválido: use HH:MM (exemplo: 08:00)")
				continue
			}
			cap, err := strconv.Atoi(read("assentos: "))
			if err != nil || cap < 1 {
				fmt.Println("informe um número de assentos maior que zero")
				continue
			}
			prices := make([]int64, 0, len(cities)-1)
			okPrices := true
			for i := 0; i < len(cities)-1; i++ {
				raw := read(fmt.Sprintf("Preço %s → %s (R$): ", cities[i], cities[i+1]))
				cents, err := cliui.ParseBRLToCents(raw)
				if err != nil {
					fmt.Println(err)
					okPrices = false
					break
				}
				prices = append(prices, cents)
			}
			if !okPrices {
				continue
			}
			var out protocol.RideView
			if err := c.MustOK(protocol.OpPublishRide, protocol.PublishRideData{
				Cities: cities, DepartureDate: iso, DepartureTime: hora, Capacity: cap, SegmentPrices: prices,
			}, &out); err != nil {
				fmt.Println(cliui.FriendlyError(err))
				continue
			}
			fmt.Print(cliui.FormatRide(0, out, nil))
		case "2":
			rides, err := loadDriverRides(c)
			if err != nil {
				fmt.Println(cliui.FriendlyError(err))
				continue
			}
			lastRides = rides
			printDriverRides(c, rides)
		case "3":
			rides, err := ensureRides(c, lastRides)
			if err != nil {
				fmt.Println(cliui.FriendlyError(err))
				continue
			}
			lastRides = rides
			printDriverRides(c, rides)
			idx, err := pickRide(read("número da carona: "), rides)
			if err != nil {
				fmt.Println(err)
				continue
			}
			pass, err := loadPassengers(c, rides[idx].RideID)
			if err != nil {
				fmt.Println(cliui.FriendlyError(err))
				continue
			}
			fmt.Print(cliui.FormatRide(idx+1, rides[idx], pass))
		case "4":
			rides, err := ensureRides(c, lastRides)
			if err != nil {
				fmt.Println(cliui.FriendlyError(err))
				continue
			}
			lastRides = rides
			printDriverRides(c, rides)
			idx, err := pickRide(read("número da carona: "), rides)
			if err != nil {
				fmt.Println(err)
				continue
			}
			fmt.Println("ID:", rides[idx].RideID)
			var out protocol.RideView
			if err := c.MustOK(protocol.OpCancelRide, protocol.CancelRideData{RideID: rides[idx].RideID}, &out); err != nil {
				fmt.Println(cliui.FriendlyError(err))
				continue
			}
			fmt.Print(cliui.FormatRide(idx+1, out, nil))
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

func loadDriverRides(c *client.Client) ([]protocol.RideView, error) {
	var out protocol.ListDriverRidesResult
	if err := c.MustOK(protocol.OpListDriverRides, map[string]any{}, &out); err != nil {
		return nil, err
	}
	sort.Slice(out.Rides, func(i, j int) bool { return out.Rides[i].RideID < out.Rides[j].RideID })
	return out.Rides, nil
}

func ensureRides(c *client.Client, last []protocol.RideView) ([]protocol.RideView, error) {
	if len(last) > 0 {
		return last, nil
	}
	return loadDriverRides(c)
}

func printDriverRides(c *client.Client, rides []protocol.RideView) {
	if len(rides) == 0 {
		fmt.Println("nenhuma carona publicada")
		return
	}
	for i, r := range rides {
		pass, _ := loadPassengers(c, r.RideID)
		fmt.Print(cliui.FormatRide(i+1, r, pass))
	}
}

func loadPassengers(c *client.Client, rideID string) (*protocol.ListRidePassengersResult, error) {
	var out protocol.ListRidePassengersResult
	if err := c.MustOK(protocol.OpListRidePassengers, protocol.ListRidePassengersData{RideID: rideID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

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
