package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Request é o envelope de toda operação.
// Data fica como json.RawMessage para o dispatcher decodificar o payload
// específico da operation sem um union type rígido.
type Request struct {
	Version   int             `json:"version"`
	Operation string          `json:"operation"`
	RequestID string          `json:"requestId"`
	Data      json.RawMessage `json:"data"`
}

type Response struct {
	Version   int             `json:"version"`
	RequestID string          `json:"requestId"`
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data"`
	Error     *ErrorBody      `json:"error"`
}

func DecodeRequest(raw []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *Request) Validate() error {
	if r.Version != ProtocolVersion {
		return fmt.Errorf("unsupported version %d (expected %d)", r.Version, ProtocolVersion)
	}
	if strings.TrimSpace(r.Operation) == "" {
		return fmt.Errorf("operation is required")
	}
	if strings.TrimSpace(r.RequestID) == "" {
		return fmt.Errorf("requestId is required")
	}
	return nil
}

func OK(requestID string, data any) Response {
	raw, err := json.Marshal(data)
	if err != nil {
		return Error(requestID, CodeInternalError, "failed to encode response")
	}
	if data == nil {
		raw = []byte("{}")
	}
	return Response{
		Version:   ProtocolVersion,
		RequestID: requestID,
		Status:    StatusOK,
		Data:      raw,
		Error:     nil,
	}
}

func Error(requestID, code, message string) Response {
	return Response{
		Version:   ProtocolVersion,
		RequestID: requestID,
		Status:    StatusError,
		Data:      nil,
		Error: &ErrorBody{
			Code:    code,
			Message: message,
		},
	}
}

func MarshalResponse(resp Response) ([]byte, error) {
	return json.Marshal(resp)
}

// PeekRequestID tenta extrair requestId mesmo de JSON incompleto/inválido
// no restante dos campos, para devolver o correlator ao cliente.
func PeekRequestID(raw []byte) string {
	var tmp struct {
		RequestID string `json:"requestId"`
	}
	_ = json.Unmarshal(raw, &tmp)
	return tmp.RequestID
}
