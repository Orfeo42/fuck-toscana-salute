package config

import (
	"fmt"
	"net"
	"time"

	"github.com/caarlos0/env/v11"
)

type configError string

func (e configError) Error() string { return string(e) }

const (
	ErrNonLoopbackAddr configError = "listen address must be loopback"
	ErrInvalidDuration configError = "invalid duration"
)

type Config struct {
	Addr           string        `env:"FTS_ADDR" envDefault:"127.0.0.1:8787"`
	BrowserChannel string        `env:"FTS_BROWSER_CHANNEL" envDefault:"chrome"`
	LoginTimeout   time.Duration `env:"FTS_LOGIN_TIMEOUT" envDefault:"10m"`
	RefreshMargin  time.Duration `env:"FTS_REFRESH_MARGIN" envDefault:"60s"`
	RefreshTick    time.Duration `env:"FTS_REFRESH_TICK" envDefault:"15s"`
}

func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}

	host, _, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return Config{}, fmt.Errorf("split addr: %w", ErrNonLoopbackAddr)
	}
	if host != "127.0.0.1" && host != "::1" && host != "localhost" {
		return Config{}, ErrNonLoopbackAddr
	}

	if cfg.LoginTimeout <= 0 || cfg.RefreshTick <= 0 || cfg.RefreshMargin < 0 {
		return Config{}, ErrInvalidDuration
	}

	return cfg, nil
}
