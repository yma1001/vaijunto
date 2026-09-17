package domain

import "strings"

// NormalizeCity tira espaços das pontas. Comparações de cidade usam isso
// para "Salvador" e " Salvador " serem o mesmo vértice no grafo.
func NormalizeCity(s string) string {
	return strings.TrimSpace(s)
}

// CitiesEqual compara cidades sem diferenciar maiúsculas.
func CitiesEqual(a, b string) bool {
	return strings.EqualFold(NormalizeCity(a), NormalizeCity(b))
}

// CityIndex devolve o índice da cidade na rota, ou -1.
func CityIndex(cities []string, name string) int {
	for i, c := range cities {
		if CitiesEqual(c, name) {
			return i
		}
	}
	return -1
}

// SegmentIndexesBetween devolve os índices de trecho de origin até destination
// na ordem da rota. origin deve preceder destination.
func SegmentIndexesBetween(cities []string, origin, destination string) ([]int, bool) {
	oi := CityIndex(cities, origin)
	di := CityIndex(cities, destination)
	if oi < 0 || di < 0 || oi >= di {
		return nil, false
	}
	idx := make([]int, 0, di-oi)
	for i := oi; i < di; i++ {
		idx = append(idx, i)
	}
	return idx, true
}

// CopyStrings devolve uma cópia da fatia para o snapshot não vazar o slice interno.
func CopyStrings(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

// CopyRide copia carona e trechos para o caller não mutar o mapa do Store.
func CopyRide(r Ride) Ride {
	r.Cities = CopyStrings(r.Cities)
	r.Segments = append([]Segment(nil), r.Segments...)
	return r
}

// CopyReservation copia a reserva e os índices de segmento de cada leg.
func CopyReservation(r Reservation) Reservation {
	legs := make([]Leg, len(r.Legs))
	for i, l := range r.Legs {
		l.SegmentIndexes = append([]int(nil), l.SegmentIndexes...)
		legs[i] = l
	}
	r.Legs = legs
	return r
}

// CopyUser existe por simetria com Ride/Reservation (User não tem fatias internas).
func CopyUser(u User) User { return u }
