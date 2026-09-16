package search

import (
	"sort"
	"strings"

	"github.com/yma1001/vaijunto/internal/domain"
)

// O grafo NÃO é uma estrutura global sincronizada. É derivado das caronas
// ativas no momento da busca (ainda sob RLock no caller). Cidade = vértice;
// trecho com availableSeats > 0 na data pedida = aresta dirigida.
//
// DFS em caminhos simples (não revisita cidade) evita ciclos. Limites
// MAX_TRANSFERS / MAX_RESULTS são técnicos do protótipo.

type edge struct {
	ride         domain.Ride
	segmentIndex int
}

type step struct {
	rideID string
	si     int
	from   string
	to     string
}

func Search(rides []domain.Ride, origin, dest, date string, maxTransfers, maxResults int) []domain.Itinerary {
	origin = domain.NormalizeCity(origin)
	dest = domain.NormalizeCity(dest)
	if origin == "" || dest == "" || domain.CitiesEqual(origin, dest) {
		return nil
	}

	adj := map[string][]edge{}
	for _, r := range rides {
		if r.Status != domain.RideActive {
			continue
		}
		if r.DepartureDate != date {
			continue
		}
		for i, seg := range r.Segments {
			if seg.AvailableSeats < 1 {
				continue
			}
			key := strings.ToLower(domain.NormalizeCity(seg.Origin))
			adj[key] = append(adj[key], edge{ride: r, segmentIndex: i})
		}
	}

	var found []domain.Itinerary
	var walk func(city string, path []step, visited map[string]bool)
	walk = func(city string, path []step, visited map[string]bool) {
		if len(found) >= maxResults*4 {
			// corta explosão; a ordenação + truncamento acontecem no final
			return
		}
		if domain.CitiesEqual(city, dest) && len(path) > 0 {
			found = append(found, buildItinerary(rides, path, maxTransfers))
			return
		}
		key := strings.ToLower(domain.NormalizeCity(city))
		for _, e := range adj[key] {
			seg := e.ride.Segments[e.segmentIndex]
			next := seg.Destination
			vk := strings.ToLower(domain.NormalizeCity(next))
			if visited[vk] {
				continue
			}
			nvis := copyVisited(visited)
			nvis[vk] = true
			npath := append(append([]step(nil), path...), step{
				rideID: e.ride.RideID,
				si:     e.segmentIndex,
				from:   seg.Origin,
				to:     seg.Destination,
			})
			if transfersOf(npath) > maxTransfers {
				continue
			}
			walk(next, npath, nvis)
		}
	}

	startVisited := map[string]bool{strings.ToLower(origin): true}
	walk(origin, nil, startVisited)

	out := make([]domain.Itinerary, 0, len(found))
	for _, it := range found {
		if it.Transfers > maxTransfers || len(it.Legs) == 0 {
			continue
		}
		out = append(out, it)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TotalPrice != out[j].TotalPrice {
			return out[i].TotalPrice < out[j].TotalPrice
		}
		if out[i].Transfers != out[j].Transfers {
			return out[i].Transfers < out[j].Transfers
		}
		return itineraryKey(out[i]) < itineraryKey(out[j])
	})
	if len(out) > maxResults {
		out = out[:maxResults]
	}
	return out
}

func buildItinerary(rides []domain.Ride, path []step, maxTransfers int) domain.Itinerary {
	if len(path) == 0 {
		return domain.Itinerary{}
	}
	rideByID := map[string]domain.Ride{}
	for _, r := range rides {
		rideByID[r.RideID] = r
	}

	// Compacta segmentos consecutivos da mesma carona em um único Leg.
	var legs []domain.Leg
	cur := domain.Leg{
		RideID:         path[0].rideID,
		Origin:         path[0].from,
		SegmentIndexes: []int{path[0].si},
	}
	ride := rideByID[path[0].rideID]
	cur.DriverID = ride.DriverID
	cur.DepartureDate = ride.DepartureDate
	cur.DepartureTime = ride.DepartureTime
	cur.Price = ride.Segments[path[0].si].Price
	cur.AvailableSeats = ride.Segments[path[0].si].AvailableSeats
	cur.Destination = path[0].to

	for i := 1; i < len(path); i++ {
		st := path[i]
		if st.rideID == cur.RideID {
			cur.SegmentIndexes = append(cur.SegmentIndexes, st.si)
			r := rideByID[st.rideID]
			cur.Price += r.Segments[st.si].Price
			if r.Segments[st.si].AvailableSeats < cur.AvailableSeats {
				cur.AvailableSeats = r.Segments[st.si].AvailableSeats
			}
			cur.Destination = st.to
			continue
		}
		legs = append(legs, cur)
		r := rideByID[st.rideID]
		cur = domain.Leg{
			RideID:         st.rideID,
			DriverID:       r.DriverID,
			Origin:         st.from,
			Destination:    st.to,
			DepartureDate:  r.DepartureDate,
			DepartureTime:  r.DepartureTime,
			Price:          r.Segments[st.si].Price,
			AvailableSeats: r.Segments[st.si].AvailableSeats,
			SegmentIndexes: []int{st.si},
		}
	}
	legs = append(legs, cur)

	var total int64
	for _, l := range legs {
		total += l.Price
	}
	transfers := len(legs) - 1
	if transfers < 0 {
		transfers = 0
	}
	if transfers > maxTransfers {
		return domain.Itinerary{}
	}
	return domain.Itinerary{TotalPrice: total, Transfers: transfers, Legs: legs}
}

func transfersOf(path []step) int {
	if len(path) == 0 {
		return 0
	}
	n := 0
	cur := path[0].rideID
	for i := 1; i < len(path); i++ {
		if path[i].rideID != cur {
			n++
			cur = path[i].rideID
		}
	}
	return n
}

func copyVisited(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}

func itineraryKey(it domain.Itinerary) string {
	var b strings.Builder
	for _, l := range it.Legs {
		b.WriteString(l.RideID)
		b.WriteByte('|')
		b.WriteString(l.Origin)
		b.WriteByte('>')
		b.WriteString(l.Destination)
		b.WriteByte(';')
	}
	return b.String()
}
