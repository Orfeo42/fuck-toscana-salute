package localrecords

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/store"
)

func (s *Service) Link(ctx context.Context, nre string, ref referti.Ref) error {
	kind, err := referti.ParseKind(string(ref.Kind))
	if err != nil || kind == referti.KindFSE {
		return ErrInvalidReportKind
	}
	if kind == referti.KindLocal {
		if _, err := s.findReport(ctx, ref.AbstractID); err != nil {
			return err
		}
	}
	normalized := normalizeNRE(nre)
	link := store.Link{NRE: normalized, ReportKey: ref.Key()}
	override := store.Override{NRE: normalized, Status: DoneStatus, UpdatedAt: s.now().UTC()}
	if err := s.store.PutLinkWithOverride(ctx, link, override); err != nil {
		return fmt.Errorf("localrecords: put link with override: %w", err)
	}
	slog.InfoContext(ctx, "report linked", slog.String("kind", string(kind)))
	return nil
}

func (s *Service) Unlink(ctx context.Context, nre string, ref referti.Ref) error {
	if err := s.store.DeleteLink(ctx, store.Link{NRE: normalizeNRE(nre), ReportKey: ref.Key()}); err != nil {
		return fmt.Errorf("localrecords: delete link: %w", err)
	}
	slog.InfoContext(ctx, "report unlinked", slog.String("kind", string(ref.Kind)))
	return nil
}

func (s *Service) Links(ctx context.Context) ([]Link, error) {
	rows, err := s.store.Links(ctx)
	if err != nil {
		return nil, fmt.Errorf("localrecords: list links: %w", err)
	}
	links := make([]Link, 0, len(rows))
	for _, row := range rows {
		ref, err := referti.ParseRefKey(row.ReportKey)
		if err != nil {
			slog.WarnContext(ctx, "skipping malformed link key")
			continue
		}
		links = append(links, Link{NRE: row.NRE, Ref: ref})
	}
	return links, nil
}

func (s *Service) SetStatus(ctx context.Context, nre string, status int) error {
	if !slices.Contains(cup.SelectableStatuses, cup.Status(status)) {
		return ErrInvalidStatus
	}
	o := store.Override{NRE: normalizeNRE(nre), Status: status, UpdatedAt: s.now().UTC()}
	if err := s.store.PutOverride(ctx, o); err != nil {
		return fmt.Errorf("localrecords: put override: %w", err)
	}
	slog.InfoContext(ctx, "status override set", slog.Int("status", status))
	return nil
}

func (s *Service) ResetStatus(ctx context.Context, nre string) error {
	if err := s.store.DeleteOverride(ctx, normalizeNRE(nre)); err != nil {
		return fmt.Errorf("localrecords: delete override: %w", err)
	}
	slog.InfoContext(ctx, "status override reset")
	return nil
}

func (s *Service) Overrides(ctx context.Context) ([]Override, error) {
	rows, err := s.store.Overrides(ctx)
	if err != nil {
		return nil, fmt.Errorf("localrecords: list overrides: %w", err)
	}
	overrides := make([]Override, 0, len(rows))
	for _, row := range rows {
		overrides = append(overrides, Override{NRE: row.NRE, Status: row.Status, UpdatedAt: row.UpdatedAt})
	}
	return overrides, nil
}
