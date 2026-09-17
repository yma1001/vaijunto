// Pacote service liga o Store à busca e converte o domínio para as views do protocolo.
// Não pega lock: quem sincroniza é o Store (snapshot) e o grafo trabalha na cópia.
package service

import (
	"fmt"

	"github.com/yma1001/vaijunto/internal/domain"
	"github.com/yma1001/vaijunto/internal/protocol"
	"github.com/yma1001/vaijunto/internal/search"
	"github.com/yma1001/vaijunto/internal/store"
)

// Service encapsula limites de baldeação/resultados e o acesso ao Store na busca.
type Service struct {
	Store        *store.Store
	MaxTransfers int
	MaxResults   int
}

// New constrói o serviço usado pelo dispatcher do servidor.
func New(st *store.Store, maxTransfers, maxResults int) *Service {
	return &Service{Store: st, MaxTransfers: maxTransfers, MaxResults: maxResults}
}

// Search tira um snapshot das caronas e devolve itinerários. Não reserva assento.
func (s *Service) Search(origin, dest, date string) ([]domain.Itinerary, error) {
	if origin == "" || dest == "" || date == "" {
		return nil, fmt.Errorf("%w: origin, destination and date required", store.ErrValidation)
	}
	rides := s.Store.SnapshotRides()
	return search.Search(rides, origin, dest, date, s.MaxTransfers, s.MaxResults), nil
}

// RideView projeta a carona interna no JSON que o cliente recebe.
func RideView(r domain.Ride) protocol.RideView {
	segs := make([]protocol.SegmentView, len(r.Segments))
	for i, sg := range r.Segments {
		segs[i] = protocol.SegmentView{
			SegmentIndex:   sg.SegmentIndex,
			Origin:         sg.Origin,
			Destination:    sg.Destination,
			Price:          sg.Price,
			AvailableSeats: sg.AvailableSeats,
		}
	}
	return protocol.RideView{
		RideID:        r.RideID,
		DriverID:      r.DriverID,
		Cities:        r.Cities,
		DepartureDate: r.DepartureDate,
		DepartureTime: r.DepartureTime,
		Capacity:      r.Capacity,
		Status:        r.Status,
		Segments:      segs,
	}
}

// ItineraryView projeta o resultado da busca para o envelope SEARCH_ITINERARIES.
func ItineraryView(it domain.Itinerary) protocol.ItineraryView {
	legs := make([]protocol.LegView, len(it.Legs))
	for i, l := range it.Legs {
		legs[i] = LegView(l)
	}
	return protocol.ItineraryView{
		TotalPrice: it.TotalPrice,
		Transfers:  it.Transfers,
		Legs:       legs,
	}
}

// LegView omite os índices internos de segmento; o cliente só precisa de origin/destino/rideId.
func LegView(l domain.Leg) protocol.LegView {
	return protocol.LegView{
		RideID:         l.RideID,
		DriverID:       l.DriverID,
		Origin:         l.Origin,
		Destination:    l.Destination,
		DepartureDate:  l.DepartureDate,
		DepartureTime:  l.DepartureTime,
		Price:          l.Price,
		AvailableSeats: l.AvailableSeats,
	}
}

// ReservationView projeta a reserva persistida para LIST/CONFIRM/CANCEL.
func ReservationView(r domain.Reservation) protocol.ReservationView {
	legs := make([]protocol.LegView, len(r.Legs))
	for i, l := range r.Legs {
		legs[i] = LegView(l)
	}
	return protocol.ReservationView{
		ReservationID: r.ReservationID,
		PassengerID:   r.PassengerID,
		Status:        r.Status,
		TotalPrice:    r.TotalPrice,
		CreatedAt:     r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Legs:          legs,
	}
}
