package domain

import "time"

const (
	RideActive    = "ACTIVE"
	RideCancelled = "CANCELLED"

	ResConfirmed = "CONFIRMED"
	ResCancelled = "CANCELLED"
)

type User struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Password string `json:"password"` // protótipo: texto plano. NÃO é segurança de produção.
	Role     string `json:"role"`
}

type Ride struct {
	RideID        string    `json:"rideId"`
	DriverID      string    `json:"driverId"`
	Cities        []string  `json:"cities"`
	DepartureDate string    `json:"departureDate"`
	DepartureTime string    `json:"departureTime"`
	Capacity      int       `json:"capacity"`
	Status        string    `json:"status"`
	Segments      []Segment `json:"segments"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Segment é um trecho adjacente da rota (cidades[i] → cidades[i+1]).
// A disponibilidade é POR TRECHO, não pela carona inteira: um passageiro
// A→C consome A-B e B-C, mas deixa C-D livre.
type Segment struct {
	RideID         string `json:"rideId"`
	SegmentIndex   int    `json:"segmentIndex"`
	Origin         string `json:"origin"`
	Destination    string `json:"destination"`
	Price          int64  `json:"price"` // centavos
	AvailableSeats int    `json:"availableSeats"`
}

type Reservation struct {
	ReservationID string    `json:"reservationId"`
	PassengerID   string    `json:"passengerId"`
	RequestID     string    `json:"requestId"`
	Status        string    `json:"status"`
	TotalPrice    int64     `json:"totalPrice"`
	Legs          []Leg     `json:"legs"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Leg é a porção contínua de UMA carona usada na reserva (pode cobrir
// vários Segment internos).
type Leg struct {
	RideID         string `json:"rideId"`
	DriverID       string `json:"driverId"`
	Origin         string `json:"origin"`
	Destination    string `json:"destination"`
	DepartureDate  string `json:"departureDate"`
	DepartureTime  string `json:"departureTime"`
	Price          int64  `json:"price"`
	AvailableSeats int    `json:"availableSeats"`
	SegmentIndexes []int  `json:"segmentIndexes"`
}

type Itinerary struct {
	TotalPrice int64
	Transfers  int
	Legs       []Leg
}

type LegInput struct {
	RideID      string
	Origin      string
	Destination string
}
