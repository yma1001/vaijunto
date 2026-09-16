package protocol

// Payloads de request/response. Campos em camelCase JSON para interoperar
// com clientes em qualquer linguagem.

type LoginData struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResult struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type RegisterData struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type RegisterResult struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type PingResult struct {
	Message string `json:"message"`
}

type PublishRideData struct {
	Cities        []string `json:"cities"`
	DepartureDate string   `json:"departureDate"` // YYYY-MM-DD
	DepartureTime string   `json:"departureTime"` // HH:MM (24h)
	Capacity      int      `json:"capacity"`
	SegmentPrices []int64  `json:"segmentPrices"` // centavos por trecho, len = len(cities)-1
}

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

type SegmentView struct {
	SegmentIndex   int    `json:"segmentIndex"`
	Origin         string `json:"origin"`
	Destination    string `json:"destination"`
	Price          int64  `json:"price"`
	AvailableSeats int    `json:"availableSeats"`
}

type ListDriverRidesResult struct {
	Rides []RideView `json:"rides"`
}

type CancelRideData struct {
	RideID string `json:"rideId"`
}

type ListRidePassengersData struct {
	RideID string `json:"rideId"`
}

type SegmentPassengersView struct {
	SegmentIndex int              `json:"segmentIndex"`
	Origin       string           `json:"origin"`
	Destination  string           `json:"destination"`
	Passengers   []PassengerOnLeg `json:"passengers"`
}

type PassengerOnLeg struct {
	UserID        string `json:"userId"`
	Username      string `json:"username"`
	ReservationID string `json:"reservationId"`
}

type ListRidePassengersResult struct {
	RideID   string                  `json:"rideId"`
	Segments []SegmentPassengersView `json:"segments"`
}

type SearchItinerariesData struct {
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Date        string `json:"date"` // YYYY-MM-DD
}

type ItineraryView struct {
	TotalPrice int64     `json:"totalPrice"`
	Transfers  int       `json:"transfers"`
	Legs       []LegView `json:"legs"`
}

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

type SearchItinerariesResult struct {
	Itineraries []ItineraryView `json:"itineraries"`
}

type ConfirmReservationData struct {
	Legs []ConfirmLeg `json:"legs"`
}

type ConfirmLeg struct {
	RideID      string `json:"rideId"`
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
}

type ReservationView struct {
	ReservationID string     `json:"reservationId"`
	PassengerID   string     `json:"passengerId"`
	Status        string     `json:"status"`
	TotalPrice    int64      `json:"totalPrice"`
	CreatedAt     string     `json:"createdAt"`
	Legs          []LegView  `json:"legs"`
}

type ConfirmReservationResult struct {
	Reservation ReservationView `json:"reservation"`
}

type ListReservationsResult struct {
	Reservations []ReservationView `json:"reservations"`
}

type CancelReservationData struct {
	ReservationID string `json:"reservationId"`
}

type CancelReservationResult struct {
	Reservation ReservationView `json:"reservation"`
}
