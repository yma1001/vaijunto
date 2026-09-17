package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/yma1001/vaijunto/internal/domain"
	"github.com/yma1001/vaijunto/internal/protocol"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	st, err := New(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func publishABC(t *testing.T, st *Store, capacity int) domain.Ride {
	t.Helper()
	r, err := st.PublishRide("user-driver-1", []string{"Salvador", "Feira de Santana", "Jequié"}, "2026-10-01", "08:00", capacity, []int64{1000, 1500})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestSeedUsersPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.Authenticate("motorista1", "senha123")
	if err != nil || u.Role != "DRIVER" {
		t.Fatalf("%v %#v", err, u)
	}
	st2, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st2.Authenticate("passageiro1", "senha123"); err != nil {
		t.Fatal("users must survive restart")
	}
}

func TestAvailabilityPerSegment(t *testing.T) {
	st := testStore(t)
	r := publishABC(t, st, 2)
	_, err := st.ConfirmReservation("user-pass-1", "req-a", []domain.LegInput{
		{RideID: r.RideID, Origin: "Salvador", Destination: "Feira de Santana"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := st.ListDriverRides("user-driver-1")[0]
	if got.Segments[0].AvailableSeats != 1 {
		t.Fatalf("A-B want 1 got %d", got.Segments[0].AvailableSeats)
	}
	if got.Segments[1].AvailableSeats != 2 {
		t.Fatalf("B-C should stay 2, got %d", got.Segments[1].AvailableSeats)
	}
}

func TestAtomicLastSegmentUnavailable(t *testing.T) {
	st := testStore(t)
	r1, _ := st.PublishRide("user-driver-1", []string{"Salvador", "Feira de Santana"}, "2026-10-01", "08:00", 1, []int64{1000})
	r2, _ := st.PublishRide("user-driver-2", []string{"Feira de Santana", "Jequié"}, "2026-10-01", "09:00", 1, []int64{1000})
	_, err := st.ConfirmReservation("user-pass-1", "fill-last", []domain.LegInput{
		{RideID: r2.RideID, Origin: "Feira de Santana", Destination: "Jequié"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.ConfirmReservation("user-pass-2", "composite", []domain.LegInput{
		{RideID: r1.RideID, Origin: "Salvador", Destination: "Feira de Santana"},
		{RideID: r2.RideID, Origin: "Feira de Santana", Destination: "Jequié"},
	})
	if err != ErrNoSeats {
		t.Fatalf("want ErrNoSeats got %v", err)
	}
	r1b := st.ListDriverRides("user-driver-1")[0]
	if r1b.Segments[0].AvailableSeats != 1 {
		t.Fatalf("partial reservation leaked: A-B=%d", r1b.Segments[0].AvailableSeats)
	}
	if v := st.CheckInvariants(); len(v) > 0 {
		t.Fatal(v)
	}
}

func TestLastSeatContention(t *testing.T) {
	st := testStore(t)
	r := publishABC(t, st, 1)
	var wg sync.WaitGroup
	var ok, fail int
	var mu sync.Mutex
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			pid := "user-pass-1"
			if i%2 == 1 {
				pid = "user-pass-2"
			}
			_, err := st.ConfirmReservation(pid, "", []domain.LegInput{
				{RideID: r.RideID, Origin: "Salvador", Destination: "Jequié"},
			})
			mu.Lock()
			if err == nil {
				ok++
			} else {
				fail++
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if ok != 1 {
		t.Fatalf("expected exactly 1 success, got %d ok %d fail", ok, fail)
	}
	got := st.ListDriverRides("user-driver-1")[0]
	if got.Segments[0].AvailableSeats != 0 || got.Segments[1].AvailableSeats != 0 {
		t.Fatalf("seats %#v", got.Segments)
	}
	if v := st.CheckInvariants(); len(v) > 0 {
		t.Fatal(v)
	}
}

func TestCancelIdempotent(t *testing.T) {
	st := testStore(t)
	r := publishABC(t, st, 1)
	res, err := st.ConfirmReservation("user-pass-1", "once", []domain.LegInput{
		{RideID: r.RideID, Origin: "Salvador", Destination: "Jequié"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CancelReservation("user-pass-1", res.ReservationID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CancelReservation("user-pass-1", res.ReservationID); err != nil {
		t.Fatal(err)
	}
	got := st.ListDriverRides("user-driver-1")[0]
	if got.Segments[0].AvailableSeats != 1 || got.Segments[1].AvailableSeats != 1 {
		t.Fatalf("double restore? %#v", got.Segments)
	}
}

func TestConfirmIdempotentRequestID(t *testing.T) {
	st := testStore(t)
	r := publishABC(t, st, 2)
	in := []domain.LegInput{{RideID: r.RideID, Origin: "Salvador", Destination: "Feira de Santana"}}
	a, err := st.ConfirmReservation("user-pass-1", "same-req", in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.ConfirmReservation("user-pass-1", "same-req", in)
	if err != nil {
		t.Fatal(err)
	}
	if a.ReservationID != b.ReservationID {
		t.Fatalf("duplicate reservation %s vs %s", a.ReservationID, b.ReservationID)
	}
	got := st.ListDriverRides("user-driver-1")[0]
	if got.Segments[0].AvailableSeats != 1 {
		t.Fatalf("second confirm consumed a seat: %d", got.Segments[0].AvailableSeats)
	}
}

func TestCancelRideRestoresComposite(t *testing.T) {
	st := testStore(t)
	a, _ := st.PublishRide("user-driver-1", []string{"Salvador", "Feira de Santana"}, "2026-10-01", "08:00", 1, []int64{1000})
	b, _ := st.PublishRide("user-driver-2", []string{"Feira de Santana", "Jequié"}, "2026-10-01", "09:00", 1, []int64{2000})
	_, err := st.ConfirmReservation("user-pass-1", "comp", []domain.LegInput{
		{RideID: a.RideID, Origin: "Salvador", Destination: "Feira de Santana"},
		{RideID: b.RideID, Origin: "Feira de Santana", Destination: "Jequié"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CancelRide("user-driver-1", a.RideID); err != nil {
		t.Fatal(err)
	}
	bb := st.ListDriverRides("user-driver-2")[0]
	if bb.Segments[0].AvailableSeats != 1 {
		t.Fatalf("other ride should be restored, got %d", bb.Segments[0].AvailableSeats)
	}
	res := st.ListReservations("user-pass-1")
	if res[0].Status != domain.ResCancelled {
		t.Fatalf("reservation status %s", res[0].Status)
	}
}

func TestPersistenceRestartKeepsRides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, _ := New(path)
	r := publishABC(t, st, 2)
	_, _ = st.ConfirmReservation("user-pass-1", "p", []domain.LegInput{
		{RideID: r.RideID, Origin: "Salvador", Destination: "Feira de Santana"},
	})
	st2, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	rides := st2.ListDriverRides("user-driver-1")
	if len(rides) != 1 || rides[0].Segments[0].AvailableSeats != 1 {
		t.Fatalf("persisted state lost: %#v", rides)
	}
	list := st2.ListReservations("user-pass-1")
	if len(list) != 1 {
		t.Fatal("reservation not persisted")
	}
}

func confirmSalvadorFeira(t *testing.T, st *Store, passengerID, requestID string, ride domain.Ride) domain.Reservation {
	t.Helper()
	res, err := st.ConfirmReservation(passengerID, requestID, []domain.LegInput{
		{RideID: ride.RideID, Origin: "Salvador", Destination: "Feira de Santana"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func assertPassengerReservationAndSeats(t *testing.T, st *Store, passengerID, reservationID, rideID string, wantSeats int) {
	t.Helper()
	if _, err := st.Authenticate("passageiro1", "senha123"); err != nil {
		t.Fatal("users must survive restart:", err)
	}
	if _, err := st.Authenticate("motorista1", "senha123"); err != nil {
		t.Fatal("driver must survive restart:", err)
	}
	rides := st.ListDriverRides("user-driver-1")
	if len(rides) != 1 || rides[0].RideID != rideID {
		t.Fatalf("rides lost after reload: %#v", rides)
	}
	if rides[0].Segments[0].AvailableSeats != wantSeats {
		t.Fatalf("seats after reload: want %d got %d", wantSeats, rides[0].Segments[0].AvailableSeats)
	}
	list := st.ListReservations(passengerID)
	if len(list) != 1 {
		t.Fatalf("LIST/GetReservations empty after reload: %#v", list)
	}
	if list[0].ReservationID != reservationID {
		t.Fatalf("reservation id %s vs %s", list[0].ReservationID, reservationID)
	}
	if list[0].PassengerID != passengerID {
		t.Fatalf("passengerId mismatch after reload: %s vs session/persisted %s", list[0].PassengerID, passengerID)
	}
	if list[0].Status != domain.ResConfirmed {
		t.Fatalf("status %s", list[0].Status)
	}
}

// Regression: confirm → persist → new Store on the same temp file must LIST the
// reservation for that passenger and keep seats decremented. IP is not in the file.
func TestConfirmReservationSurvivesNewStoreLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	ride := publishABC(t, st, 2)
	res := confirmSalvadorFeira(t, st, "user-pass-1", "req-reload", ride)

	raw := readPersisted(t, st)
	if !bytes.Contains(raw, []byte(`"reservations"`)) {
		t.Fatal("confirm must persist reservations in JSON")
	}
	var file persistedState
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Reservations) != 1 {
		t.Fatalf("reservations omitted from JSON: %+v", file)
	}
	if file.Reservations[0].ReservationID != res.ReservationID || file.Reservations[0].PassengerID != "user-pass-1" {
		t.Fatalf("persisted reservation: %+v", file.Reservations[0])
	}

	st2, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if st2.Path() != st.Path() {
		t.Fatalf("DATA_PATH changed: %s vs %s", st.Path(), st2.Path())
	}
	assertPassengerReservationAndSeats(t, st2, "user-pass-1", res.ReservationID, ride.RideID, 1)
}

func wipeConfirmIndexInFile(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	obj["confirmIndex"] = json.RawMessage(`{}`)
	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
}

// confirmIndex may be missing/empty after a crash or an old file. Load must still
// return the reservation AND rebuild the idempotency index from reservations.
func TestLoadRebuildsConfirmIndexFromReservations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	ride := publishABC(t, st, 2)
	in := []domain.LegInput{{RideID: ride.RideID, Origin: "Salvador", Destination: "Feira de Santana"}}
	res, err := st.ConfirmReservation("user-pass-1", "req-idem-reload", in)
	if err != nil {
		t.Fatal(err)
	}
	wipeConfirmIndexInFile(t, path)

	st2, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	assertPassengerReservationAndSeats(t, st2, "user-pass-1", res.ReservationID, ride.RideID, 1)

	again, err := st2.ConfirmReservation("user-pass-1", "req-idem-reload", in)
	if err != nil {
		t.Fatal(err)
	}
	if again.ReservationID != res.ReservationID {
		t.Fatalf("confirmIndex not rebuilt from reservations: %s vs %s", again.ReservationID, res.ReservationID)
	}
	if rideSeats(t, st2, "user-driver-1", ride.RideID)[0] != 1 {
		t.Fatal("retransmit after reload consumed another seat")
	}
}

func TestResolveDataPathIsAbsolute(t *testing.T) {
	got, err := resolveDataPath("data/state.json")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("DATA_PATH must be stored absolute, got %s", got)
	}
	if !strings.HasSuffix(got, "data/state.json") && !strings.HasSuffix(got, `data\state.json`) {
		t.Fatalf("suffix lost: %s", got)
	}
}

func TestRegisterBothRolesDuplicateAndRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	st, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	driver, err := st.Register("novo-motorista", "s1", protocol.RoleDriver)
	if err != nil || driver.UserID == "" || driver.Role != protocol.RoleDriver {
		t.Fatalf("driver: %#v %v", driver, err)
	}
	pass, err := st.Register("novo-passageiro", "s2", protocol.RolePassenger)
	if err != nil || pass.UserID == "" || pass.Role != protocol.RolePassenger {
		t.Fatalf("passenger: %#v %v", pass, err)
	}
	if driver.UserID == pass.UserID {
		t.Fatal("server must generate distinct userIds")
	}
	_, err = st.Register("novo-motorista", "outra", protocol.RoleDriver)
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("duplicate want ErrAlreadyExists got %v", err)
	}
	_, err = st.Register("x", "y", "ADMIN")
	if err == nil || !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid role: %v", err)
	}
	st2, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if u, err := st2.Authenticate("novo-motorista", "s1"); err != nil || u.UserID != driver.UserID {
		t.Fatalf("driver lost after restart: %#v %v", u, err)
	}
	if u, err := st2.Authenticate("novo-passageiro", "s2"); err != nil || u.UserID != pass.UserID {
		t.Fatalf("passenger lost after restart: %#v %v", u, err)
	}
}

func TestPublishRideTwoAndFourCities(t *testing.T) {
	st := testStore(t)
	two, err := st.PublishRide("user-driver-1", []string{"Salvador", "Feira de Santana"}, "2026-10-10", "08:00", 2, []int64{1500})
	if err != nil || len(two.Cities) != 2 || len(two.Segments) != 1 {
		t.Fatalf("two cities: %#v %v", two, err)
	}
	four, err := st.PublishRide("user-driver-1", []string{"Salvador", "Feira de Santana", "Jequié", "Vitória da Conquista"}, "2026-10-10", "09:00", 2, []int64{1500, 2000, 2500})
	if err != nil || len(four.Cities) != 4 || len(four.Segments) != 3 {
		t.Fatalf("four cities: %#v %v", four, err)
	}
}

func TestNeverNegativeOrAboveCapacity(t *testing.T) {
	st := testStore(t)
	r := publishABC(t, st, 2)
	for i := 0; i < 5; i++ {
		_, _ = st.ConfirmReservation("user-pass-1", "", []domain.LegInput{
			{RideID: r.RideID, Origin: "Salvador", Destination: "Jequié"},
		})
	}
	if v := st.CheckInvariants(); len(v) > 0 {
		t.Fatal(v)
	}
}

func rideSeats(t *testing.T, st *Store, driverID, rideID string) []int {
	t.Helper()
	for _, r := range st.ListDriverRides(driverID) {
		if r.RideID != rideID {
			continue
		}
		out := make([]int, len(r.Segments))
		for i, seg := range r.Segments {
			out[i] = seg.AvailableSeats
		}
		return out
	}
	t.Fatalf("ride %s not found", rideID)
	return nil
}

func readPersisted(t *testing.T, st *Store) []byte {
	t.Helper()
	raw, err := os.ReadFile(st.Path())
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mustRejectUnchanged(t *testing.T, st *Store, driverID, rideID, passengerID string, beforeSeats []int, beforeFile []byte, err error) {
	t.Helper()
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
	got := rideSeats(t, st, driverID, rideID)
	if !slices.Equal(got, beforeSeats) {
		t.Fatalf("seats mutated: got %v want %v", got, beforeSeats)
	}
	if n := len(st.ListReservations(passengerID)); n != 0 {
		t.Fatalf("must not create reservation, got %d", n)
	}
	if !bytes.Equal(readPersisted(t, st), beforeFile) {
		t.Fatal("persisted state changed after rejected confirm")
	}
	if v := st.CheckInvariants(); len(v) > 0 {
		t.Fatal(v)
	}
}

// Regression: duplicate identical legs used to confirm and decrement the same
// segments twice (capacity 1 became -1). Must fail as VALIDATION_ERROR with
// state intact — do not clamp negatives to hide the bug.
func TestConfirmRejectsDuplicateIdenticalLegCapacity1(t *testing.T) {
	st := testStore(t)
	r := publishABC(t, st, 1)
	beforeSeats := rideSeats(t, st, "user-driver-1", r.RideID)
	beforeFile := readPersisted(t, st)
	_, err := st.ConfirmReservation("user-pass-1", "dup-cap1", []domain.LegInput{
		{RideID: r.RideID, Origin: "Salvador", Destination: "Jequié"},
		{RideID: r.RideID, Origin: "Salvador", Destination: "Jequié"},
	})
	mustRejectUnchanged(t, st, "user-driver-1", r.RideID, "user-pass-1", beforeSeats, beforeFile, err)
	if beforeSeats[0] != 1 || rideSeats(t, st, "user-driver-1", r.RideID)[0] != 1 {
		t.Fatalf("capacity-1 duplicate must not go negative, seats=%v", rideSeats(t, st, "user-driver-1", r.RideID))
	}
}

func TestConfirmRejectsDuplicateIdenticalLegCapacityGreaterThan1(t *testing.T) {
	st := testStore(t)
	r := publishABC(t, st, 3)
	beforeSeats := rideSeats(t, st, "user-driver-1", r.RideID)
	beforeFile := readPersisted(t, st)
	_, err := st.ConfirmReservation("user-pass-1", "dup-cap3", []domain.LegInput{
		{RideID: r.RideID, Origin: "Salvador", Destination: "Jequié"},
		{RideID: r.RideID, Origin: "Salvador", Destination: "Jequié"},
	})
	mustRejectUnchanged(t, st, "user-driver-1", r.RideID, "user-pass-1", beforeSeats, beforeFile, err)
}

func TestConfirmRejectsPartialSegmentOverlap(t *testing.T) {
	st := testStore(t)
	r := publishABC(t, st, 2)
	beforeSeats := rideSeats(t, st, "user-driver-1", r.RideID)
	beforeFile := readPersisted(t, st)
	_, err := st.ConfirmReservation("user-pass-1", "overlap-bc", []domain.LegInput{
		{RideID: r.RideID, Origin: "Salvador", Destination: "Jequié"},
		{RideID: r.RideID, Origin: "Feira de Santana", Destination: "Jequié"},
	})
	mustRejectUnchanged(t, st, "user-driver-1", r.RideID, "user-pass-1", beforeSeats, beforeFile, err)
}

func TestConfirmRejectLeavesSeatsReservationsAndFileUnchanged(t *testing.T) {
	st := testStore(t)
	r := publishABC(t, st, 1)
	beforeSeats := rideSeats(t, st, "user-driver-1", r.RideID)
	beforeFile := readPersisted(t, st)
	_, err := st.ConfirmReservation("user-pass-1", "no-mutate", []domain.LegInput{
		{RideID: r.RideID, Origin: "Salvador", Destination: "Feira de Santana"},
		{RideID: r.RideID, Origin: "Salvador", Destination: "Feira de Santana"},
	})
	mustRejectUnchanged(t, st, "user-driver-1", r.RideID, "user-pass-1", beforeSeats, beforeFile, err)
}

func TestConfirmRejectsDisconnectedItinerary(t *testing.T) {
	st := testStore(t)
	r, err := st.PublishRide("user-driver-1", []string{"Salvador", "Feira de Santana", "Jequié", "Vitória da Conquista"}, "2026-10-01", "08:00", 2, []int64{1000, 1500, 2000})
	if err != nil {
		t.Fatal(err)
	}
	beforeSeats := rideSeats(t, st, "user-driver-1", r.RideID)
	beforeFile := readPersisted(t, st)
	_, err = st.ConfirmReservation("user-pass-1", "disconnected", []domain.LegInput{
		{RideID: r.RideID, Origin: "Salvador", Destination: "Feira de Santana"},
		{RideID: r.RideID, Origin: "Jequié", Destination: "Vitória da Conquista"},
	})
	mustRejectUnchanged(t, st, "user-driver-1", r.RideID, "user-pass-1", beforeSeats, beforeFile, err)
}

func TestConfirmRejectsIncompatibleDates(t *testing.T) {
	st := testStore(t)
	a, err := st.PublishRide("user-driver-1", []string{"Salvador", "Feira de Santana"}, "2026-10-01", "08:00", 1, []int64{1000})
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.PublishRide("user-driver-2", []string{"Feira de Santana", "Jequié"}, "2026-10-02", "09:00", 1, []int64{2000})
	if err != nil {
		t.Fatal(err)
	}
	beforeA := rideSeats(t, st, "user-driver-1", a.RideID)
	beforeB := rideSeats(t, st, "user-driver-2", b.RideID)
	beforeFile := readPersisted(t, st)
	_, err = st.ConfirmReservation("user-pass-1", "dates", []domain.LegInput{
		{RideID: a.RideID, Origin: "Salvador", Destination: "Feira de Santana"},
		{RideID: b.RideID, Origin: "Feira de Santana", Destination: "Jequié"},
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
	if !slices.Equal(rideSeats(t, st, "user-driver-1", a.RideID), beforeA) {
		t.Fatal("ride A seats mutated")
	}
	if !slices.Equal(rideSeats(t, st, "user-driver-2", b.RideID), beforeB) {
		t.Fatal("ride B seats mutated")
	}
	if len(st.ListReservations("user-pass-1")) != 0 {
		t.Fatal("must not create reservation")
	}
	if !bytes.Equal(readPersisted(t, st), beforeFile) {
		t.Fatal("persisted state changed after rejected confirm")
	}
}

func TestConfirmValidCompositeDistinctDrivers(t *testing.T) {
	st := testStore(t)
	a, err := st.PublishRide("user-driver-1", []string{"Salvador", "Feira de Santana"}, "2026-10-01", "08:00", 1, []int64{1000})
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.PublishRide("user-driver-2", []string{"Feira de Santana", "Jequié"}, "2026-10-01", "09:00", 1, []int64{2000})
	if err != nil {
		t.Fatal(err)
	}
	res, err := st.ConfirmReservation("user-pass-1", "composite-ok", []domain.LegInput{
		{RideID: a.RideID, Origin: "Salvador", Destination: "Feira de Santana"},
		{RideID: b.RideID, Origin: "Feira de Santana", Destination: "Jequié"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.ResConfirmed || len(res.Legs) != 2 || res.TotalPrice != 3000 {
		t.Fatalf("unexpected reservation: %#v", res)
	}
	if rideSeats(t, st, "user-driver-1", a.RideID)[0] != 0 {
		t.Fatal("driver 1 seat not consumed")
	}
	if rideSeats(t, st, "user-driver-2", b.RideID)[0] != 0 {
		t.Fatal("driver 2 seat not consumed")
	}
}

func TestConfirmValidConsecutiveSegmentsSameRide(t *testing.T) {
	st := testStore(t)
	r := publishABC(t, st, 2)
	res, err := st.ConfirmReservation("user-pass-1", "same-ride-legs", []domain.LegInput{
		{RideID: r.RideID, Origin: "Salvador", Destination: "Feira de Santana"},
		{RideID: r.RideID, Origin: "Feira de Santana", Destination: "Jequié"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.ResConfirmed || len(res.Legs) != 2 {
		t.Fatalf("%#v", res)
	}
	got := rideSeats(t, st, "user-driver-1", r.RideID)
	if got[0] != 1 || got[1] != 1 {
		t.Fatalf("want 1,1 got %v", got)
	}
}
