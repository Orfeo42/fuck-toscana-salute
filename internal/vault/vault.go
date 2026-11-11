package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"fuck-toscana-salute/internal/session"
)

const wireVersion = 1

type vaultError string

func (e vaultError) Error() string { return string(e) }

const ErrCorrupt = vaultError("vault: stored snapshot is corrupt")

type wireTokens struct {
	Access           string    `json:"access"`
	Refresh          string    `json:"refresh"`
	AccessExpiresAt  time.Time `json:"accessExpiresAt"`
	RefreshExpiresAt time.Time `json:"refreshExpiresAt"`
	GestioneVerified bool      `json:"gestioneVerified"`
	StartedAt        time.Time `json:"startedAt"`
}

type wireIdentity struct {
	CodiceFiscale    string `json:"codiceFiscale"`
	TesseraSanitaria string `json:"tesseraSanitaria"`
}

type wireSnapshot struct {
	Version  int                   `json:"version"`
	Tokens   map[string]wireTokens `json:"tokens"`
	Identity wireIdentity          `json:"identity"`
}

type Vault struct {
	backend Backend
	service string
	user    string
}

var _ session.Persister = (*Vault)(nil)

func New(service, user string) *Vault {
	return NewWithBackend(KeyringBackend{}, service, user)
}

func NewWithBackend(b Backend, service, user string) *Vault {
	return &Vault{backend: b, service: service, user: user}
}

func (v *Vault) Save(ctx context.Context, snapshot session.Snapshot) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("vault: save: %w", err)
	}

	if len(snapshot.Tokens) == 0 && snapshot.Identity == (session.Identity{}) {
		err := v.backend.Delete(v.service, v.user)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("vault: save: %w", err)
		}
		return nil
	}

	wire := wireSnapshot{
		Version: wireVersion,
		Tokens:  make(map[string]wireTokens, len(snapshot.Tokens)),
		Identity: wireIdentity{
			CodiceFiscale:    snapshot.Identity.CodiceFiscale,
			TesseraSanitaria: snapshot.Identity.TesseraSanitaria,
		},
	}
	for portal, t := range snapshot.Tokens {
		wire.Tokens[string(portal)] = wireTokens{
			Access:           t.Access,
			Refresh:          t.Refresh,
			AccessExpiresAt:  t.AccessExpiresAt,
			RefreshExpiresAt: t.RefreshExpiresAt,
			GestioneVerified: t.GestioneVerified,
			StartedAt:        t.StartedAt,
		}
	}

	data, err := json.Marshal(wire)
	if err != nil {
		return fmt.Errorf("vault: save: marshal failed")
	}
	if err := v.backend.Set(v.service, v.user, string(data)); err != nil {
		return fmt.Errorf("vault: save: %w", err)
	}
	return nil
}

func (v *Vault) Load(ctx context.Context) (session.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return session.Snapshot{}, fmt.Errorf("vault: load: %w", err)
	}

	raw, err := v.backend.Get(v.service, v.user)
	if errors.Is(err, ErrNotFound) {
		return session.Snapshot{}, session.ErrNoSnapshot
	}
	if err != nil {
		return session.Snapshot{}, fmt.Errorf("vault: load: %w", err)
	}

	var wire wireSnapshot
	if err := json.Unmarshal([]byte(raw), &wire); err != nil {
		return session.Snapshot{}, fmt.Errorf("vault: load: %w", ErrCorrupt)
	}
	if wire.Version != wireVersion {
		return session.Snapshot{}, fmt.Errorf("vault: load: %w", ErrCorrupt)
	}

	snapshot := session.Snapshot{
		Tokens: make(map[session.Portal]session.Tokens, len(wire.Tokens)),
		Identity: session.Identity{
			CodiceFiscale:    wire.Identity.CodiceFiscale,
			TesseraSanitaria: wire.Identity.TesseraSanitaria,
		},
	}
	for key, t := range wire.Tokens {
		portal := session.Portal(key)
		if portal != session.PortalCUP && portal != session.PortalFSE {
			continue
		}
		snapshot.Tokens[portal] = session.Tokens{
			Access:           t.Access,
			Refresh:          t.Refresh,
			AccessExpiresAt:  t.AccessExpiresAt,
			RefreshExpiresAt: t.RefreshExpiresAt,
			GestioneVerified: t.GestioneVerified,
			StartedAt:        t.StartedAt,
		}
	}
	return snapshot, nil
}
