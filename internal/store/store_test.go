package store

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/yma1001/vaijunto/internal/domain"
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
