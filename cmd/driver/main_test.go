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

func TestDriverLogoutReturnsToMenuSwitchesUserAndZeroExits(t *testing.T) {
	cfg, st, _ := server.StartTestServer(t)
	setup, err := client.Dial(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := setup.Login("motorista1", "senha123"); err != nil {
		t.Fatal(err)
	}
	var ride protocol.RideView
	if err := setup.MustOK(protocol.OpPublishRide, protocol.PublishRideData{
		Cities: []string{"Salvador", "Jequié"}, DepartureDate: "2026-10-10",
		DepartureTime: "08:00", Capacity: 2, SegmentPrices: []int64{1500},
	}, &ride); err != nil {
		t.Fatal(err)
	}
	_ = setup.Close()

	for _, nl := range []string{"\n", "\r\n"} {
		c, err := client.Dial(cfg)
		if err != nil {
			t.Fatal(err)
		}
		script := strings.Join([]string{
			"1", "motorista1", "senha123",
			"2",
			"6",
			"1", "motorista2", "senha123",
			"2",
			"0",
		}, nl) + nl
		out := runCLI(t, c, script)
		_ = c.Close()
		if strings.Count(out, "1) entrar") < 2 {
			t.Fatalf("newline %q: logout should return to initial menu\n%s", nl, out)
		}
		if !strings.Contains(out, "ok, autenticado como motorista1") {
			t.Fatalf("newline %q: missing first login\n%s", nl, out)
		}
		if !strings.Contains(out, "ok, autenticado como motorista2") {
			t.Fatalf("newline %q: missing switch user\n%s", nl, out)
		}
		if !strings.Contains(out, ride.RideID) {
			t.Fatalf("newline %q: motorista1 should still see own ride\n%s", nl, out)
		}
		after := out
		if i := strings.LastIndex(out, "ok, autenticado como motorista2"); i >= 0 {
			after = out[i:]
		}
		if strings.Contains(after, ride.RideID) {
			t.Fatalf("newline %q: last rides cache leaked to motorista2\n%s", nl, after)
		}
		if !strings.Contains(after, "nenhuma carona publicada") {
			t.Fatalf("newline %q: motorista2 should have empty list\n%s", nl, after)
		}
		if strings.Contains(out, "opção inválida") {
			t.Fatalf("newline %q: 6 must not be opção inválida\n%s", nl, out)
		}
		if nl == "\n" {
			rides := st.ListDriverRides("user-driver-1")
			if len(rides) == 0 || rides[0].RideID != ride.RideID {
				t.Fatal("logout must not delete rides")
			}
		}
	}
}

func TestDriverZeroFromInitialMenuExits(t *testing.T) {
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
