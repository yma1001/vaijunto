// Pacote domain descreve o modelo de dados do servidor: usuário, carona,
// trecho, reserva e itinerário. Essas structs são a fonte de verdade em
// memória (Store) e no arquivo state.json.
package domain

import "time"

const (
	RideActive    = "ACTIVE"
	RideCancelled = "CANCELLED"

	ResConfirmed = "CONFIRMED"
	ResCancelled = "CANCELLED"
)

// User é uma conta do protótipo (motorista ou passageiro).
type User struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Password string `json:"password"` // protótipo: texto plano. NÃO é segurança de produção.
	Role     string `json:"role"`
}

// Ride é uma carona publicada por um motorista. Cities é a rota ordenada;
// cada par adjacente vira um Segment com vaga e preço próprios.
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

// Reservation é a posse de vaga depois de um CONFIRM atômico.
// Legs pode misturar caronas de motoristas diferentes (itinerário composto).
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

// Itinerary é só o resultado da busca. Não é entidade persistida e não
// segura vaga: o passageiro reenvia as legs no CONFIRM_RESERVATION.
type Itinerary struct {
	TotalPrice int64
	Transfers  int
	Legs       []Leg
}

// LegInput é o que o cliente manda no confirm: carona + intervalo de cidades.
type LegInput struct {
	RideID      string
	Origin      string
	Destination string
}
