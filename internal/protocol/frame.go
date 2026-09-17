package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Framing sobre TCP.
//
// TCP entrega um fluxo de bytes, não mensagens. Dois Write consecutivos podem
// chegar coalescidos ou fatiados em vários Read. Sem fronteira explícita o
// receptor não sabe onde termina um JSON e começa o próximo.
//
// Formato escolhido (decisão de projeto, não requisito literal do enunciado):
//
//	[4 bytes uint32 big-endian = N][N bytes de payload UTF-8 JSON]
//
// Big-endian (network byte order) é o convencional em protocolos de rede e
// permite que um cliente Python faça struct.unpack(">I", header).
// io.ReadFull garante que lemos exatamente 4 bytes e depois exatamente N bytes,
// mesmo quando o sistema operacional entrega o segmento TCP aos pedaços.

var (
	ErrFrameTooLarge   = errors.New("frame exceeds MAX_PAYLOAD")
	ErrFrameEmpty      = errors.New("frame length is zero")
	ErrIncompleteFrame = errors.New("incomplete frame")
	ErrPayloadTooLarge = errors.New("payload exceeds MAX_PAYLOAD")
)

// ReadFrame lê exatamente 4 bytes de tamanho e depois N bytes de JSON.
// io.ReadFull espera o restante se o SO entregar o segmento TCP aos pedaços.
func ReadFrame(r io.Reader, maxPayload int) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("%w: header: %v", ErrIncompleteFrame, err)
		}
		return nil, err
	}
	n := binary.BigEndian.Uint32(header[:])
	if n == 0 {
		return nil, ErrFrameEmpty
	}
	if int(n) > maxPayload {
		// Não tentamos ler o restante: um N absurdo esgotaria memória.
		// A conexão deve ser fechada pelo caller.
		return nil, fmt.Errorf("%w: %d > %d", ErrFrameTooLarge, n, maxPayload)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("%w: payload: %v", ErrIncompleteFrame, err)
		}
		return nil, err
	}
	return buf, nil
}

// WriteFrame envia o header big-endian e o payload, repetindo Write se for parcial.
func WriteFrame(w io.Writer, payload []byte) error {
	if len(payload) == 0 {
		return ErrFrameEmpty
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
	if err := writeFull(w, header[:]); err != nil {
		return err
	}
	return writeFull(w, payload)
}

// writeFull trata Write parcial. O contrato de io.Writer permite n < len(p)
// sem erro; em sockets isso acontece. Sem o loop, o frame sairia truncado.
func writeFull(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		p = p[n:]
	}
	return nil
}

// MaxPayloadOK é o mesmo limite usado no header, exposto para testes e o cliente.
func MaxPayloadOK(n, max int) error {
	if n > max {
		return ErrPayloadTooLarge
	}
	return nil
}
