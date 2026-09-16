package config

import (
	"os"
	"strconv"
	"time"
)

// Config agrupa parâmetros de execução. Nada disso é requisito do enunciado:
// são limites e defaults do protótipo, todos sobrescrevíveis por ambiente.
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

func (c Config) ListenAddr() string {
	return c.ListenHost + ":" + c.ListenPort
}

func (c Config) ServerAddr() string {
	return c.ServerHost + ":" + c.ServerPort
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

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
