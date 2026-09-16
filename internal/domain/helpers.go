package domain

import "strings"

func NormalizeCity(s string) string {
	return strings.TrimSpace(s)
}

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

func CopyStrings(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func CopyRide(r Ride) Ride {
	r.Cities = CopyStrings(r.Cities)
	r.Segments = append([]Segment(nil), r.Segments...)
	return r
}

func CopyReservation(r Reservation) Reservation {
	legs := make([]Leg, len(r.Legs))
	for i, l := range r.Legs {
		l.SegmentIndexes = append([]int(nil), l.SegmentIndexes...)
		legs[i] = l
	}
	r.Legs = legs
	return r
}

func CopyUser(u User) User { return u }
