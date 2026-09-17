// Pacote client é a biblioteca TCP usada pelos CLIs, testes, smoke e loadtest.
// Mantém uma conexão persistente: depois do LOGIN, as próximas operações
// reutilizam o mesmo socket (a sessão mora no servidor, nesta conexão).
package client

import (
	"encoding/json"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/yma1001/vaijunto/internal/config"
	"github.com/yma1001/vaijunto/internal/protocol"
)

// Client é a biblioteca TCP usada pelos CLIs, testes e loadtest.
// Mantém uma conexão persistente após LOGIN, como a sessão do servidor.
type Client struct {
	cfg    config.Config
	conn   net.Conn
	seq    atomic.Int64
	UserID string
	Role   string
	Name   string
}

// Dial abre o TCP para SERVER_HOST:SERVER_PORT com timeout de connect.
func Dial(cfg config.Config) (*Client, error) {
	d := net.Dialer{Timeout: cfg.ConnectTimeout}
	conn, err := d.Dial("tcp", cfg.ServerAddr())
	if err != nil {
		return nil, err
	}
	return &Client{cfg: cfg, conn: conn}, nil
}

// Close encerra o socket. Não envia LOGOUT — use Logout se quiser o handshake.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// nextID gera requestId único o bastante para correlacionar e para a idempotência do CONFIRM.
func (c *Client) nextID() string {
	n := c.seq.Add(1)
	return fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), n)
}

// clearIdentity apaga o cache local de login; o servidor já esqueceu a Session quando o TCP caiu.
func (c *Client) clearIdentity() {
	c.UserID = ""
	c.Role = ""
	c.Name = ""
}

// Reconnect fecha o socket antigo e abre outro. A sessão TCP anterior morreu; precisa LOGIN de novo.
func (c *Client) Reconnect() error {
	d := net.Dialer{Timeout: c.cfg.ConnectTimeout}
	conn, err := d.Dial("tcp", c.cfg.ServerAddr())
	if err != nil {
		return err
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = conn
	return nil
}

// Logout envia LOGOUT (o servidor fecha o TCP), limpa a identidade local
// e abre uma conexão nova para o próximo LOGIN no mesmo processo.
func (c *Client) Logout() error {
	if c.conn != nil {
		_ = c.MustOK(protocol.OpLogout, map[string]any{}, nil)
		_ = c.Close()
		c.conn = nil
	}
	c.clearIdentity()
	return c.Reconnect()
}

// Call envia um frame (envelope JSON) e espera a resposta. Sem pipelining: um pedido por vez.
func (c *Client) Call(operation, requestID string, data any) (protocol.Response, error) {
	if c.conn == nil {
		return protocol.Response{}, fmt.Errorf("not connected")
	}
	if requestID == "" {
		requestID = c.nextID()
	}
	rawData, err := json.Marshal(data)
	if err != nil {
		return protocol.Response{}, err
	}
	if data == nil {
		rawData = []byte("{}")
	}
	req := protocol.Request{
		Version:   protocol.ProtocolVersion,
		Operation: operation,
		RequestID: requestID,
		Data:      rawData,
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return protocol.Response{}, err
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteTimeout))
	if err := protocol.WriteFrame(c.conn, payload); err != nil {
		return protocol.Response{}, err
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(c.cfg.ReadTimeout))
	if c.cfg.ReadTimeout == 0 {
		_ = c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	}
	raw, err := protocol.ReadFrame(c.conn, c.cfg.MaxPayloadBytes)
	if err != nil {
		return protocol.Response{}, err
	}
	var resp protocol.Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return protocol.Response{}, err
	}
	return resp, nil
}

// MustOK chama Call e falha se status != OK. dest, se não for nil, recebe o data da resposta.
func (c *Client) MustOK(operation string, data any, dest any) error {
	resp, err := c.Call(operation, "", data)
	if err != nil {
		return err
	}
	if resp.Status != protocol.StatusOK {
		code := ""
		msg := ""
		if resp.Error != nil {
			code, msg = resp.Error.Code, resp.Error.Message
		}
		return fmt.Errorf("%s failed: %s %s", operation, code, msg)
	}
	if dest != nil && len(resp.Data) > 0 {
		return json.Unmarshal(resp.Data, dest)
	}
	return nil
}

// Ping envia PING sem login. Serve para checar se o servidor está no ar.
func (c *Client) Ping() error {
	var out protocol.PingResult
	return c.MustOK(protocol.OpPing, map[string]any{}, &out)
}

// Login autentica e guarda userId/role localmente, espelhando a Session do servidor.
func (c *Client) Login(user, pass string) error {
	if c.conn == nil {
		if err := c.Reconnect(); err != nil {
			return err
		}
	}
	var out protocol.LoginResult
	if err := c.MustOK(protocol.OpLogin, protocol.LoginData{Username: user, Password: pass}, &out); err != nil {
		return err
	}
	c.UserID = out.UserID
	c.Role = out.Role
	c.Name = out.Username
	return nil
}

// Register cria a conta no servidor. Não deixa a conexão autenticada.
func (c *Client) Register(user, pass, role string) (protocol.RegisterResult, error) {
	var out protocol.RegisterResult
	if err := c.MustOK(protocol.OpRegister, protocol.RegisterData{
		Username: user, Password: pass, Role: role,
	}, &out); err != nil {
		return protocol.RegisterResult{}, err
	}
	return out, nil
}

// Conn expõe o socket cru (testes de frame incompleto / disconnect).
func (c *Client) Conn() net.Conn { return c.conn }
