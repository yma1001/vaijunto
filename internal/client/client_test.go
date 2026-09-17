package client

import (
	"testing"

	"github.com/yma1001/vaijunto/internal/protocol"
	"github.com/yma1001/vaijunto/internal/server"
)

func TestLogoutClearsIdentityAndReconnects(t *testing.T) {
	cfg, _, _ := server.StartTestServer(t)
	c, err := Dial(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Login("passageiro1", "senha123"); err != nil {
		t.Fatal(err)
	}
	if c.Role != protocol.RolePassenger || c.UserID == "" {
		t.Fatalf("login: %+v", c)
	}
	if err := c.Logout(); err != nil {
		t.Fatal(err)
	}
	if c.Role != "" || c.UserID != "" || c.Name != "" {
		t.Fatalf("identity still set after logout: role=%q user=%q name=%q", c.Role, c.UserID, c.Name)
	}
	if err := c.Ping(); err != nil {
		t.Fatalf("new connection should accept PING: %v", err)
	}
	if err := c.Login("passageiro2", "senha123"); err != nil {
		t.Fatal(err)
	}
	if c.Name != "passageiro2" || c.Role != protocol.RolePassenger {
		t.Fatalf("switch user: name=%q role=%q", c.Name, c.Role)
	}
}

func TestLogoutDoesNotDeleteAccounts(t *testing.T) {
	cfg, st, _ := server.StartTestServer(t)
	c, err := Dial(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Login("motorista1", "senha123"); err != nil {
		t.Fatal(err)
	}
	if err := c.Logout(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Authenticate("motorista1", "senha123"); err != nil {
		t.Fatal("logout must not remove persisted accounts")
	}
}
