package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	ErrNonLoopbackViewer configError = "viewer address must be loopback"

	ErrDatabasePassword      configError = "cannot read database password file"
	ErrDatabasePasswordEmpty configError = "database password file is empty"
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
	SecretKeyFile  string        `env:"FTS_SECRET_KEY_FILE,required"`
	ViewerAddr     string        `env:"FTS_VIEWER_ADDR"`

	DatabaseHost         string `env:"FTS_DATABASE_HOST,required"`
	DatabasePort         int    `env:"FTS_DATABASE_PORT" envDefault:"5432"`
	DatabaseName         string `env:"FTS_DATABASE_NAME" envDefault:"fts"`
	DatabaseUser         string `env:"FTS_DATABASE_USER" envDefault:"fts"`
	DatabasePasswordFile string `env:"FTS_DATABASE_PASSWORD_FILE,required"`
}

func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}

	if _, _, err := net.SplitHostPort(cfg.Addr); err != nil {
		return Config{}, fmt.Errorf("split addr: %w", ErrNonLoopbackAddr)
	}
	if !cfg.ListenAny && !loopbackAddr(cfg.Addr) {
		return Config{}, ErrNonLoopbackAddr
	}

	if cfg.LoginTimeout <= 0 || cfg.RefreshTick <= 0 || cfg.RefreshMargin < 0 {
		return Config{}, ErrInvalidDuration
	}

	if cfg.ViewerAddr != "" && !loopbackAddr(cfg.ViewerAddr) {
		return Config{}, ErrNonLoopbackViewer
	}

	if cfg.LoginMode != LoginModeWindow && cfg.LoginMode != LoginModeBookmarklet {
		return Config{}, ErrInvalidLoginMode
	}

	if cfg.DataDir == "" {
		appData, err := defaultDataDir()
		if err != nil {
			return Config{}, err
		}
		cfg.DataDir = appData
	}
	if !filepath.IsAbs(cfg.DataDir) {
		return Config{}, ErrInvalidDataDir
	}
	if cfg.ProfileDir == "" {
		cfg.ProfileDir = filepath.Join(cfg.DataDir, "chrome-profile")
	}
	if !filepath.IsAbs(cfg.ProfileDir) {
		return Config{}, ErrInvalidProfileDir
	}

	return cfg, nil
}

func (c Config) DatabaseDSN() (string, error) {
	raw, err := os.ReadFile(c.DatabasePasswordFile)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrDatabasePassword, err)
	}
	password := strings.TrimSpace(string(raw))
	if password == "" {
		return "", ErrDatabasePasswordEmpty
	}
	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.DatabaseUser, password),
		Host:     net.JoinHostPort(c.DatabaseHost, strconv.Itoa(c.DatabasePort)),
		Path:     "/" + c.DatabaseName,
		RawQuery: "sslmode=disable",
	}
	return dsn.String(), nil
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

func loopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}
