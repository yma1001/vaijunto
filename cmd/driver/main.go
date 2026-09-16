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
				if c.Role != protocol.RoleDriver {
					fmt.Println("esta conta não é DRIVER; use o cliente passageiro")
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

		fmt.Println("1) publicar carona  2) minhas caronas  3) passageiros da carona")
		fmt.Println("4) cancelar carona  5) ping  6) logout  0) sair")
		switch read("> ") {
		case "1":
			cities := splitCSV(read("cidades (A, B, C): "))
			date := read("data YYYY-MM-DD: ")
			hora := read("horário HH:MM: ")
			cap, _ := strconv.Atoi(read("assentos: "))
			prices := parsePrices(read("preços em centavos, um por trecho (ex. 1500,2000): "))
			var out protocol.RideView
			if err := c.MustOK(protocol.OpPublishRide, protocol.PublishRideData{
				Cities: cities, DepartureDate: date, DepartureTime: hora, Capacity: cap, SegmentPrices: prices,
			}, &out); err != nil {
				fmt.Println("erro:", err)
				continue
			}
			printJSON(out)
		case "2":
			var out protocol.ListDriverRidesResult
			if err := c.MustOK(protocol.OpListDriverRides, map[string]any{}, &out); err != nil {
				fmt.Println("erro:", err)
				continue
			}
			printJSON(out)
		case "3":
			id := read("rideId: ")
			var out protocol.ListRidePassengersResult
			if err := c.MustOK(protocol.OpListRidePassengers, protocol.ListRidePassengersData{RideID: id}, &out); err != nil {
				fmt.Println("erro:", err)
				continue
			}
			printJSON(out)
		case "4":
			id := read("rideId: ")
			var out protocol.RideView
			if err := c.MustOK(protocol.OpCancelRide, protocol.CancelRideData{RideID: id}, &out); err != nil {
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

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parsePrices(s string) []int64 {
	var out []int64
	for _, p := range splitCSV(s) {
		n, _ := strconv.ParseInt(p, 10, 64)
		out = append(out, n)
	}
	return out
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}
