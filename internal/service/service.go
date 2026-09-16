package service

import (
	"fmt"

	"github.com/yma1001/vaijunto/internal/domain"
	"github.com/yma1001/vaijunto/internal/protocol"
	"github.com/yma1001/vaijunto/internal/search"
	"github.com/yma1001/vaijunto/internal/store"
)

type Service struct {
	Store        *store.Store
	MaxTransfers int
	MaxResults   int
}

func New(st *store.Store, maxTransfers, maxResults int) *Service {
	return &Service{Store: st, MaxTransfers: maxTransfers, MaxResults: maxResults}
}

func (s *Service) Search(origin, dest, date string) ([]domain.Itinerary, error) {
	if origin == "" || dest == "" || date == "" {
		return nil, fmt.Errorf("%w: origin, destination and date required", store.ErrValidation)
	}
	rides := s.Store.SnapshotRides()
	return search.Search(rides, origin, dest, date, s.MaxTransfers, s.MaxResults), nil
}

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
