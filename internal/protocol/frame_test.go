package protocol

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"testing"
)

func TestWriteReadRoundTrip(t *testing.T) {
	payload := []byte(`{"version":1,"operation":"PING","requestId":"a"}`)
	var buf bytes.Buffer
	if err := WriteFrame(&buf, payload); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFrame(&buf, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %s", got)
	}
}

func TestCoalescedMessages(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteFrame(&buf, []byte(`{"a":1}`))
	_ = WriteFrame(&buf, []byte(`{"b":2}`))
	a, err := ReadFrame(&buf, 1024)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ReadFrame(&buf, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != `{"a":1}` || string(b) != `{"b":2}` {
		t.Fatalf("a=%s b=%s", a, b)
	}
}

func TestPartialReads(t *testing.T) {
	payload := []byte(`{"hello":"world-partial-read"}`)
	var raw bytes.Buffer
	_ = WriteFrame(&raw, payload)
	r := &oneByteReader{r: bytes.NewReader(raw.Bytes())}
	got, err := ReadFrame(r, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %s", got)
	}
}

func TestIncompleteHeader(t *testing.T) {
	r := bytes.NewReader([]byte{0x00, 0x00})
	_, err := ReadFrame(r, 1024)
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("expected incomplete, got %v", err)
	}
}

func TestIncompletePayload(t *testing.T) {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], 10)
	r := io.MultiReader(bytes.NewReader(hdr[:]), bytes.NewReader([]byte("abc")))
	_, err := ReadFrame(r, 1024)
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("expected incomplete, got %v", err)
	}
}

func TestFrameTooLarge(t *testing.T) {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], 5000)
	_, err := ReadFrame(bytes.NewReader(hdr[:]), 100)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected too large, got %v", err)
	}
}

func TestEmptyFrameRejected(t *testing.T) {
	var hdr [4]byte
	_, err := ReadFrame(bytes.NewReader(hdr[:]), 100)
	if err == nil {
		t.Fatal("expected error")
	}
}

type oneByteReader struct {
	r io.Reader
}

func (o *oneByteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return o.r.Read(p[:1])
}
