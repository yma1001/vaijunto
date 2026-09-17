// smoke publica uma carona com 1 vaga e dois passageiros tentam confirmar.
// Esperado: um OK e um NO_SEATS. Usado pelo scripts/smoke.sh e pelo Docker.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/yma1001/vaijunto/internal/client"
	"github.com/yma1001/vaijunto/internal/config"
	"github.com/yma1001/vaijunto/internal/protocol"
)

// smoke é um cliente não-interativo para provar servidor+protocolo+reserva.
func main() {
	cfg := config.Load()
	deadline := time.Now().Add(20 * time.Second)
	var d *client.Client
	var err error
	for time.Now().Before(deadline) {
		d, err = client.Dial(cfg)
		if err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil {
		fatal("dial server: %v", err)
	}
	if err := d.Login("motorista1", "senha123"); err != nil {
		fatal("login driver: %v", err)
	}
	date := "2026-12-01"
	var ride protocol.RideView
	if err := d.MustOK(protocol.OpPublishRide, protocol.PublishRideData{
		Cities:        []string{"Salvador", "Feira de Santana", "Jequié"},
		DepartureDate: date,
		DepartureTime: "08:00",
		Capacity:      1,
		SegmentPrices: []int64{1000, 1500},
	}, &ride); err != nil {
		fatal("publish: %v", err)
	}
	_ = d.Close()

	ok, fail := 0, 0
	for _, user := range []string{"passageiro1", "passageiro2"} {
		c, err := client.Dial(cfg)
		if err != nil {
			fatal("dial passenger: %v", err)
		}
		if err := c.Login(user, "senha123"); err != nil {
			fatal("login %s: %v", user, err)
		}
		resp, err := c.Call(protocol.OpConfirmReservation, fmt.Sprintf("smoke-%s-%d", user, time.Now().UnixNano()), protocol.ConfirmReservationData{
			Legs: []protocol.ConfirmLeg{{RideID: ride.RideID, Origin: "Salvador", Destination: "Jequié"}},
		})
		_ = c.Close()
		if err == nil && resp.Status == protocol.StatusOK {
			ok++
		} else {
			fail++
		}
	}
	out := map[string]any{"rideId": ride.RideID, "success": ok, "fail": fail, "expect": "success=1 fail=1"}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
	if ok != 1 {
		os.Exit(1)
	}
	fmt.Println("SMOKE OK")
}

// fatal aborta o smoke com mensagem em stderr (falha de dial, login ou publish).
func fatal(f string, a ...any) {
	fmt.Fprintf(os.Stderr, f+"\n", a...)
	os.Exit(1)
}
