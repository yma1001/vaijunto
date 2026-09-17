package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/yma1001/vaijunto/internal/client"
	"github.com/yma1001/vaijunto/internal/protocol"
	"github.com/yma1001/vaijunto/internal/server"
)

// runCLI alimenta o menu do passageiro com um script de linhas e captura a saída.
func runCLI(t *testing.T, c *client.Client, script string) string {
	t.Helper()
	var out bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		run(c, strings.NewReader(script), &out)
	}()
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatalf("CLI hung\n%s", out.String())
	}
	return out.String()
}

// TestPassengerLogoutReturnsToMenuSwitchesUserAndZeroExits: logout não mata o processo; troca de conta funciona.
func TestPassengerLogoutReturnsToMenuSwitchesUserAndZeroExits(t *testing.T) {
	cfg, st, _ := server.StartTestServer(t)
	d, err := client.Dial(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Login("motorista1", "senha123"); err != nil {
		t.Fatal(err)
	}
	var ride protocol.RideView
	if err := d.MustOK(protocol.OpPublishRide, protocol.PublishRideData{
		Cities: []string{"Salvador", "Jequié"}, DepartureDate: "2026-10-10",
		DepartureTime: "08:00", Capacity: 2, SegmentPrices: []int64{1500},
	}, &ride); err != nil {
		t.Fatal(err)
	}
	_ = d.Close()

	for _, nl := range []string{"\n", "\r\n"} {
		c, err := client.Dial(cfg)
		if err != nil {
			t.Fatal(err)
		}
		script := strings.Join([]string{
			"1", "passageiro1", "senha123",
			"1", "Salvador", "Jequié", "10/10/2026",
			"6",
			"1", "passageiro2", "senha123",
			"2",
			"0",
		}, nl) + nl
		out := runCLI(t, c, script)
		_ = c.Close()
		if strings.Count(out, "1) entrar") < 2 {
			t.Fatalf("newline %q: logout should return to initial menu\n%s", nl, out)
		}
		if !strings.Contains(out, "ok, autenticado como passageiro1") {
			t.Fatalf("newline %q: missing first login\n%s", nl, out)
		}
		if !strings.Contains(out, "ok, autenticado como passageiro2") {
			t.Fatalf("newline %q: missing switch user\n%s", nl, out)
		}
		if !strings.Contains(out, "faça uma busca primeiro") {
			t.Fatalf("newline %q: last search must be cleared\n%s", nl, out)
		}
		if strings.Contains(out, "opção inválida") {
			t.Fatalf("newline %q: 6 must not be opção inválida\n%s", nl, out)
		}
		if nl == "\n" && st.ListDriverRides("user-driver-1")[0].RideID != ride.RideID {
			t.Fatal("logout must not delete rides")
		}
	}
}

// TestPassengerZeroFromInitialMenuExits: 0 no menu inicial encerra o processo.
func TestPassengerZeroFromInitialMenuExits(t *testing.T) {
	cfg, _, _ := server.StartTestServer(t)
	c, err := client.Dial(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	out := runCLI(t, c, "0\n")
	if !strings.Contains(out, "1) entrar") {
		t.Fatalf("expected initial menu\n%s", out)
	}
	if strings.Contains(out, "6) logout") {
		t.Fatalf("0 should exit before authenticated menu\n%s", out)
	}
}

// TestPassengerUnknownOptionIsInvalidNotExit: opção inválida não encerra o cliente.
func TestPassengerUnknownOptionIsInvalidNotExit(t *testing.T) {
	cfg, _, _ := server.StartTestServer(t)
	c, err := client.Dial(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	out := runCLI(t, c, "x\n0\n")
	if !strings.Contains(out, "opção inválida") {
		t.Fatalf("expected opção inválida\n%s", out)
	}
	if strings.Count(out, "1) entrar") < 2 {
		t.Fatalf("invalid option must stay in the menu\n%s", out)
	}
}
