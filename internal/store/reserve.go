package store

import (
	"fmt"

	"github.com/yma1001/vaijunto/internal/domain"
)

func confirmKey(passengerID, requestID string) string {
	return passengerID + "|" + requestID
}

// ConfirmReservation é a seção crítica da atomicidade (INV-3, INV-10).
//
// Passos, todos ainda sob s.mu.Lock:
//  1. idempotência por (passenger, requestId) — retransmissão devolve a mesma reserva
//  2. expandir cada leg em índices de segmento
//  3. REVALIDAR disponibilidade de TODOS os trechos (a busca era só snapshot)
//  4. se algum trecho falhar, nenhum é decrementado (tudo-ou-nada)
//  5. decrementar todos, gravar reserva, persistir
func (s *Store) ConfirmReservation(passengerID, requestID string, inputs []domain.LegInput) (domain.Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if requestID != "" {
		if id, ok := s.confirmIndex[confirmKey(passengerID, requestID)]; ok {
			if res, exists := s.reservations[id]; exists {
				return domain.CopyReservation(res), nil
			}
		}
	}
	if len(inputs) == 0 {
		return domain.Reservation{}, fmt.Errorf("%w: legs required", ErrValidation)
	}

	type dec struct {
		rideID string
		index  int
	}
	var decrements []dec
	var legs []domain.Leg
	var total int64

	for _, in := range inputs {
		ride, ok := s.rides[in.RideID]
		if !ok {
			return domain.Reservation{}, fmt.Errorf("%w: ride %s", ErrNotFound, in.RideID)
		}
		if ride.Status != domain.RideActive {
			return domain.Reservation{}, fmt.Errorf("%w: ride %s not active", ErrConflict, in.RideID)
		}
		idxs, ok := domain.SegmentIndexesBetween(ride.Cities, in.Origin, in.Destination)
		if !ok {
			return domain.Reservation{}, fmt.Errorf("%w: %s -> %s not on ride %s", ErrValidation, in.Origin, in.Destination, in.RideID)
		}
		var price int64
		minAvail := ride.Capacity
		for _, si := range idxs {
			seg := ride.Segments[si]
			if seg.AvailableSeats < 1 {
				// Falha no último trecho (ou em qualquer um): aborta SEM mutar.
				return domain.Reservation{}, ErrNoSeats
			}
			if seg.AvailableSeats < minAvail {
				minAvail = seg.AvailableSeats
			}
			decrements = append(decrements, dec{rideID: ride.RideID, index: si})
			price += seg.Price
		}
		legs = append(legs, domain.Leg{
			RideID:         ride.RideID,
			DriverID:       ride.DriverID,
			Origin:         ride.Cities[idxs[0]],
			Destination:    ride.Cities[idxs[len(idxs)-1]+1],
			DepartureDate:  ride.DepartureDate,
			DepartureTime:  ride.DepartureTime,
			Price:          price,
			AvailableSeats: minAvail,
			SegmentIndexes: idxs,
		})
		total += price
	}

	for _, d := range decrements {
		ride := s.rides[d.rideID]
		ride.Segments[d.index].AvailableSeats--
		s.rides[d.rideID] = ride
	}

	res := domain.Reservation{
		ReservationID: newID("res-"),
		PassengerID:   passengerID,
		RequestID:     requestID,
		Status:        domain.ResConfirmed,
		TotalPrice:    total,
		Legs:          legs,
		CreatedAt:     s.now(),
	}
	s.reservations[res.ReservationID] = res
	if requestID != "" {
		s.confirmIndex[confirmKey(passengerID, requestID)] = res.ReservationID
	}

	if err := s.persistLocked(); err != nil {
		for _, d := range decrements {
			ride := s.rides[d.rideID]
			ride.Segments[d.index].AvailableSeats++
			s.rides[d.rideID] = ride
		}
		delete(s.reservations, res.ReservationID)
		delete(s.confirmIndex, confirmKey(passengerID, requestID))
		return domain.Reservation{}, err
	}
	return domain.CopyReservation(res), nil
}

// CancelReservation é idempotente (INV-4): repetir o cancelamento NÃO
// devolve assentos de novo.
func (s *Store) CancelReservation(passengerID, reservationID string) (domain.Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, ok := s.reservations[reservationID]
	if !ok {
		return domain.Reservation{}, fmt.Errorf("%w: reservation", ErrNotFound)
	}
	if res.PassengerID != passengerID {
		return domain.Reservation{}, ErrForbidden
	}
	if res.Status == domain.ResCancelled {
		return domain.CopyReservation(res), nil
	}

	s.restoreReservationLocked(res)
	res.Status = domain.ResCancelled
	s.reservations[reservationID] = res
	if err := s.persistLocked(); err != nil {
		return domain.Reservation{}, err
	}
	return domain.CopyReservation(res), nil
}

func (s *Store) restoreReservationLocked(res domain.Reservation) {
	for _, leg := range res.Legs {
		ride, ok := s.rides[leg.RideID]
		if !ok || ride.Status != domain.RideActive {
			continue
		}
		for _, si := range leg.SegmentIndexes {
			if si < 0 || si >= len(ride.Segments) {
				continue
			}
			if ride.Segments[si].AvailableSeats < ride.Capacity {
				ride.Segments[si].AvailableSeats++
			}
		}
		s.rides[leg.RideID] = ride
	}
}

// CancelRide cancela a carona inteira (não um trecho isolado).
// Reservas confirmadas que usam essa carona são canceladas. Em itinerários
// compostos, a capacidade das OUTRAS caronas ainda ativas é restaurada
// exatamente uma vez.
func (s *Store) CancelRide(driverID, rideID string) (domain.Ride, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ride, ok := s.rides[rideID]
	if !ok {
		return domain.Ride{}, fmt.Errorf("%w: ride", ErrNotFound)
	}
	if ride.DriverID != driverID {
		return domain.Ride{}, ErrForbidden
	}
	if ride.Status == domain.RideCancelled {
		return domain.CopyRide(ride), nil
	}

	ride.Status = domain.RideCancelled
	s.rides[rideID] = ride

	for id, res := range s.reservations {
		if res.Status != domain.ResConfirmed {
			continue
		}
		if reservationUsesRide(res, rideID) {
			s.restoreReservationLocked(res)
			res.Status = domain.ResCancelled
			s.reservations[id] = res
		}
	}

	if err := s.persistLocked(); err != nil {
		return domain.Ride{}, err
	}
	return domain.CopyRide(s.rides[rideID]), nil
}

func reservationUsesRide(res domain.Reservation, rideID string) bool {
	for _, leg := range res.Legs {
		if leg.RideID == rideID {
			return true
		}
	}
	return false
}

func (s *Store) ListRidePassengers(driverID, rideID string) (domain.Ride, [][]passengerSeat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ride, ok := s.rides[rideID]
	if !ok {
		return domain.Ride{}, nil, fmt.Errorf("%w: ride", ErrNotFound)
	}
	if ride.DriverID != driverID {
		return domain.Ride{}, nil, ErrForbidden
	}
	bySeg := make([][]passengerSeat, len(ride.Segments))
	for _, res := range s.reservations {
		if res.Status != domain.ResConfirmed {
			continue
		}
		u := s.usersByID[res.PassengerID]
		for _, leg := range res.Legs {
			if leg.RideID != rideID {
				continue
			}
			for _, si := range leg.SegmentIndexes {
				if si >= 0 && si < len(bySeg) {
					bySeg[si] = append(bySeg[si], passengerSeat{
						UserID:        res.PassengerID,
						Username:      u.Username,
						ReservationID: res.ReservationID,
					})
				}
			}
		}
	}
	return domain.CopyRide(ride), bySeg, nil
}

type passengerSeat struct {
	UserID        string
	Username      string
	ReservationID string
}

func (s *Store) RidePassengers(driverID, rideID string) (domain.Ride, [][]PassengerOnSegment, error) {
	ride, seats, err := s.ListRidePassengers(driverID, rideID)
	if err != nil {
		return domain.Ride{}, nil, err
	}
	out := make([][]PassengerOnSegment, len(seats))
	for i, list := range seats {
		for _, p := range list {
			out[i] = append(out[i], PassengerOnSegment(p))
		}
	}
	return ride, out, nil
}

type PassengerOnSegment struct {
	UserID        string
	Username      string
	ReservationID string
}
