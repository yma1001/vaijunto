package protocol

// Códigos de erro estáveis do protocolo de aplicação.
// O cliente de outra linguagem deve tratar estes valores, não mensagens em português.
const (
	CodeInvalidFrame        = "INVALID_FRAME"
	CodeInvalidJSON         = "INVALID_JSON"
	CodeValidationError     = "VALIDATION_ERROR"
	CodeUnknownOperation    = "UNKNOWN_OPERATION"
	CodeUnauthenticated     = "UNAUTHENTICATED"
	CodeForbidden           = "FORBIDDEN"
	CodeNotFound            = "NOT_FOUND"
	CodeNoSeats             = "NO_SEATS"
	CodeReservationConflict = "RESERVATION_CONFLICT"
	CodeAlreadyCancelled    = "ALREADY_CANCELLED"
	CodeInternalError       = "INTERNAL_ERROR"
)

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
