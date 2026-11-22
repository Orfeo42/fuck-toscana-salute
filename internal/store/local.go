package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
)

const (
	sourceLocalReport   = "local_report"
	sourceReportLink    = "report_link"
	sourceVisitOverride = "visit_override"
)

const createLocalReportsTable = `
CREATE TABLE IF NOT EXISTS local_reports (
	id TEXT PRIMARY KEY,
	meta_enc BYTEA NOT NULL,
	data_enc BYTEA NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
)`

const createReportLinksTable = `
CREATE TABLE IF NOT EXISTS report_links (
	visit_mac TEXT NOT NULL,
	report_mac TEXT NOT NULL,
	payload_enc BYTEA NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (visit_mac, report_mac)
)`

const createReportLinksIndex = `
CREATE INDEX IF NOT EXISTS idx_report_links_report ON report_links (report_mac)`

const createVisitOverridesTable = `
CREATE TABLE IF NOT EXISTS visit_overrides (
	visit_mac TEXT PRIMARY KEY,
	payload_enc BYTEA NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
)`

const insertLocalReportQuery = `
INSERT INTO local_reports (id, meta_enc, data_enc, created_at)
VALUES (:id, :meta_enc, :data_enc, :created_at)`

const insertLinkQuery = `
INSERT INTO report_links (visit_mac, report_mac, payload_enc, created_at)
VALUES (:visit_mac, :report_mac, :payload_enc, :created_at)
ON CONFLICT DO NOTHING`

const upsertOverrideQuery = `
INSERT INTO visit_overrides (visit_mac, payload_enc, updated_at)
VALUES (:visit_mac, :payload_enc, :updated_at)
ON CONFLICT (visit_mac) DO UPDATE
SET payload_enc = EXCLUDED.payload_enc, updated_at = EXCLUDED.updated_at`

const updateLocalReportMetaQuery = `
UPDATE local_reports SET meta_enc = :meta_enc WHERE id = :id`

const deleteLinksByReportQuery = `
DELETE FROM report_links WHERE report_mac = :report_mac`

const deleteLocalReportQuery = `
DELETE FROM local_reports WHERE id = :id`

const localReportsQuery = `
SELECT id, meta_enc, created_at
FROM local_reports
ORDER BY created_at DESC, id`

const localReportDataQuery = `
SELECT id, meta_enc, data_enc, created_at
FROM local_reports
WHERE id = :id`

const deleteLinkQuery = `
DELETE FROM report_links WHERE visit_mac = :visit_mac AND report_mac = :report_mac`

const linksQuery = `
SELECT visit_mac, report_mac, payload_enc, created_at FROM report_links`

const deleteOverrideQuery = `
DELETE FROM visit_overrides WHERE visit_mac = :visit_mac`

const overridesQuery = `
SELECT visit_mac, payload_enc, updated_at FROM visit_overrides`

type LocalReport struct {
	ID        string
	Meta      []byte
	Data      []byte
	CreatedAt time.Time
}

type LocalReportMeta struct {
	ID        string
	Meta      []byte
	CreatedAt time.Time
}

type Link struct {
	NRE       string
	ReportKey string
}

type Override struct {
	NRE       string
	Status    int
	UpdatedAt time.Time
}

type localReportRow struct {
	ID        string    `db:"id"`
	MetaEnc   []byte    `db:"meta_enc"`
	DataEnc   []byte    `db:"data_enc"`
	CreatedAt time.Time `db:"created_at"`
}

type localReportIDRow struct {
	ID string `db:"id"`
}

type localReportMetaUpdate struct {
	ID      string `db:"id"`
	MetaEnc []byte `db:"meta_enc"`
}

type linkRow struct {
	VisitMAC   string    `db:"visit_mac"`
	ReportMAC  string    `db:"report_mac"`
	PayloadEnc []byte    `db:"payload_enc"`
	CreatedAt  time.Time `db:"created_at"`
}

type linkKeyRow struct {
	VisitMAC  string `db:"visit_mac"`
	ReportMAC string `db:"report_mac"`
}

type reportMACRow struct {
	ReportMAC string `db:"report_mac"`
}

type linkPayload struct {
	NRE       string `json:"nre"`
	ReportKey string `json:"reportKey"`
}

type overrideRow struct {
	VisitMAC   string    `db:"visit_mac"`
	PayloadEnc []byte    `db:"payload_enc"`
	UpdatedAt  time.Time `db:"updated_at"`
}

type overridePayload struct {
	NRE    string `json:"nre"`
	Status int    `json:"status"`
}

func (s *Store) createLocalTables(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, createLocalReportsTable); err != nil {
		return fmt.Errorf("store: create local reports table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, createReportLinksTable); err != nil {
		return fmt.Errorf("store: create report links table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, createReportLinksIndex); err != nil {
		return fmt.Errorf("store: create report links index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, createVisitOverridesTable); err != nil {
		return fmt.Errorf("store: create visit overrides table: %w", err)
	}
	return nil
}

func (s *Store) sealLink(l Link) (linkRow, error) {
	visitMAC := s.mac(l.NRE)
	reportMAC := s.mac(l.ReportKey)
	plain, err := json.Marshal(linkPayload(l))
	if err != nil {
		return linkRow{}, fmt.Errorf("store: marshal link: %w", err)
	}
	enc, err := s.seal(plain, sourceReportLink, visitMAC+":"+reportMAC)
	if err != nil {
		return linkRow{}, err
	}
	return linkRow{VisitMAC: visitMAC, ReportMAC: reportMAC, PayloadEnc: enc, CreatedAt: time.Now().UTC()}, nil
}

func (s *Store) openLink(r linkRow) (Link, error) {
	plain, err := s.open(r.PayloadEnc, sourceReportLink, r.VisitMAC+":"+r.ReportMAC)
	if err != nil {
		return Link{}, err
	}
	var p linkPayload
	if err := json.Unmarshal(plain, &p); err != nil {
		return Link{}, fmt.Errorf("%w: %w", ErrDecrypt, err)
	}
	return Link(p), nil
}

func (s *Store) sealOverride(o Override) (overrideRow, error) {
	visitMAC := s.mac(o.NRE)
	plain, err := json.Marshal(overridePayload{NRE: o.NRE, Status: o.Status})
	if err != nil {
		return overrideRow{}, fmt.Errorf("store: marshal override: %w", err)
	}
	enc, err := s.seal(plain, sourceVisitOverride, visitMAC)
	if err != nil {
		return overrideRow{}, err
	}
	return overrideRow{VisitMAC: visitMAC, PayloadEnc: enc, UpdatedAt: o.UpdatedAt.UTC()}, nil
}

func (s *Store) openOverride(r overrideRow) (Override, error) {
	plain, err := s.open(r.PayloadEnc, sourceVisitOverride, r.VisitMAC)
	if err != nil {
		return Override{}, err
	}
	var p overridePayload
	if err := json.Unmarshal(plain, &p); err != nil {
		return Override{}, fmt.Errorf("%w: %w", ErrDecrypt, err)
	}
	return Override{NRE: p.NRE, Status: p.Status, UpdatedAt: r.UpdatedAt.UTC()}, nil
}

func rollback(ctx context.Context, tx *sqlx.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		slog.WarnContext(ctx, "store: rollback", slog.String("error", err.Error()))
	}
}

func closeRows(ctx context.Context, rows *sqlx.Rows) {
	if err := rows.Close(); err != nil {
		slog.WarnContext(ctx, "store: close rows", slog.String("error", err.Error()))
	}
}

func (s *Store) InsertLocalReport(ctx context.Context, r LocalReport, links []Link, override *Override) error {
	metaEnc, err := s.seal(r.Meta, sourceLocalReport, r.ID)
	if err != nil {
		return err
	}
	dataEnc, err := s.seal(r.Data, sourceLocalReport, r.ID)
	if err != nil {
		return err
	}
	linkRows := make([]linkRow, 0, len(links))
	for _, l := range links {
		lr, err := s.sealLink(l)
		if err != nil {
			return err
		}
		linkRows = append(linkRows, lr)
	}
	var ovRow overrideRow
	if override != nil {
		ovRow, err = s.sealOverride(*override)
		if err != nil {
			return err
		}
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: insert local report begin: %w", err)
	}
	defer rollback(ctx, tx)

	row := localReportRow{ID: r.ID, MetaEnc: metaEnc, DataEnc: dataEnc, CreatedAt: r.CreatedAt.UTC()}
	if _, err := tx.NamedExecContext(ctx, insertLocalReportQuery, row); err != nil {
		return fmt.Errorf("store: insert local report: %w", err)
	}
	for _, lr := range linkRows {
		if _, err := tx.NamedExecContext(ctx, insertLinkQuery, lr); err != nil {
			return fmt.Errorf("store: insert local report link: %w", err)
		}
	}
	if override != nil {
		if _, err := tx.NamedExecContext(ctx, upsertOverrideQuery, ovRow); err != nil {
			return fmt.Errorf("store: insert local report override: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: insert local report commit: %w", err)
	}
	return nil
}

func (s *Store) UpdateLocalReportMeta(ctx context.Context, id string, meta []byte) error {
	metaEnc, err := s.seal(meta, sourceLocalReport, id)
	if err != nil {
		return err
	}
	res, err := sqlx.NamedExecContext(ctx, s.db, updateLocalReportMetaQuery, localReportMetaUpdate{ID: id, MetaEnc: metaEnc})
	if err != nil {
		return fmt.Errorf("store: update local report meta: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: update local report meta: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteLocalReport(ctx context.Context, id, reportKey string) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: delete local report begin: %w", err)
	}
	defer rollback(ctx, tx)

	if _, err := tx.NamedExecContext(ctx, deleteLinksByReportQuery, reportMACRow{ReportMAC: s.mac(reportKey)}); err != nil {
		return fmt.Errorf("store: delete local report links: %w", err)
	}
	res, err := tx.NamedExecContext(ctx, deleteLocalReportQuery, localReportIDRow{ID: id})
	if err != nil {
		return fmt.Errorf("store: delete local report: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: delete local report: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: delete local report commit: %w", err)
	}
	return nil
}

func (s *Store) LocalReports(ctx context.Context) ([]LocalReportMeta, error) {
	rows, err := s.db.QueryxContext(ctx, localReportsQuery)
	if err != nil {
		return nil, fmt.Errorf("store: local reports: %w", err)
	}
	defer closeRows(ctx, rows)
	var out []LocalReportMeta
	for rows.Next() {
		var r localReportRow
		if err := rows.StructScan(&r); err != nil {
			return nil, fmt.Errorf("store: local reports scan: %w", err)
		}
		meta, err := s.open(r.MetaEnc, sourceLocalReport, r.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, LocalReportMeta{ID: r.ID, Meta: meta, CreatedAt: r.CreatedAt.UTC()})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: local reports: %w", err)
	}
	return out, nil
}

func (s *Store) LocalReportData(ctx context.Context, id string) (LocalReport, error) {
	rows, err := sqlx.NamedQueryContext(ctx, s.db, localReportDataQuery, localReportIDRow{ID: id})
	if err != nil {
		return LocalReport{}, fmt.Errorf("store: local report data: %w", err)
	}
	defer closeRows(ctx, rows)
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return LocalReport{}, fmt.Errorf("store: local report data: %w", err)
		}
		return LocalReport{}, ErrNotFound
	}
	var r localReportRow
	if err := rows.StructScan(&r); err != nil {
		return LocalReport{}, fmt.Errorf("store: local report data scan: %w", err)
	}
	meta, err := s.open(r.MetaEnc, sourceLocalReport, r.ID)
	if err != nil {
		return LocalReport{}, err
	}
	data, err := s.open(r.DataEnc, sourceLocalReport, r.ID)
	if err != nil {
		return LocalReport{}, err
	}
	return LocalReport{ID: r.ID, Meta: meta, Data: data, CreatedAt: r.CreatedAt.UTC()}, nil
}

func (s *Store) PutLinkWithOverride(ctx context.Context, l Link, o Override) error {
	linkRow, err := s.sealLink(l)
	if err != nil {
		return err
	}
	ovRow, err := s.sealOverride(o)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: put link with override begin: %w", err)
	}
	defer rollback(ctx, tx)

	if _, err := tx.NamedExecContext(ctx, insertLinkQuery, linkRow); err != nil {
		return fmt.Errorf("store: put link: %w", err)
	}
	if _, err := tx.NamedExecContext(ctx, upsertOverrideQuery, ovRow); err != nil {
		return fmt.Errorf("store: put override: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: put link with override commit: %w", err)
	}
	return nil
}

func (s *Store) DeleteLink(ctx context.Context, l Link) error {
	key := linkKeyRow{VisitMAC: s.mac(l.NRE), ReportMAC: s.mac(l.ReportKey)}
	if _, err := sqlx.NamedExecContext(ctx, s.db, deleteLinkQuery, key); err != nil {
		return fmt.Errorf("store: delete link: %w", err)
	}
	return nil
}

func (s *Store) Links(ctx context.Context) ([]Link, error) {
	rows, err := s.db.QueryxContext(ctx, linksQuery)
	if err != nil {
		return nil, fmt.Errorf("store: links: %w", err)
	}
	defer closeRows(ctx, rows)
	var out []Link
	for rows.Next() {
		var r linkRow
		if err := rows.StructScan(&r); err != nil {
			return nil, fmt.Errorf("store: links scan: %w", err)
		}
		l, err := s.openLink(r)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: links: %w", err)
	}
	return out, nil
}

func (s *Store) PutOverride(ctx context.Context, o Override) error {
	row, err := s.sealOverride(o)
	if err != nil {
		return err
	}
	if _, err := sqlx.NamedExecContext(ctx, s.db, upsertOverrideQuery, row); err != nil {
		return fmt.Errorf("store: put override: %w", err)
	}
	return nil
}

func (s *Store) DeleteOverride(ctx context.Context, nre string) error {
	key := struct {
		VisitMAC string `db:"visit_mac"`
	}{VisitMAC: s.mac(nre)}
	if _, err := sqlx.NamedExecContext(ctx, s.db, deleteOverrideQuery, key); err != nil {
		return fmt.Errorf("store: delete override: %w", err)
	}
	return nil
}

func (s *Store) Overrides(ctx context.Context) ([]Override, error) {
	rows, err := s.db.QueryxContext(ctx, overridesQuery)
	if err != nil {
		return nil, fmt.Errorf("store: overrides: %w", err)
	}
	defer closeRows(ctx, rows)
	var out []Override
	for rows.Next() {
		var r overrideRow
		if err := rows.StructScan(&r); err != nil {
			return nil, fmt.Errorf("store: overrides scan: %w", err)
		}
		o, err := s.openOverride(r)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: overrides: %w", err)
	}
	return out, nil
}
