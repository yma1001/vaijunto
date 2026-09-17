package protocol

import "testing"

// TestRequestValidate: version ≠ 1 ou operation vazia são rejeitados no envelope.
func TestRequestValidate(t *testing.T) {
	ok := &Request{Version: 1, Operation: "PING", RequestID: "a"}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := &Request{Version: 2, Operation: "PING", RequestID: "a"}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected version error")
	}
	empty := &Request{Version: 1, Operation: "", RequestID: "a"}
	if err := empty.Validate(); err == nil {
		t.Fatal("expected operation error")
	}
}

// TestPeekRequestID: tenta ecoar o correlator mesmo com JSON incompleto.
func TestPeekRequestID(t *testing.T) {
	if PeekRequestID([]byte(`{"requestId":"xyz","operation":"PING"}`)) != "xyz" {
		t.Fatal("should extract requestId")
	}
	if PeekRequestID([]byte(`not-json`)) != "" {
		t.Fatal("garbage should yield empty id")
	}
}

// TestErrorEnvelope: resposta ERROR carrega code e o mesmo requestId.
func TestErrorEnvelope(t *testing.T) {
	resp := Error("r1", CodeNoSeats, "full")
	if resp.Status != StatusError || resp.Error.Code != CodeNoSeats || resp.RequestID != "r1" {
		t.Fatalf("%+v", resp)
	}
}
