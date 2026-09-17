// Pacote config lê host, porta, caminhos e timeouts das variáveis de ambiente.
// LISTEN_HOST/SERVER_PORT valem para o servidor; SERVER_HOST para os clientes.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config agrupa parâmetros de execução. São limites do protótipo
// (payload, baldeações, timeouts), todos sobrescrevíveis por ambiente.
type Config struct {
	ListenHost string
	ListenPort string

	ServerHost string
	ServerPort string

	DataPath string

	MaxPayloadBytes int
	MaxTransfers    int
	MaxResults      int

	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	ShutdownWait   time.Duration
	ConnectTimeout time.Duration
}

// Load preenche Config com defaults (0.0.0.0:5000, 1 MiB, 3 baldeações, 10 min idle).
func Load() Config {
	return Config{
		ListenHost:      env("LISTEN_HOST", "0.0.0.0"),
		ListenPort:      env("SERVER_PORT", "5000"),
		ServerHost:      env("SERVER_HOST", "127.0.0.1"),
		ServerPort:      env("SERVER_PORT", "5000"),
		DataPath:        env("DATA_PATH", "data/state.json"),
		MaxPayloadBytes: envInt("MAX_PAYLOAD", 1<<20), // 1 MiB — limite técnico, não oficial
		MaxTransfers:    envInt("MAX_TRANSFERS", 3),
		MaxResults:      envInt("MAX_RESULTS", 20),
		ReadTimeout:     envDuration("READ_TIMEOUT", 30*time.Second),
		WriteTimeout:    envDuration("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:     envDuration("IDLE_TIMEOUT", 10*time.Minute),
		ShutdownWait:    envDuration("SHUTDOWN_WAIT", 5*time.Second),
		ConnectTimeout:  envDuration("CONNECT_TIMEOUT", 10*time.Second),
	}
}

// ListenAddr é o bind do servidor (host:porta). 0.0.0.0 permite outro PC na LAN.
func (c Config) ListenAddr() string {
	return c.ListenHost + ":" + c.ListenPort
}

// ServerAddr é o destino do Dial dos clientes (SERVER_HOST:SERVER_PORT).
func (c Config) ServerAddr() string {
	return c.ServerHost + ":" + c.ServerPort
}

// env lê string do ambiente ou devolve o default.
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envInt lê um inteiro do ambiente; valor inválido cai no default (não aborta a subida).
func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// envDuration aceita strings no formato do time.ParseDuration ("30s", "10m").
func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
