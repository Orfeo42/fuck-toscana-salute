package store

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type storeError string

func (e storeError) Error() string { return string(e) }

const (
	ErrNotFound storeError = "store: record not found"
	ErrKey      storeError = "store: key must be 32 bytes"
	ErrDecrypt  storeError = "store: cannot decrypt row"
	ErrConnect  storeError = "store: cannot connect"
)

const createTable = `
CREATE TABLE IF NOT EXISTS responses (
	id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	source TEXT NOT NULL,
	subject_mac TEXT NOT NULL,
	subject_enc BYTEA NOT NULL,
	fetched_at TIMESTAMPTZ NOT NULL,
	status INTEGER NOT NULL,
	payload_enc BYTEA NOT NULL
)`

const createIndex = `
CREATE INDEX IF NOT EXISTS idx_responses_lookup
ON responses (source, subject_mac, fetched_at DESC)`

const insertQuery = `
INSERT INTO responses (source, subject_mac, subject_enc, fetched_at, status, payload_enc)
VALUES (:source, :subject_mac, :subject_enc, :fetched_at, :status, :payload_enc)`

const latestQuery = `
SELECT id, source, subject_mac, subject_enc, fetched_at, status, payload_enc
FROM responses
WHERE source = :source AND subject_mac = :subject_mac
ORDER BY fetched_at DESC, id DESC
LIMIT 1`

const latestBySourceQuery = `
SELECT DISTINCT ON (subject_mac) id, source, subject_mac, subject_enc, fetched_at, status, payload_enc
FROM responses
WHERE source = :source
ORDER BY subject_mac, fetched_at DESC, id DESC`

const anyRowQuery = `
SELECT id, source, subject_mac, subject_enc, fetched_at, status, payload_enc
FROM responses
LIMIT 1`

type Record struct {
	Source    string
	Subject   string
	FetchedAt time.Time
	Status    int
	Payload   []byte
}

type row struct {
	ID         int64     `db:"id"`
	Source     string    `db:"source"`
	SubjectMAC string    `db:"subject_mac"`
	SubjectEnc []byte    `db:"subject_enc"`
	FetchedAt  time.Time `db:"fetched_at"`
	Status     int       `db:"status"`
	PayloadEnc []byte    `db:"payload_enc"`
}

type lookup struct {
	Source     string `db:"source"`
	SubjectMAC string `db:"subject_mac"`
}

type Store struct {
	db     *sqlx.DB
	aead   cipher.AEAD
	macKey []byte
}

func Open(ctx context.Context, dsn string, key []byte) (*Store, error) {
	if len(key) != 32 {
		return nil, ErrKey
	}
	encKey, err := hkdf.Key(sha256.New, key, nil, "enc", 32)
	if err != nil {
		return nil, fmt.Errorf("store: derive enc key: %w", err)
	}
	macKey, err := hkdf.Key(sha256.New, key, nil, "mac", 32)
	if err != nil {
		return nil, fmt.Errorf("store: derive mac key: %w", err)
	}
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("store: cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("store: gcm: %w", err)
	}

	db, err := sqlx.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnect, err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		return nil, errors.Join(fmt.Errorf("%w: %w", ErrConnect, err), db.Close())
	}
	s := &Store{db: db, aead: aead, macKey: macKey}
	if err := s.init(ctx); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	slog.InfoContext(ctx, "store opened")
	return s, nil
}

func (s *Store) init(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, createTable); err != nil {
		return fmt.Errorf("store: create table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, createIndex); err != nil {
		return fmt.Errorf("store: create index: %w", err)
	}
	var r row
	rows, err := s.db.QueryxContext(ctx, anyRowQuery)
	if err != nil {
		return fmt.Errorf("store: probe: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.WarnContext(ctx, "store: close rows", slog.String("error", err.Error()))
		}
	}()
	if !rows.Next() {
		return rows.Err()
	}
	if err := rows.StructScan(&r); err != nil {
		return fmt.Errorf("store: probe scan: %w", err)
	}
	if _, err := s.decode(r); err != nil {
		return err
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) mac(subject string) string {
	h := hmac.New(sha256.New, s.macKey)
	h.Write([]byte(subject))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Store) seal(plain []byte, source, subjectMAC string) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("store: nonce: %w", err)
	}
	return s.aead.Seal(nonce, nonce, plain, aad(source, subjectMAC)), nil
}

func (s *Store) open(blob []byte, source, subjectMAC string) ([]byte, error) {
	n := s.aead.NonceSize()
	if len(blob) < n {
		return nil, ErrDecrypt
	}
	plain, err := s.aead.Open(nil, blob[:n], blob[n:], aad(source, subjectMAC))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecrypt, err)
	}
	return plain, nil
}

func aad(source, subjectMAC string) []byte {
	return []byte(source + "\x00" + subjectMAC)
}

func (s *Store) decode(r row) (Record, error) {
	subject, err := s.open(r.SubjectEnc, r.Source, r.SubjectMAC)
	if err != nil {
		return Record{}, err
	}
	payload, err := s.open(r.PayloadEnc, r.Source, r.SubjectMAC)
	if err != nil {
		return Record{}, err
	}
	return Record{
		Source:    r.Source,
		Subject:   string(subject),
		FetchedAt: r.FetchedAt.UTC(),
		Status:    r.Status,
		Payload:   payload,
	}, nil
}

func (s *Store) Put(ctx context.Context, rec Record) error {
	subjectMAC := s.mac(rec.Subject)
	subjectEnc, err := s.seal([]byte(rec.Subject), rec.Source, subjectMAC)
	if err != nil {
		return err
	}
	payloadEnc, err := s.seal(rec.Payload, rec.Source, subjectMAC)
	if err != nil {
		return err
	}
	r := row{
		Source:     rec.Source,
		SubjectMAC: subjectMAC,
		SubjectEnc: subjectEnc,
		FetchedAt:  rec.FetchedAt.UTC(),
		Status:     rec.Status,
		PayloadEnc: payloadEnc,
	}
	if _, err := sqlx.NamedExecContext(ctx, s.db, insertQuery, r); err != nil {
		return fmt.Errorf("store: put: %w", err)
	}
	return nil
}

func (s *Store) Latest(ctx context.Context, source, subject string) (Record, error) {
	q := lookup{Source: source, SubjectMAC: s.mac(subject)}
	rows, err := sqlx.NamedQueryContext(ctx, s.db, latestQuery, q)
	if err != nil {
		return Record{}, fmt.Errorf("store: latest: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.WarnContext(ctx, "store: close rows", slog.String("error", err.Error()))
		}
	}()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return Record{}, fmt.Errorf("store: latest: %w", err)
		}
		return Record{}, ErrNotFound
	}
	var r row
	if err := rows.StructScan(&r); err != nil {
		return Record{}, fmt.Errorf("store: latest scan: %w", err)
	}
	return s.decode(r)
}

func (s *Store) LatestBySource(ctx context.Context, source string) ([]Record, error) {
	rows, err := sqlx.NamedQueryContext(ctx, s.db, latestBySourceQuery, lookup{Source: source})
	if err != nil {
		return nil, fmt.Errorf("store: latest by source: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.WarnContext(ctx, "store: close rows", slog.String("error", err.Error()))
		}
	}()
	var out []Record
	for rows.Next() {
		var r row
		if err := rows.StructScan(&r); err != nil {
			return nil, fmt.Errorf("store: latest by source scan: %w", err)
		}
		rec, err := s.decode(r)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: latest by source: %w", err)
	}
	return out, nil
}
