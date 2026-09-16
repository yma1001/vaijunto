package protocol

// Operações do protocolo único (motorista e passageiro compartilham o envelope).
// O papel da sessão decide o que é autorizado.
const (
	OpPing                = "PING"
	OpLogin               = "LOGIN"
	OpLogout              = "LOGOUT"
	OpRegister            = "REGISTER"
	OpPublishRide         = "PUBLISH_RIDE"
	OpListDriverRides     = "LIST_DRIVER_RIDES"
	OpCancelRide          = "CANCEL_RIDE"
	OpListRidePassengers  = "LIST_RIDE_PASSENGERS"
	OpSearchItineraries   = "SEARCH_ITINERARIES"
	OpConfirmReservation  = "CONFIRM_RESERVATION"
	OpListReservations    = "LIST_RESERVATIONS"
	OpCancelReservation   = "CANCEL_RESERVATION"
)

const (
	RoleDriver    = "DRIVER"
	RolePassenger = "PASSENGER"
)

const (
	StatusOK    = "OK"
	StatusError = "ERROR"
)

const ProtocolVersion = 1
