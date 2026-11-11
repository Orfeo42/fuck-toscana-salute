package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"fuck-toscana-salute/internal/vault"
)

const (
	secretAAD     = "secret"
	secretTimeout = 10 * time.Second
)

const createSecretsTable = `
CREATE TABLE IF NOT EXISTS secrets (
	key_mac TEXT PRIMARY KEY,
	value_enc BYTEA NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
)`

const upsertSecretQuery = `
INSERT INTO secrets (key_mac, value_enc, updated_at)
VALUES (:key_mac, :value_enc, :updated_at)
ON CONFLICT (key_mac) DO UPDATE
SET value_enc = EXCLUDED.value_enc, updated_at = EXCLUDED.updated_at`

const getSecretQuery = `
SELECT key_mac, value_enc, updated_at
FROM secrets
WHERE key_mac = :key_mac`

const deleteSecretQuery = `DELETE FROM secrets WHERE key_mac = :key_mac`

const anySecretQuery = `SELECT key_mac, value_enc, updated_at FROM secrets LIMIT 1`

type secretRow struct {
	KeyMAC    string    `db:"key_mac"`
	ValueEnc  []byte    `db:"value_enc"`
	UpdatedAt time.Time `db:"updated_at"`
}

type secretKey struct {
	KeyMAC string `db:"key_mac"`
}

var _ vault.Backend = (*Store)(nil)

func (s *Store) secretMAC(service, user string) string {
	return s.mac(service + "\x00" + user)
}

func (s *Store) Get(service, user string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), secretTimeout)
	defer cancel()
	rows, err := sqlx.NamedQueryContext(ctx, s.db, getSecretQuery, secretKey{KeyMAC: s.secretMAC(service, user)})
	if err != nil {
		return "", fmt.Errorf("store: get secret: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", fmt.Errorf("store: get secret: %w", err)
		}
		return "", vault.ErrNotFound
	}
	var r secretRow
	if err := rows.StructScan(&r); err != nil {
		return "", fmt.Errorf("store: get secret scan: %w", err)
	}
	plain, err := s.open(r.ValueEnc, secretAAD, r.KeyMAC)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (s *Store) Set(service, user, value string) error {
	ctx, cancel := context.WithTimeout(context.Background(), secretTimeout)
	defer cancel()
	keyMAC := s.secretMAC(service, user)
	enc, err := s.seal([]byte(value), secretAAD, keyMAC)
	if err != nil {
		return err
	}
	r := secretRow{KeyMAC: keyMAC, ValueEnc: enc, UpdatedAt: time.Now().UTC()}
	if _, err := sqlx.NamedExecContext(ctx, s.db, upsertSecretQuery, r); err != nil {
		return fmt.Errorf("store: set secret: %w", err)
	}
	return nil
}

func (s *Store) Delete(service, user string) error {
	ctx, cancel := context.WithTimeout(context.Background(), secretTimeout)
	defer cancel()
	if _, err := sqlx.NamedExecContext(ctx, s.db, deleteSecretQuery, secretKey{KeyMAC: s.secretMAC(service, user)}); err != nil {
		return fmt.Errorf("store: delete secret: %w", err)
	}
	return nil
}
