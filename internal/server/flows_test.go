package server

import (
	"encoding/json"
	"testing"

	"github.com/yma1001/vaijunto/internal/client"
	"github.com/yma1001/vaijunto/internal/protocol"
)

func TestSearchDoesNotReserve(t *testing.T) {
	cfg, st, _ := startTestServer(t)
	ride := publishSample(t, cfg)
	p, _ := client.Dial(cfg)
	defer p.Close()
	_ = p.Login("passageiro1", "senha123")
	var found protocol.SearchItinerariesResult
	if err := p.MustOK(protocol.OpSearchItineraries, protocol.SearchItinerariesData{
		Origin: "Salvador", Destination: "Jequié", Date: "2026-10-10",
	}, &found); err != nil {
		t.Fatal(err)
	}
	if len(found.Itineraries) == 0 {
		t.Fatal("expected snapshot")
	}
	seats := st.ListDriverRides("user-driver-1")[0].Segments[0].AvailableSeats
	if seats != ride.Capacity {
		t.Fatalf("INV-5: search consumed a seat: %d", seats)
	}
}

func TestCompositeSearchAndConfirmTCP(t *testing.T) {
	cfg, _, _ := startTestServer(t)
	d, _ := client.Dial(cfg)
	defer d.Close()
	_ = d.Login("motorista1", "senha123")
	var a, b protocol.RideView
	_ = d.MustOK(protocol.OpPublishRide, protocol.PublishRideData{
		Cities: []string{"Salvador", "Feira de Santana"}, DepartureDate: "2026-11-01",
		DepartureTime: "08:00", Capacity: 1, SegmentPrices: []int64{1000},
	}, &a)
	d2, _ := client.Dial(cfg)
	defer d2.Close()
	_ = d2.Login("motorista2", "senha123")
	_ = d2.MustOK(protocol.OpPublishRide, protocol.PublishRideData{
		Cities: []string{"Feira de Santana", "Jequié"}, DepartureDate: "2026-11-01",
		DepartureTime: "09:00", Capacity: 1, SegmentPrices: []int64{2000},
	}, &b)

	p, _ := client.Dial(cfg)
	defer p.Close()
	_ = p.Login("passageiro1", "senha123")
	var found protocol.SearchItinerariesResult
	if err := p.MustOK(protocol.OpSearchItineraries, protocol.SearchItinerariesData{
		Origin: "Salvador", Destination: "Jequié", Date: "2026-11-01",
	}, &found); err != nil {
		t.Fatal(err)
	}
	if len(found.Itineraries) == 0 || found.Itineraries[0].Transfers != 1 {
		t.Fatalf("expected composite: %+v", found)
	}
	legs := []protocol.ConfirmLeg{}
	for _, l := range found.Itineraries[0].Legs {
		legs = append(legs, protocol.ConfirmLeg{RideID: l.RideID, Origin: l.Origin, Destination: l.Destination})
	}
	var conf protocol.ConfirmReservationResult
	if err := p.MustOK(protocol.OpConfirmReservation, protocol.ConfirmReservationData{Legs: legs}, &conf); err != nil {
		t.Fatal(err)
	}

	var pass protocol.ListRidePassengersResult
	if err := d.MustOK(protocol.OpListRidePassengers, protocol.ListRidePassengersData{RideID: a.RideID}, &pass); err != nil {
		t.Fatal(err)
	}
	if len(pass.Segments) == 0 || len(pass.Segments[0].Passengers) != 1 {
		t.Fatalf("driver should see passenger: %+v", pass)
	}
}

func TestRegisterLoginLogout(t *testing.T) {
	cfg, _, _ := startTestServer(t)
	c, _ := client.Dial(cfg)
	defer c.Close()
	var reg protocol.RegisterResult
	if err := c.MustOK(protocol.OpRegister, protocol.RegisterData{
		Username: "nova", Password: "abc", Role: protocol.RolePassenger,
	}, &reg); err != nil {
		t.Fatal(err)
	}
	if err := c.Login("nova", "abc"); err != nil {
		t.Fatal(err)
	}
	if err := c.MustOK(protocol.OpLogout, map[string]any{}, nil); err != nil {
		t.Fatal(err)
	}
	// conexão fecha no LOGOUT; um PING seguinte deve falhar
	if err := c.Ping(); err == nil {
		t.Fatal("expected closed connection after LOGOUT")
	}
}

func TestDriverCancelRideTCP(t *testing.T) {
	cfg, _, _ := startTestServer(t)
	ride := publishSample(t, cfg)
	p, _ := client.Dial(cfg)
	defer p.Close()
	_ = p.Login("passageiro1", "senha123")
	var conf protocol.ConfirmReservationResult
	if err := p.MustOK(protocol.OpConfirmReservation, protocol.ConfirmReservationData{
		Legs: []protocol.ConfirmLeg{{RideID: ride.RideID, Origin: "Salvador", Destination: "Jequié"}},
	}, &conf); err != nil {
		t.Fatal(err)
	}
	d, _ := client.Dial(cfg)
	defer d.Close()
	_ = d.Login("motorista1", "senha123")
	var cancelled protocol.RideView
	if err := d.MustOK(protocol.OpCancelRide, protocol.CancelRideData{RideID: ride.RideID}, &cancelled); err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != "CANCELLED" {
		t.Fatal(cancelled.Status)
	}
	var list protocol.ListReservationsResult
	if err := p.MustOK(protocol.OpListReservations, map[string]any{}, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Reservations) == 0 || list.Reservations[0].Status != "CANCELLED" {
		raw, _ := json.Marshal(list)
		t.Fatalf("passenger should see cancelled: %s", raw)
	}
}
