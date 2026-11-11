package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/caarlos0/env/v11"
)

type configError string

func (e configError) Error() string { return string(e) }

const (
	ErrNonLoopbackAddr   configError = "listen address must be loopback"
	ErrInvalidDuration   configError = "invalid duration"
	ErrInvalidProfileDir configError = "profile directory must be an absolute path"
)

type Config struct {
	Addr           string        `env:"FTS_ADDR" envDefault:"127.0.0.1:8787"`
	BrowserChannel string        `env:"FTS_BROWSER_CHANNEL" envDefault:"chrome"`
	LoginTimeout   time.Duration `env:"FTS_LOGIN_TIMEOUT" envDefault:"10m"`
	RefreshMargin  time.Duration `env:"FTS_REFRESH_MARGIN" envDefault:"60s"`
	RefreshTick    time.Duration `env:"FTS_REFRESH_TICK" envDefault:"15s"`
	ProfileDir     string        `env:"FTS_PROFILE_DIR"`
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

	if cfg.ProfileDir == "" {
		dataHome := os.Getenv("XDG_DATA_HOME")
		if !filepath.IsAbs(dataHome) {
			home, err := os.UserHomeDir()
			if err != nil {
				return Config{}, fmt.Errorf("user home dir: %w", err)
			}
			dataHome = filepath.Join(home, ".local", "share")
		}
		cfg.ProfileDir = filepath.Join(dataHome, "fuck-toscana-salute", "chrome-profile")
	}
	if !filepath.IsAbs(cfg.ProfileDir) {
		return Config{}, ErrInvalidProfileDir
	}

	return cfg, nil
}
