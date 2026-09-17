package protocol

// Payloads de request/response. Campos em camelCase JSON para um cliente
// em qualquer linguagem conseguir falar com o servidor lendo só o PROTOCOL.md.

// LoginData é o corpo de LOGIN (ainda sem sessão na conexão).
type LoginData struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResult devolve a identidade que o servidor gruda na Session da conexão.
type LoginResult struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// RegisterData cria conta DRIVER ou PASSENGER. O userId é gerado no servidor.
type RegisterData struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// RegisterResult devolve o userId gerado. A CLI pede LOGIN em seguida; não autentica sozinha.
type RegisterResult struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// PingResult é a resposta de PING (não exige login).
type PingResult struct {
	Message string `json:"message"`
}

// PublishRideData descreve a rota, data, hora, capacidade e preço por trecho (centavos).
type PublishRideData struct {
	Cities        []string `json:"cities"`
	DepartureDate string   `json:"departureDate"` // YYYY-MM-DD
	DepartureTime string   `json:"departureTime"` // HH:MM (24h)
	Capacity      int      `json:"capacity"`
	SegmentPrices []int64  `json:"segmentPrices"` // centavos por trecho, len = len(cities)-1
}

// RideView é a carona como o cliente a vê (sem senha, com trechos e vagas atuais).
type RideView struct {
	RideID        string        `json:"rideId"`
	DriverID      string        `json:"driverId"`
	Cities        []string      `json:"cities"`
	DepartureDate string        `json:"departureDate"`
	DepartureTime string        `json:"departureTime"`
	Capacity      int           `json:"capacity"`
	Status        string        `json:"status"`
	Segments      []SegmentView `json:"segments"`
}

// SegmentView é um trecho adjacente da rota, com preço e assentos disponíveis.
type SegmentView struct {
	SegmentIndex   int    `json:"segmentIndex"`
	Origin         string `json:"origin"`
	Destination    string `json:"destination"`
	Price          int64  `json:"price"`
	AvailableSeats int    `json:"availableSeats"`
}

// ListDriverRidesResult agrupa as caronas do motorista autenticado.
type ListDriverRidesResult struct {
	Rides []RideView `json:"rides"`
}

// CancelRideData identifica a carona inteira a cancelar (não um trecho isolado).
type CancelRideData struct {
	RideID string `json:"rideId"`
}

// ListRidePassengersData pede os passageiros confirmados daquela carona, agrupados por trecho.
type ListRidePassengersData struct {
	RideID string `json:"rideId"`
}

// SegmentPassengersView lista quem embarca naquele trecho (pode não ir até o fim da rota).
type SegmentPassengersView struct {
	SegmentIndex int              `json:"segmentIndex"`
	Origin       string           `json:"origin"`
	Destination  string           `json:"destination"`
	Passengers   []PassengerOnLeg `json:"passengers"`
}

// PassengerOnLeg identifica quem confirmou aquele trecho.
type PassengerOnLeg struct {
	UserID        string `json:"userId"`
	Username      string `json:"username"`
	ReservationID string `json:"reservationId"`
}

// ListRidePassengersResult é a carona fatiada por trecho com a lista de passageiros de cada um.
type ListRidePassengersResult struct {
	RideID   string                  `json:"rideId"`
	Segments []SegmentPassengersView `json:"segments"`
}

// SearchItinerariesData é origem, destino e data (YYYY-MM-DD) da busca do passageiro.
type SearchItinerariesData struct {
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Date        string `json:"date"` // YYYY-MM-DD
}

// ItineraryView é uma opção de viagem (direta ou composta). Não segura vaga.
type ItineraryView struct {
	TotalPrice int64     `json:"totalPrice"`
	Transfers  int       `json:"transfers"`
	Legs       []LegView `json:"legs"`
}

// LegView é um pedaço contínuo de uma carona dentro de um itinerário ou reserva.
type LegView struct {
	RideID         string `json:"rideId"`
	DriverID       string `json:"driverId"`
	Origin         string `json:"origin"`
	Destination    string `json:"destination"`
	DepartureDate  string `json:"departureDate"`
	DepartureTime  string `json:"departureTime"`
	Price          int64  `json:"price"`
	AvailableSeats int    `json:"availableSeats"`
}

// SearchItinerariesResult devolve até MaxResults opções, já ordenadas por preço.
type SearchItinerariesResult struct {
	Itineraries []ItineraryView `json:"itineraries"`
}

// ConfirmReservationData reenvia as legs escolhidas; o servidor revalida todas.
type ConfirmReservationData struct {
	Legs []ConfirmLeg `json:"legs"`
}

// ConfirmLeg aponta um intervalo contínuo numa carona (origin e destination na rota).
type ConfirmLeg struct {
	RideID      string `json:"rideId"`
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
}

// ReservationView é a reserva confirmada ou cancelada, com o preço total em centavos.
type ReservationView struct {
	ReservationID string    `json:"reservationId"`
	PassengerID   string    `json:"passengerId"`
	Status        string    `json:"status"`
	TotalPrice    int64     `json:"totalPrice"`
	CreatedAt     string    `json:"createdAt"`
	Legs          []LegView `json:"legs"`
}

// ConfirmReservationResult devolve a reserva recém-criada (ou a mesma, se o requestId repetir).
type ConfirmReservationResult struct {
	Reservation ReservationView `json:"reservation"`
}

// ListReservationsResult é o que o passageiro vê em "minhas reservas".
type ListReservationsResult struct {
	Reservations []ReservationView `json:"reservations"`
}

// CancelReservationData identifica a reserva a devolver. Segunda chamada é idempotente.
type CancelReservationData struct {
	ReservationID string `json:"reservationId"`
}

// CancelReservationResult ecoa a reserva já marcada CANCELLED.
type CancelReservationResult struct {
	Reservation ReservationView `json:"reservation"`
}
