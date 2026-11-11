package session

import (
	"context"
	"time"
)

type Portal string

const (
	PortalCUP Portal = "cup"
	PortalFSE Portal = "fse"
)

type Tokens struct {
	Access           string
	Refresh          string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
	GestioneVerified bool
	StartedAt        time.Time
}

type Identity struct {
	CodiceFiscale    string
	TesseraSanitaria string
}

type State string

const (
	StateLoggedOut State = "logged_out"
	StateActive    State = "active"
	StateExpired   State = "expired"
)

type PortalStatus struct {
	Portal           Portal
	State            State
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

type Status struct {
	Portals       []PortalStatus
	GestioneReady bool
}

type sessionError string

func (e sessionError) Error() string { return string(e) }

const (
	ErrNotLoggedIn     = sessionError("session: not logged in")
	ErrSessionExpired  = sessionError("session: expired")
	ErrRefreshRejected = sessionError("session: refresh rejected by identity provider")
	ErrNoSnapshot      = sessionError("session: no saved snapshot")
)

type Snapshot struct {
	Tokens   map[Portal]Tokens
	Identity Identity
}

//go:generate moq -out persister_mock.go . Persister:MockPersister
type Persister interface {
	Save(ctx context.Context, snapshot Snapshot) error
	Load(ctx context.Context) (Snapshot, error)
}

//go:generate moq -out refresher_mock.go . Refresher:MockRefresher
type Refresher interface {
	Refresh(ctx context.Context, portal Portal, refreshToken string) (Tokens, error)
}
