package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yma1001/vaijunto/internal/client"
	"github.com/yma1001/vaijunto/internal/config"
	"github.com/yma1001/vaijunto/internal/protocol"
)

// loadtest dispara N clientes TCP reais disputando as mesmas vagas
// e imprime sucessos, falhas, latência média e p95.
func main() {
	cfg := config.Load()
	n := 20
	if v := os.Getenv("LOADTEST_CLIENTS"); v != "" {
		fmt.Sscanf(v, "%d", &n)
	}

	admin, err := client.Dial(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial: %v\n", err)
		os.Exit(1)
	}
	if err := admin.Login("motorista1", "senha123"); err != nil {
		fmt.Fprintf(os.Stderr, "login driver: %v\n", err)
		os.Exit(1)
	}
	date := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	var ride protocol.RideView
	if err := admin.MustOK(protocol.OpPublishRide, protocol.PublishRideData{
		Cities: []string{"Salvador", "Feira de Santana"}, DepartureDate: date, DepartureTime: "08:00",
		Capacity: 1, SegmentPrices: []int64{1000},
	}, &ride); err != nil {
		fmt.Fprintf(os.Stderr, "publish: %v\n", err)
		os.Exit(1)
	}
	_ = admin.Close()

	var okN, failN atomic.Int64
	lat := make([]time.Duration, n)
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			user := "passageiro1"
			if i%2 == 1 {
				user = "passageiro2"
			}
			c, err := client.Dial(cfg)
			if err != nil {
				failN.Add(1)
				return
			}
			defer c.Close()
			if err := c.Login(user, "senha123"); err != nil {
				failN.Add(1)
				return
			}
			t0 := time.Now()
			resp, err := c.Call(protocol.OpConfirmReservation, fmt.Sprintf("load-%d", i), protocol.ConfirmReservationData{
				Legs: []protocol.ConfirmLeg{{RideID: ride.RideID, Origin: "Salvador", Destination: "Feira de Santana"}},
			})
			lat[i] = time.Since(t0)
			if err != nil || resp.Status != protocol.StatusOK {
				failN.Add(1)
				return
			}
			okN.Add(1)
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	cp := append([]time.Duration(nil), lat...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	var sum time.Duration
	for _, d := range cp {
		sum += d
	}
	avg := time.Duration(0)
	if n > 0 {
		avg = sum / time.Duration(n)
	}
	p95 := cp[(len(cp)*95)/100]
	report := map[string]any{
		"clients": n, "success": okN.Load(), "fail": failN.Load(),
		"elapsed_ms": elapsed.Milliseconds(), "avg_ms": avg.Milliseconds(),
		"p95_ms": p95.Milliseconds(),
		"throughput_ops_s": float64(n) / elapsed.Seconds(),
		"note":             "com capacity=1, success esperado = 1",
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
}
