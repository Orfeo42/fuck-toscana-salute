package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/config"
	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/httpapi"
	"fuck-toscana-salute/internal/keycloak"
	"fuck-toscana-salute/internal/login"
	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
	"fuck-toscana-salute/internal/vault"
)

const (
	lookupConcurrency = 4
	superviseTick     = time.Minute
	keyringService    = "fuck-toscana-salute"
	keyringUser       = "session"
)

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

	httpClient := &http.Client{Timeout: 30 * time.Second}
	store := session.NewStore(keycloak.New(httpClient, keycloak.TokenURL), vault.New(keyringService, keyringUser), cfg.RefreshMargin)
	if err := store.Restore(ctx); err != nil {
		slog.WarnContext(ctx, "session restore failed", slog.String("error", err.Error()))
	}
	go store.Run(ctx, cfg.RefreshTick)

	flow := loginflow.New(login.New(cfg.BrowserChannel, cfg.ProfileDir, cfg.LoginTimeout), store)
	go flow.Supervise(ctx, store, superviseTick)
	up := upstream.New(httpClient)
	fseClient := fse.New(up, store)
	ag := agenda.New(cup.New(up, store), fseClient, lookupConcurrency)
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.New(store, flow, ag, fseClient),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
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
