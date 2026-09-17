package server

import (
	"io"
	"log"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/yma1001/vaijunto/internal/config"
	"github.com/yma1001/vaijunto/internal/store"
)

// StartTestServer sobe o servidor em 127.0.0.1:0 com state.json temporário.
// Não usa o data/state.json do clone, para o teste não misturar estado da demo.
func StartTestServer(t *testing.T) (config.Config, *store.Store, *Server) {
	t.Helper()
	st, err := store.New(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Load()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	cfg.ListenHost = host
	cfg.ListenPort = port
	cfg.ServerHost = host
	cfg.ServerPort = port
	cfg.MaxPayloadBytes = 1 << 20
	cfg.ReadTimeout = 3 * time.Second
	cfg.WriteTimeout = 3 * time.Second
	cfg.IdleTimeout = 5 * time.Second
	cfg.ConnectTimeout = 2 * time.Second
	silent := log.New(io.Discard, "", 0)
	srv := New(cfg, st, silent)
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return cfg, st, srv
}
