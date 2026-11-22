package facilities

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
)

const maxAge = 30 * 24 * time.Hour

type Service struct {
	db             *sqlx.DB
	client         *http.Client
	ministryClient *http.Client
	ministryBase   string
	ckanURL        string
}

func New(ctx context.Context, db *sqlx.DB, client *http.Client) (*Service, error) {
	s := &Service{db: db, client: client, ministryClient: newMinistryClient(client.Timeout), ministryBase: defaultMinistryBase, ckanURL: defaultCKANURL}
	if err := s.createTables(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Service) Stale(ctx context.Context) (bool, error) {
	at, err := s.refreshedAt(ctx)
	if err != nil {
		return false, err
	}
	for _, source := range []string{SourceMinistero, SourceRegione} {
		last, ok := at[source]
		if !ok || time.Since(last) > maxAge {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) Refresh(ctx context.Context) error {
	ministry, ministryErr := s.loadMinistry(ctx)
	if ministryErr == nil {
		ministryErr = s.replaceSource(ctx, SourceMinistero, ministry, time.Now())
	}
	if ministryErr != nil {
		ministryErr = fmt.Errorf("%s: %w", SourceMinistero, ministryErr)
	}
	regione, regioneErr := s.loadRegione(ctx)
	if regioneErr == nil {
		regioneErr = s.replaceSource(ctx, SourceRegione, regione, time.Now())
	}
	if regioneErr != nil {
		regioneErr = fmt.Errorf("%s: %w", SourceRegione, regioneErr)
	}
	if err := errors.Join(ministryErr, regioneErr); err != nil {
		return err
	}
	slog.InfoContext(ctx, "facilities refreshed", slog.Int("ministero", len(ministry)), slog.Int("regione", len(regione)))
	return nil
}

func (s *Service) refreshIfStale(ctx context.Context) {
	stale, err := s.Stale(ctx)
	if err != nil {
		slog.WarnContext(ctx, "facilities staleness check failed", slog.String("error", err.Error()))
		return
	}
	if !stale {
		return
	}
	if err := s.Refresh(ctx); err != nil {
		slog.WarnContext(ctx, "facilities refresh failed", slog.String("error", err.Error()))
	}
}

func (s *Service) Run(ctx context.Context, tick time.Duration) {
	s.refreshIfStale(ctx)
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refreshIfStale(ctx)
		}
	}
}
