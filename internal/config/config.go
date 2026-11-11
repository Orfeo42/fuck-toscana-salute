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
	ErrInvalidDataDir    configError = "data directory must be an absolute path"
	ErrInvalidLoginMode  configError = "login mode must be window or bookmarklet"
)

const (
	LoginModeWindow      = "window"
	LoginModeBookmarklet = "bookmarklet"
)

type Config struct {
	Addr           string        `env:"FTS_ADDR" envDefault:"127.0.0.1:8787"`
	BrowserChannel string        `env:"FTS_BROWSER_CHANNEL" envDefault:"chrome"`
	LoginTimeout   time.Duration `env:"FTS_LOGIN_TIMEOUT" envDefault:"10m"`
	RefreshMargin  time.Duration `env:"FTS_REFRESH_MARGIN" envDefault:"60s"`
	RefreshTick    time.Duration `env:"FTS_REFRESH_TICK" envDefault:"15s"`
	ProfileDir     string        `env:"FTS_PROFILE_DIR"`
	DataDir        string        `env:"FTS_DATA_DIR"`
	LoginMode      string        `env:"FTS_LOGIN_MODE" envDefault:"window"`
	ListenAny      bool          `env:"FTS_LISTEN_ANY"`
	SecretKeyFile  string        `env:"FTS_SECRET_KEY_FILE"`

	GoogleClientFile string `env:"FTS_GOOGLE_CLIENT_FILE"`
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
	if !cfg.ListenAny && host != "127.0.0.1" && host != "::1" && host != "localhost" {
		return Config{}, ErrNonLoopbackAddr
	}

	if cfg.LoginTimeout <= 0 || cfg.RefreshTick <= 0 || cfg.RefreshMargin < 0 {
		return Config{}, ErrInvalidDuration
	}

	if cfg.LoginMode != LoginModeWindow && cfg.LoginMode != LoginModeBookmarklet {
		return Config{}, ErrInvalidLoginMode
	}

	if cfg.DataDir == "" || cfg.ProfileDir == "" {
		appData, err := defaultDataDir()
		if err != nil {
			return Config{}, err
		}
		if cfg.DataDir == "" {
			cfg.DataDir = appData
		}
		if cfg.ProfileDir == "" {
			cfg.ProfileDir = filepath.Join(appData, "chrome-profile")
		}
	}
	if !filepath.IsAbs(cfg.ProfileDir) {
		return Config{}, ErrInvalidProfileDir
	}
	if !filepath.IsAbs(cfg.DataDir) {
		return Config{}, ErrInvalidDataDir
	}

	if cfg.GoogleClientFile == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return Config{}, fmt.Errorf("user config dir: %w", err)
		}
		cfg.GoogleClientFile = filepath.Join(configDir, "fuck-toscana-salute", "google-client.json")
	}

	return cfg, nil
}

func defaultDataDir() (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(dataHome) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("user home dir: %w", err)
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "fuck-toscana-salute"), nil
}
