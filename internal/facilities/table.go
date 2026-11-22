package facilities

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"fuck-toscana-salute/internal/textutil"
)

const (
	searchLimit  = 20
	insertChunk  = 5000
	likeEscaper  = `\`
	searchSelect = `SELECT source, code, name, comune, provincia, address FROM facilities WHERE `
)

const createFacilitiesTable = `
CREATE TABLE IF NOT EXISTS facilities (
	source TEXT NOT NULL,
	code TEXT NOT NULL,
	name TEXT NOT NULL,
	comune TEXT NOT NULL,
	provincia TEXT NOT NULL,
	address TEXT NOT NULL,
	search_key TEXT NOT NULL,
	PRIMARY KEY (source, code)
)`

const createMetaTable = `
CREATE TABLE IF NOT EXISTS facilities_meta (
	source TEXT PRIMARY KEY,
	refreshed_at TIMESTAMPTZ NOT NULL
)`

const deleteSourceQuery = `DELETE FROM facilities WHERE source = :source`

const insertQuery = `
INSERT INTO facilities (source, code, name, comune, provincia, address, search_key)
VALUES (:source, :code, :name, :comune, :provincia, :address, :search_key)`

const upsertMetaQuery = `
INSERT INTO facilities_meta (source, refreshed_at)
VALUES (:source, :refreshed_at)
ON CONFLICT (source) DO UPDATE SET refreshed_at = EXCLUDED.refreshed_at`

const metaQuery = `SELECT source, refreshed_at FROM facilities_meta`

type row struct {
	Source    string `db:"source"`
	Code      string `db:"code"`
	Name      string `db:"name"`
	Comune    string `db:"comune"`
	Provincia string `db:"provincia"`
	Address   string `db:"address"`
	SearchKey string `db:"search_key"`
}

type sourceArg struct {
	Source string `db:"source"`
}

type metaRow struct {
	Source      string    `db:"source"`
	RefreshedAt time.Time `db:"refreshed_at"`
}

func (s *Service) createTables(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, createFacilitiesTable); err != nil {
		return fmt.Errorf("facilities: create table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, createMetaTable); err != nil {
		return fmt.Errorf("facilities: create meta table: %w", err)
	}
	return nil
}

func toRows(source string, list []Facility) []row {
	rows := make([]row, 0, len(list))
	for _, f := range list {
		rows = append(rows, row{
			Source:    source,
			Code:      f.Code,
			Name:      f.Name,
			Comune:    f.Comune,
			Provincia: f.Provincia,
			Address:   f.Address,
			SearchKey: textutil.Fold(f.Name + " " + f.Comune),
		})
	}
	return rows
}

func (s *Service) replaceSource(ctx context.Context, source string, list []Facility, at time.Time) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("facilities: begin: %w", err)
	}
	if err := writeSource(ctx, tx, source, toRows(source, list), at); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("facilities: commit: %w", err)
	}
	return nil
}

func writeSource(ctx context.Context, tx *sqlx.Tx, source string, rows []row, at time.Time) error {
	if _, err := sqlx.NamedExecContext(ctx, tx, deleteSourceQuery, sourceArg{Source: source}); err != nil {
		return fmt.Errorf("facilities: delete: %w", err)
	}
	for start := 0; start < len(rows); start += insertChunk {
		chunk := rows[start:min(start+insertChunk, len(rows))]
		if _, err := sqlx.NamedExecContext(ctx, tx, insertQuery, chunk); err != nil {
			return fmt.Errorf("facilities: insert: %w", err)
		}
	}
	if _, err := sqlx.NamedExecContext(ctx, tx, upsertMetaQuery, metaRow{Source: source, RefreshedAt: at}); err != nil {
		return fmt.Errorf("facilities: update meta: %w", err)
	}
	return nil
}

func (s *Service) refreshedAt(ctx context.Context) (map[string]time.Time, error) {
	var rows []metaRow
	if err := s.db.SelectContext(ctx, &rows, metaQuery); err != nil {
		return nil, fmt.Errorf("facilities: read meta: %w", err)
	}
	out := make(map[string]time.Time, len(rows))
	for _, r := range rows {
		out[r.Source] = r.RefreshedAt
	}
	return out, nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (s *Service) Search(ctx context.Context, query string) ([]Facility, error) {
	tokens := strings.Fields(textutil.Fold(query))
	if len(tokens) == 0 {
		return []Facility{}, nil
	}
	params := map[string]any{
		"prefix": escapeLike(tokens[0]) + "%",
		"limit":  searchLimit,
	}
	conds := make([]string, 0, len(tokens))
	for i, tok := range tokens {
		name := fmt.Sprintf("t%d", i)
		params[name] = "%" + escapeLike(tok) + "%"
		conds = append(conds, "search_key LIKE :"+name+" ESCAPE '"+likeEscaper+"'")
	}
	q := searchSelect + strings.Join(conds, " AND ") + `
ORDER BY (source <> 'ministero'),
	(search_key NOT LIKE :prefix ESCAPE '` + likeEscaper + `'),
	name
LIMIT :limit`
	rows, err := sqlx.NamedQueryContext(ctx, s.db, q, params)
	if err != nil {
		return nil, fmt.Errorf("facilities: search: %w", err)
	}
	out := []Facility{}
	for rows.Next() {
		var f Facility
		if err := rows.StructScan(&f); err != nil {
			return nil, errors.Join(fmt.Errorf("facilities: search scan: %w", err), rows.Close())
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("facilities: search rows: %w", err), rows.Close())
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("facilities: search close: %w", err)
	}
	return out, nil
}
