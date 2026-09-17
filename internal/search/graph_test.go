package search

import (
	"testing"

	"github.com/yma1001/vaijunto/internal/domain"
)

// ride monta uma carona de teste com vagas e preços por trecho.
func ride(id string, cities []string, date string, seats []int, prices []int64) domain.Ride {
	segs := make([]domain.Segment, len(cities)-1)
	for i := 0; i < len(cities)-1; i++ {
		segs[i] = domain.Segment{
			RideID: id, SegmentIndex: i, Origin: cities[i], Destination: cities[i+1],
			Price: prices[i], AvailableSeats: seats[i],
		}
	}
	return domain.Ride{
		RideID: id, DriverID: "d-" + id, Cities: cities, DepartureDate: date,
		DepartureTime: "08:00", Capacity: 4, Status: domain.RideActive, Segments: segs,
	}
}

// TestDirectItinerary: Salvador→Jequié numa única carona vira 1 leg, 0 baldeações.
func TestDirectItinerary(t *testing.T) {
	r := ride("r1", []string{"Salvador", "Feira de Santana", "Jequié"}, "2026-10-01", []int{2, 2}, []int64{1000, 1500})
	got := Search([]domain.Ride{r}, "Salvador", "Jequié", "2026-10-01", 3, 20)
	if len(got) != 1 {
		t.Fatalf("got %d", len(got))
	}
	if got[0].Transfers != 0 || got[0].TotalPrice != 2500 || len(got[0].Legs) != 1 {
		t.Fatalf("%+v", got[0])
	}
}

// TestCompositeItinerary: dois motoristas (Salvador→Feira + Feira→Vitória) viram 1 itinerário composto.
func TestCompositeItinerary(t *testing.T) {
	a := ride("ra", []string{"Salvador", "Feira de Santana"}, "2026-10-01", []int{1}, []int64{1000})
	b := ride("rb", []string{"Feira de Santana", "Vitória da Conquista"}, "2026-10-01", []int{1}, []int64{2000})
	got := Search([]domain.Ride{a, b}, "Salvador", "Vitória da Conquista", "2026-10-01", 3, 20)
	if len(got) != 1 {
		t.Fatalf("got %d", len(got))
	}
	if got[0].Transfers != 1 || len(got[0].Legs) != 2 {
		t.Fatalf("%+v", got[0])
	}
}

// TestSearchDoesNotUseFullSegments: trecho com 0 vagas não entra no grafo.
func TestSearchDoesNotUseFullSegments(t *testing.T) {
	a := ride("ra", []string{"Salvador", "Feira de Santana"}, "2026-10-01", []int{0}, []int64{1000})
	got := Search([]domain.Ride{a}, "Salvador", "Feira de Santana", "2026-10-01", 3, 20)
	if len(got) != 0 {
		t.Fatalf("search must ignore unavailable edges")
	}
}

// TestSortByPriceThenTransfers: o composto mais barato vem antes do direto mais caro.
func TestSortByPriceThenTransfers(t *testing.T) {
	direct := ride("cheap", []string{"A", "C"}, "2026-10-01", []int{1}, []int64{5000})
	via := ride("a", []string{"A", "B"}, "2026-10-01", []int{1}, []int64{1000})
	via2 := ride("b", []string{"B", "C"}, "2026-10-01", []int{1}, []int64{1000})
	got := Search([]domain.Ride{direct, via, via2}, "A", "C", "2026-10-01", 3, 20)
	if len(got) < 2 {
		t.Fatalf("%d", len(got))
	}
	if got[0].TotalPrice != 2000 {
		t.Fatalf("cheaper composite should win: %+v", got[0])
	}
}

// TestIgnoresOtherDate: carona de outro dia não aparece na busca.
func TestIgnoresOtherDate(t *testing.T) {
	r := ride("r1", []string{"A", "B"}, "2026-10-02", []int{1}, []int64{1})
	if got := Search([]domain.Ride{r}, "A", "B", "2026-10-01", 3, 20); len(got) != 0 {
		t.Fatal("date must match")
	}
}
