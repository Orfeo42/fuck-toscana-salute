package httpapi

import (
	"context"

	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/session"
)

//go:generate moq -out deps_mock.go . SessionStore:MockSessionStore LoginFlow:MockLoginFlow
type SessionStore interface {
	Status() session.Status
	Clear()
}

type LoginFlow interface {
	Start(ctx context.Context, portal session.Portal) error
	States() map[session.Portal]loginflow.State
}
