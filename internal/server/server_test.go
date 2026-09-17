package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yma1001/vaijunto/internal/client"
	"github.com/yma1001/vaijunto/internal/config"
	"github.com/yma1001/vaijunto/internal/protocol"
	"github.com/yma1001/vaijunto/internal/store"
)

func startTestServer(t *testing.T) (config.Config, *store.Store, *Server) {
	return StartTestServer(t)
}

func TestPingPong(t *testing.T) {
	cfg, _, _ := startTestServer(t)
	c, err := client.Dial(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestLoginAndForbiddenRole(t *testing.T) {
	cfg, _, _ := startTestServer(t)
	c, _ := client.Dial(cfg)
	defer c.Close()
	if err := c.Login("passageiro1", "senha123"); err != nil {
		t.Fatal(err)
	}
	resp, err := c.Call(protocol.OpPublishRide, "", protocol.PublishRideData{
		Cities: []string{"A", "B"}, DepartureDate: "2026-10-01", DepartureTime: "08:00", Capacity: 1, SegmentPrices: []int64{1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != protocol.StatusError || resp.Error.Code != protocol.CodeForbidden {
		t.Fatalf("%+v", resp)
	}
}

func TestUnknownOperation(t *testing.T) {
	cfg, _, _ := startTestServer(t)
	c, _ := client.Dial(cfg)
	defer c.Close()
	resp, err := c.Call("EXPLODIR_SERVIDOR", "", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil || resp.Error.Code != protocol.CodeUnknownOperation {
		t.Fatalf("%+v", resp)
	}
	if err := c.Ping(); err != nil {
		t.Fatal("server must survive unknown op")
	}
}

func TestInvalidJSONDoesNotKillServer(t *testing.T) {
	cfg, _, _ := startTestServer(t)
	conn, err := net.Dial("tcp", cfg.ServerAddr())
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("{not-json")
	if err := protocol.WriteFrame(conn, payload); err != nil {
		t.Fatal(err)
	}
	raw, err := protocol.ReadFrame(conn, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	var resp protocol.Response
	_ = json.Unmarshal(raw, &resp)
	if resp.Error == nil || resp.Error.Code != protocol.CodeInvalidJSON {
		t.Fatalf("%+v", resp)
	}
	_ = conn.Close()

	c, _ := client.Dial(cfg)
	defer c.Close()
	if err := c.Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestIncompleteFrameAndAbruptDisconnect(t *testing.T) {
	cfg, _, _ := startTestServer(t)
	conn, err := net.Dial("tcp", cfg.ServerAddr())
	if err != nil {
		t.Fatal(err)
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], 40)
	if _, err := conn.Write(hdr[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte("xxx")); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()

	c, _ := client.Dial(cfg)
	defer c.Close()
	if err := c.Ping(); err != nil {
		t.Fatal("server died after abrupt disconnect")
	}
}

func TestOversizedFrame(t *testing.T) {
	cfg, _, _ := startTestServer(t)
	conn, err := net.Dial("tcp", cfg.ServerAddr())
	if err != nil {
		t.Fatal(err)
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], 50<<20)
	_, _ = conn.Write(hdr[:])
	_ = conn.Close()
	c, _ := client.Dial(cfg)
	defer c.Close()
	if err := c.Ping(); err != nil {
		t.Fatal(err)
	}
}

func publishSample(t *testing.T, cfg config.Config) protocol.RideView {
	t.Helper()
	d, err := client.Dial(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.Login("motorista1", "senha123"); err != nil {
		t.Fatal(err)
	}
	var ride protocol.RideView
	if err := d.MustOK(protocol.OpPublishRide, protocol.PublishRideData{
		Cities:        []string{"Salvador", "Feira de Santana", "Jequié"},
		DepartureDate: "2026-10-10", DepartureTime: "07:30",
		Capacity: 1, SegmentPrices: []int64{1000, 1500},
	}, &ride); err != nil {
		t.Fatal(err)
	}
	return ride
}

func TestSearchAndConfirmAndCancelTCP(t *testing.T) {
	cfg, _, _ := startTestServer(t)
	ride := publishSample(t, cfg)

	p, _ := client.Dial(cfg)
	defer p.Close()
	if err := p.Login("passageiro1", "senha123"); err != nil {
		t.Fatal(err)
	}
	var found protocol.SearchItinerariesResult
	if err := p.MustOK(protocol.OpSearchItineraries, protocol.SearchItinerariesData{
		Origin: "Salvador", Destination: "Jequié", Date: "2026-10-10",
	}, &found); err != nil {
		t.Fatal(err)
	}
	if len(found.Itineraries) == 0 {
		t.Fatal("expected itinerary")
	}
	var conf protocol.ConfirmReservationResult
	if err := p.MustOK(protocol.OpConfirmReservation, protocol.ConfirmReservationData{
		Legs: []protocol.ConfirmLeg{{RideID: ride.RideID, Origin: "Salvador", Destination: "Jequié"}},
	}, &conf); err != nil {
		t.Fatal(err)
	}
	if conf.Reservation.Status != "CONFIRMED" {
		t.Fatal(conf.Reservation.Status)
	}
	var canc protocol.CancelReservationResult
	if err := p.MustOK(protocol.OpCancelReservation, protocol.CancelReservationData{ReservationID: conf.Reservation.ReservationID}, &canc); err != nil {
		t.Fatal(err)
	}
	if err := p.MustOK(protocol.OpCancelReservation, protocol.CancelReservationData{ReservationID: conf.Reservation.ReservationID}, &canc); err != nil {
		t.Fatal("cancel must be idempotent")
	}
}

func TestSimultaneousConfirmTCP(t *testing.T) {
	cfg, st, _ := startTestServer(t)
	ride := publishSample(t, cfg)
	var okN, failN atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c, err := client.Dial(cfg)
			if err != nil {
				failN.Add(1)
				return
			}
			defer c.Close()
			user := "passageiro1"
			if i%2 == 1 {
				user = "passageiro2"
			}
			if err := c.Login(user, "senha123"); err != nil {
				failN.Add(1)
				return
			}
			resp, err := c.Call(protocol.OpConfirmReservation, "", protocol.ConfirmReservationData{
				Legs: []protocol.ConfirmLeg{{RideID: ride.RideID, Origin: "Salvador", Destination: "Jequié"}},
			})
			if err == nil && resp.Status == protocol.StatusOK {
				okN.Add(1)
			} else {
				failN.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if okN.Load() != 1 {
		t.Fatalf("ok=%d fail=%d", okN.Load(), failN.Load())
	}
	if v := st.CheckInvariants(); len(v) > 0 {
		t.Fatal(v)
	}
}

func TestManyClients(t *testing.T) {
	cfg, _, _ := startTestServer(t)
	var wg sync.WaitGroup
	errCh := make(chan error, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := client.Dial(cfg)
			if err != nil {
				errCh <- err
				return
			}
			defer c.Close()
			if err := c.Ping(); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}

func TestPersistenceAcrossServerRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	cfg := config.Load()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	cfg.ServerHost, cfg.ServerPort = host, port
	cfg.ReadTimeout = 3 * time.Second
	cfg.WriteTimeout = 3 * time.Second
	cfg.ConnectTimeout = 2 * time.Second
	silent := log.New(io.Discard, "", 0)
	srv := New(cfg, st, silent)
	go func() { _ = srv.Serve(ln) }()
	ride := publishSample(t, cfg)
	_ = srv.Close()

	st2, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	ln2, _ := net.Listen("tcp", "127.0.0.1:0")
	host, port, _ = net.SplitHostPort(ln2.Addr().String())
	cfg.ServerHost, cfg.ServerPort = host, port
	srv2 := New(cfg, st2, silent)
	go func() { _ = srv2.Serve(ln2) }()
	t.Cleanup(func() { _ = srv2.Close() })

	p, err := client.Dial(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := p.Login("passageiro1", "senha123"); err != nil {
		t.Fatal("session does not persist, but ACCOUNT must: ", err)
	}
	var found protocol.SearchItinerariesResult
	if err := p.MustOK(protocol.OpSearchItineraries, protocol.SearchItinerariesData{
		Origin: "Salvador", Destination: "Jequié", Date: "2026-10-10",
	}, &found); err != nil {
		t.Fatal(err)
	}
	if len(found.Itineraries) == 0 {
		t.Fatal("ride lost after restart")
	}
	if found.Itineraries[0].Legs[0].RideID != ride.RideID {
		t.Fatalf("ride id changed")
	}
}

// Confirm, persist, new Store+process, LOGIN as the same passenger, LIST_RESERVATIONS
// must return the reservation. Session userId must match persisted passengerId.
func TestListReservationsAfterServerRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	cfg := config.Load()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	cfg.ServerHost, cfg.ServerPort = host, port
	cfg.ReadTimeout = 3 * time.Second
	cfg.WriteTimeout = 3 * time.Second
	cfg.ConnectTimeout = 2 * time.Second
	silent := log.New(io.Discard, "", 0)
	srv := New(cfg, st, silent)
	go func() { _ = srv.Serve(ln) }()
	ride := publishSample(t, cfg)

	p, err := client.Dial(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Login("passageiro1", "senha123"); err != nil {
		t.Fatal(err)
	}
	passengerID := p.UserID
	var conf protocol.ConfirmReservationResult
	if err := p.MustOK(protocol.OpConfirmReservation, protocol.ConfirmReservationData{
		Legs: []protocol.ConfirmLeg{{RideID: ride.RideID, Origin: "Salvador", Destination: "Jequié"}},
	}, &conf); err != nil {
		t.Fatal(err)
	}
	_ = p.Close()
	_ = srv.Close()

	st2, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	ln2, _ := net.Listen("tcp", "127.0.0.1:0")
	host, port, _ = net.SplitHostPort(ln2.Addr().String())
	cfg.ServerHost, cfg.ServerPort = host, port
	srv2 := New(cfg, st2, silent)
	go func() { _ = srv2.Serve(ln2) }()
	t.Cleanup(func() { _ = srv2.Close() })

	p2, err := client.Dial(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer p2.Close()
	if err := p2.Login("passageiro1", "senha123"); err != nil {
		t.Fatal(err)
	}
	if p2.UserID != passengerID {
		t.Fatalf("userId changed after reload: %s vs %s", p2.UserID, passengerID)
	}
	var list protocol.ListReservationsResult
	if err := p2.MustOK(protocol.OpListReservations, map[string]any{}, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Reservations) != 1 {
		t.Fatalf("LIST_RESERVATIONS empty after restart: %+v", list)
	}
	if list.Reservations[0].ReservationID != conf.Reservation.ReservationID {
		t.Fatalf("reservation lost: %+v", list.Reservations[0])
	}
	if list.Reservations[0].PassengerID != passengerID {
		t.Fatalf("LIST filter/userId mismatch: %s vs %s", list.Reservations[0].PassengerID, passengerID)
	}
	d, err := client.Dial(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.Login("motorista1", "senha123"); err != nil {
		t.Fatal(err)
	}
	var rides protocol.ListDriverRidesResult
	if err := d.MustOK(protocol.OpListDriverRides, map[string]any{}, &rides); err != nil {
		t.Fatal(err)
	}
	if len(rides.Rides) != 1 || rides.Rides[0].Segments[0].AvailableSeats != 0 {
		t.Fatalf("seats not decremented after restart: %+v", rides.Rides)
	}
}

func TestRepeatedConfirmSameRequestIDOverTCP(t *testing.T) {
	cfg, _, _ := startTestServer(t)
	ride := publishSample(t, cfg)
	p, _ := client.Dial(cfg)
	defer p.Close()
	_ = p.Login("passageiro1", "senha123")
	payload := protocol.ConfirmReservationData{
		Legs: []protocol.ConfirmLeg{{RideID: ride.RideID, Origin: "Salvador", Destination: "Jequié"}},
	}
	a, err := p.Call(protocol.OpConfirmReservation, "idem-1", payload)
	if err != nil || a.Status != protocol.StatusOK {
		t.Fatalf("%v %+v", err, a)
	}
	b, err := p.Call(protocol.OpConfirmReservation, "idem-1", payload)
	if err != nil || b.Status != protocol.StatusOK {
		t.Fatalf("%v %+v", err, b)
	}
	var ra, rb protocol.ConfirmReservationResult
	_ = json.Unmarshal(a.Data, &ra)
	_ = json.Unmarshal(b.Data, &rb)
	if ra.Reservation.ReservationID != rb.Reservation.ReservationID {
		t.Fatal("idempotency broken")
	}
}

func TestConfirmDuplicateLegsRejectedOverTCP(t *testing.T) {
	cfg, st, _ := startTestServer(t)
	ride := publishSample(t, cfg)
	before := st.ListDriverRides("user-driver-1")[0].Segments[0].AvailableSeats
	p, _ := client.Dial(cfg)
	defer p.Close()
	_ = p.Login("passageiro1", "senha123")
	resp, err := p.Call(protocol.OpConfirmReservation, "dup-tcp", protocol.ConfirmReservationData{
		Legs: []protocol.ConfirmLeg{
			{RideID: ride.RideID, Origin: "Salvador", Destination: "Jequié"},
			{RideID: ride.RideID, Origin: "Salvador", Destination: "Jequié"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != protocol.StatusError || resp.Error == nil || resp.Error.Code != protocol.CodeValidationError {
		t.Fatalf("want VALIDATION_ERROR, got %+v", resp)
	}
	if st.ListDriverRides("user-driver-1")[0].Segments[0].AvailableSeats != before {
		t.Fatal("TCP duplicate confirm mutated seats")
	}
	if len(st.ListReservations("user-pass-1")) != 0 {
		t.Fatal("TCP duplicate confirm created a reservation")
	}
}

func TestInvalidConfirmJSONOverTCPDoesNotMutate(t *testing.T) {
	cfg, st, _ := startTestServer(t)
	_ = publishSample(t, cfg)
	before := readFile(t, st.Path())
	conn, err := net.Dial("tcp", cfg.ServerAddr())
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"version":1,"operation":"CONFIRM_RESERVATION","requestId":"bad-json","data":{`)
	if err := protocol.WriteFrame(conn, payload); err != nil {
		t.Fatal(err)
	}
	raw, err := protocol.ReadFrame(conn, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	var resp protocol.Response
	_ = json.Unmarshal(raw, &resp)
	if resp.Error == nil || resp.Error.Code != protocol.CodeInvalidJSON {
		t.Fatalf("%+v", resp)
	}
	after := readFile(t, st.Path())
	if !bytes.Equal(before, after) {
		t.Fatal("invalid TCP message mutated persisted state")
	}
	c, _ := client.Dial(cfg)
	defer c.Close()
	if err := c.Ping(); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestLoadishLatencyReported(t *testing.T) {
	cfg, _, _ := startTestServer(t)
	ride := publishSample(t, cfg)
	n := 12
	lat := make([]time.Duration, n)
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c, err := client.Dial(cfg)
			if err != nil {
				return
			}
			defer c.Close()
			_ = c.Login("passageiro3", "senha123")
			t0 := time.Now()
			_, _ = c.Call(protocol.OpConfirmReservation, "", protocol.ConfirmReservationData{
				Legs: []protocol.ConfirmLeg{{RideID: ride.RideID, Origin: "Salvador", Destination: "Feira de Santana"}},
			})
			lat[i] = time.Since(t0)
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)
	if elapsed <= 0 {
		t.Fatal("no elapsed")
	}
	var sum time.Duration
	for _, d := range lat {
		sum += d
	}
	t.Logf("n=%d elapsed=%s avg=%s", n, elapsed, sum/time.Duration(n))
}

func TestUnauthenticatedSearch(t *testing.T) {
	cfg, _, _ := startTestServer(t)
	c, _ := client.Dial(cfg)
	defer c.Close()
	resp, err := c.Call(protocol.OpSearchItineraries, "", protocol.SearchItinerariesData{Origin: "A", Destination: "B", Date: "2026-10-01"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil || resp.Error.Code != protocol.CodeUnauthenticated {
		t.Fatalf("%+v", resp)
	}
}
