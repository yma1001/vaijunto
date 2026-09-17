package protocol

// Códigos de erro estáveis do protocolo de aplicação.
// O cliente de outra linguagem trata estes valores, não o texto em inglês/português.
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

// ErrorBody vai no campo error da Response quando status=ERROR.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
