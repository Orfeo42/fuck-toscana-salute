package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/config"
	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/google"
	"fuck-toscana-salute/internal/httpapi"
	"fuck-toscana-salute/internal/keycloak"
	"fuck-toscana-salute/internal/login"
	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
	"fuck-toscana-salute/internal/vault"
	"fuck-toscana-salute/internal/webui"
)

const (
	lookupConcurrency = 4
	superviseTick     = time.Minute
	keyringService    = "fuck-toscana-salute"
	keyringUser       = "session"
	keyringGoogleUser = "google"
	secretsFile       = "secrets.enc"
)

type documents struct {
	*fse.Client
	bookings *cup.Client
}

func (d documents) Promemoria(ctx context.Context, bookingID string) ([]byte, error) {
	return d.bookings.Promemoria(ctx, bookingID)
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.ErrorContext(ctx, "fts failed", slog.String("error", err.Error()))
		stop()
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	return serve(ctx, cfg, ln)
}

func serve(ctx context.Context, cfg config.Config, ln net.Listener) error {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	secrets, err := secretBackend(ctx, cfg)
	if err != nil {
		return err
	}
	store := session.NewStore(keycloak.New(httpClient, keycloak.TokenURL), vault.NewWithBackend(secrets, keyringService, keyringUser), cfg.RefreshMargin)
	if err := store.Restore(ctx); err != nil {
		slog.WarnContext(ctx, "session restore failed", slog.String("error", err.Error()))
	}
	go store.Run(ctx, cfg.RefreshTick)

	flow := loginflow.New(login.New(cfg.BrowserChannel, cfg.ProfileDir, cfg.LoginTimeout), store)
	if cfg.LoginMode == config.LoginModeWindow {
		go flow.Supervise(ctx, store, superviseTick)
	}
	up := upstream.New(httpClient)
	fseClient := fse.New(up, store)
	cupClient := cup.New(up, store)
	ag := agenda.New(cupClient, fseClient, lookupConcurrency)
	googleCfg, err := google.ConfigFromFile(cfg.GoogleClientFile, "http://"+ln.Addr().String()+google.CallbackPath)
	if errors.Is(err, fs.ErrNotExist) {
		slog.WarnContext(ctx, "google not configured: client file missing")
	} else if err != nil {
		return fmt.Errorf("google config: %w", err)
	}
	googleClient := google.New(googleCfg, vault.NewTokenVaultWithBackend(secrets, keyringService, keyringGoogleUser), httpClient)
	mux := http.NewServeMux()
	api := httpapi.New(store, flow, ag, documents{Client: fseClient, bookings: cupClient}, googleClient, cfg.LoginMode)
	for _, pattern := range []string{"/api/", "/openapi.json", "/openapi.yaml", "/docs", "/schemas/"} {
		mux.Handle(pattern, api)
	}
	mux.Handle(google.CallbackPath, googleClient.CallbackHandler())
	mux.Handle("/", webui.Handler())
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	slog.InfoContext(ctx, "fts listening", slog.String("addr", ln.Addr().String()))

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	slog.InfoContext(ctx, "fts stopped")
	return nil
}

func secretBackend(ctx context.Context, cfg config.Config) (vault.Backend, error) {
	if cfg.SecretKeyFile == "" {
		slog.InfoContext(ctx, "secrets in system keyring")
		return vault.KeyringBackend{}, nil
	}
	key, err := vault.LoadKey(cfg.SecretKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load secret key: %w", err)
	}
	backend, err := vault.NewFileBackend(filepath.Join(cfg.DataDir, secretsFile), key)
	if err != nil {
		return nil, fmt.Errorf("open secrets file: %w", err)
	}
	slog.InfoContext(ctx, "secrets in encrypted file", slog.String("dir", cfg.DataDir))
	return backend, nil
}
