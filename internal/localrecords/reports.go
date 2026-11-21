package localrecords

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"fuck-toscana-salute/internal/store"
)

func detectMime(data []byte) (string, error) {
	mime, _, _ := strings.Cut(http.DetectContentType(data), ";")
	switch mime {
	case "application/pdf", "image/jpeg", "image/png":
		return mime, nil
	}
	return "", ErrUnsupportedType
}

func (s *Service) Create(ctx context.Context, nr NewReport) (Report, error) {
	if len(nr.Data) == 0 {
		return Report{}, ErrEmptyFile
	}
	if len(nr.Data) > MaxFileBytes {
		return Report{}, ErrTooLarge
	}
	mime, err := detectMime(nr.Data)
	if err != nil {
		return Report{}, err
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return Report{}, fmt.Errorf("localrecords: generate id: %w", err)
	}
	now := s.now().UTC()
	report := Report{
		ID:        hex.EncodeToString(raw),
		Meta:      normalizeMeta(nr.Meta, nr.Filename),
		Filename:  nr.Filename,
		MimeType:  mime,
		Origin:    nr.Origin,
		CreatedAt: now,
	}
	rec, err := toStoreLocal(report, nr.Data)
	if err != nil {
		return Report{}, err
	}
	var links []store.Link
	var override *store.Override
	nre := normalizeNRE(nr.NRE)
	if nre != "" {
		links = []store.Link{{NRE: nre, ReportKey: LocalRef(report.ID).Key()}}
		override = &store.Override{NRE: nre, Status: DoneStatus, UpdatedAt: now}
	}
	if err := s.store.InsertLocalReport(ctx, rec, links, override); err != nil {
		return Report{}, fmt.Errorf("localrecords: insert report: %w", err)
	}
	slog.InfoContext(ctx, "local report created",
		slog.String("origin", string(report.Origin)),
		slog.String("mime", mime),
		slog.Int("size", len(nr.Data)),
		slog.Bool("linked", nre != ""),
	)
	return report, nil
}

func (s *Service) findReport(ctx context.Context, id string) (Report, error) {
	metas, err := s.store.LocalReports(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("localrecords: list reports: %w", err)
	}
	for _, m := range metas {
		if m.ID == id {
			return decodeReport(m.ID, m.Meta, m.CreatedAt)
		}
	}
	return Report{}, ErrNotFound
}

func (s *Service) Update(ctx context.Context, id string, meta Meta) error {
	current, err := s.findReport(ctx, id)
	if err != nil {
		return err
	}
	current.Meta = normalizeMeta(meta, current.Filename)
	blob, err := encodeMeta(current)
	if err != nil {
		return err
	}
	if err := s.store.UpdateLocalReportMeta(ctx, id, blob); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("localrecords: update report meta: %w", err)
	}
	slog.InfoContext(ctx, "local report updated")
	return nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.store.DeleteLocalReport(ctx, id, LocalRef(id).Key()); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("localrecords: delete report: %w", err)
	}
	slog.InfoContext(ctx, "local report deleted")
	return nil
}

func (s *Service) List(ctx context.Context) ([]Report, error) {
	metas, err := s.store.LocalReports(ctx)
	if err != nil {
		return nil, fmt.Errorf("localrecords: list reports: %w", err)
	}
	reports := make([]Report, 0, len(metas))
	for _, m := range metas {
		r, err := decodeReport(m.ID, m.Meta, m.CreatedAt)
		if err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}
	sort.SliceStable(reports, func(i, j int) bool {
		a, b := reports[i], reports[j]
		if a.Meta.Date.IsZero() != b.Meta.Date.IsZero() {
			return !a.Meta.Date.IsZero()
		}
		if !a.Meta.Date.Equal(b.Meta.Date) {
			return a.Meta.Date.After(b.Meta.Date)
		}
		return a.CreatedAt.After(b.CreatedAt)
	})
	return reports, nil
}

func (s *Service) File(ctx context.Context, id string) (File, error) {
	rec, err := s.store.LocalReportData(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return File{}, ErrNotFound
		}
		return File{}, fmt.Errorf("localrecords: read report: %w", err)
	}
	r, err := decodeReport(rec.ID, rec.Meta, rec.CreatedAt)
	if err != nil {
		return File{}, err
	}
	return File{Data: rec.Data, MimeType: r.MimeType, Filename: r.Filename}, nil
}
