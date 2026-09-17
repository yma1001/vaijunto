package store

import (
	"fmt"

	"github.com/yma1001/vaijunto/internal/domain"
)

// confirmKey é a chave de idempotência do CONFIRM: o mesmo pedido não vende a vaga duas vezes.
func confirmKey(passengerID, requestID string) string {
	return passengerID + "|" + requestID
}

// decrement é um assento a descontar depois que o plano inteiro passou na revalidação.
type decrement struct {
	rideID string
	index  int
}

// segKey identifica um trecho físico. Pedido com o mesmo par duas vezes é VALIDATION_ERROR.
type segKey struct {
	rideID string
	index  int
}

// ConfirmReservation é a seção crítica da atomicidade (INV-3, INV-10).
//
// Passos, todos ainda sob s.mu.Lock:
//  1. idempotência por (passenger, requestId) — retransmissão devolve a mesma reserva
//  2. expandir cada leg em índices de segmento (caminho válido na carona)
//  3. mesma data em todas as caronas (regra same-day da busca; DEC-009)
//  4. rejeitar (rideId, segmentIndex) repetido no mesmo pedido — VALIDATION_ERROR,
//     sem deduplicar em silêncio (evita decremento duplo / disponibilidade negativa)
//  5. REVALIDAR disponibilidade de TODOS os trechos (a busca era só snapshot)
//  6. legs consecutivas devem conectar destino → origem seguinte
//  7. se qualquer checagem falhar: Unlock implícito no defer, estado intacto
//  8. só então decrementar, gravar reserva, persistir
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

	decrements, legs, total, err := s.planConfirmLocked(inputs)
	if err != nil {
		return domain.Reservation{}, err
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

// planConfirmLocked valida o pedido inteiro e monta a lista de decrementos.
// Não muta rides, reservations, confirmIndex nem o arquivo.
func (s *Store) planConfirmLocked(inputs []domain.LegInput) ([]decrement, []domain.Leg, int64, error) {
	var decrements []decrement
	var legs []domain.Leg
	var total int64
	seen := map[segKey]struct{}{}
	var prevDest string
	var itineraryDate string

	for i, in := range inputs {
		ride, ok := s.rides[in.RideID]
		if !ok {
			return nil, nil, 0, fmt.Errorf("%w: ride %s", ErrNotFound, in.RideID)
		}
		if ride.Status != domain.RideActive {
			return nil, nil, 0, fmt.Errorf("%w: ride %s not active", ErrConflict, in.RideID)
		}
		idxs, ok := domain.SegmentIndexesBetween(ride.Cities, in.Origin, in.Destination)
		if !ok {
			return nil, nil, 0, fmt.Errorf("%w: %s -> %s not on ride %s", ErrValidation, in.Origin, in.Destination, in.RideID)
		}
		origin := ride.Cities[idxs[0]]
		dest := ride.Cities[idxs[len(idxs)-1]+1]
		if itineraryDate == "" {
			itineraryDate = ride.DepartureDate
		} else if ride.DepartureDate != itineraryDate {
			return nil, nil, 0, fmt.Errorf("%w: incompatible dates %s vs %s", ErrValidation, itineraryDate, ride.DepartureDate)
		}

		var price int64
		minAvail := ride.Capacity
		for _, si := range idxs {
			key := segKey{rideID: ride.RideID, index: si}
			if _, dup := seen[key]; dup {
				return nil, nil, 0, fmt.Errorf("%w: overlapping segment %s[%d]", ErrValidation, ride.RideID, si)
			}
			seen[key] = struct{}{}
			seg := ride.Segments[si]
			if seg.AvailableSeats < 1 {
				// Falha no último trecho (ou em qualquer um): aborta SEM mutar.
				return nil, nil, 0, ErrNoSeats
			}
			if seg.AvailableSeats < minAvail {
				minAvail = seg.AvailableSeats
			}
			decrements = append(decrements, decrement{rideID: ride.RideID, index: si})
			price += seg.Price
		}
		if i > 0 && !domain.CitiesEqual(prevDest, origin) {
			return nil, nil, 0, fmt.Errorf("%w: disconnected itinerary (%s -> %s)", ErrValidation, prevDest, origin)
		}
		legs = append(legs, domain.Leg{
			RideID:         ride.RideID,
			DriverID:       ride.DriverID,
			Origin:         origin,
			Destination:    dest,
			DepartureDate:  ride.DepartureDate,
			DepartureTime:  ride.DepartureTime,
			Price:          price,
			AvailableSeats: minAvail,
			SegmentIndexes: idxs,
		})
		total += price
		prevDest = dest
	}
	return decrements, legs, total, nil
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

// restoreReservationLocked devolve um assento em cada segmento da reserva, sem passar da capacity.
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

// reservationUsesRide diz se alguma leg da reserva aponta para aquela carona.
func reservationUsesRide(res domain.Reservation, rideID string) bool {
	for _, leg := range res.Legs {
		if leg.RideID == rideID {
			return true
		}
	}
	return false
}

// ListRidePassengers agrupa passageiros confirmados por índice de trecho, para o motorista acompanhar.
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

// passengerSeat é o registro interno de quem ocupa um trecho.
type passengerSeat struct {
	UserID        string
	Username      string
	ReservationID string
}

// RidePassengers é a versão exportada usada pelo servidor na operação LIST_RIDE_PASSENGERS.
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

// PassengerOnSegment é o que sai no JSON: userId, username e reservationId daquele trecho.
type PassengerOnSegment struct {
	UserID        string
	Username      string
	ReservationID string
}
