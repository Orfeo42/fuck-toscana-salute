package httpapi

import (
	"time"

	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/session"
)

type SessionBody struct {
	Portals       []PortalBody `json:"portals"`
	GestioneReady bool         `json:"gestioneReady"`
}

type PortalBody struct {
	Portal           string     `json:"portal" enum:"cup,fse"`
	State            string     `json:"state" enum:"logged_out,active,expired"`
	AccessExpiresAt  *time.Time `json:"accessExpiresAt,omitempty"`
	RefreshExpiresAt *time.Time `json:"refreshExpiresAt,omitempty"`
	Login            LoginBody  `json:"login"`
}

type LoginBody struct {
	Phase string `json:"phase" enum:"idle,waiting,failed"`
	Error string `json:"error,omitempty"`
}

func toSessionBody(status session.Status, states map[session.Portal]loginflow.State) SessionBody {
	portals := make([]PortalBody, 0, len(status.Portals))
	for _, p := range status.Portals {
		login := LoginBody{Phase: string(loginflow.PhaseIdle)}
		if state, ok := states[p.Portal]; ok {
			login = LoginBody{Phase: string(state.Phase), Error: state.Error}
		}
		portals = append(portals, PortalBody{
			Portal:           string(p.Portal),
			State:            string(p.State),
			AccessExpiresAt:  timePtr(p.AccessExpiresAt),
			RefreshExpiresAt: timePtr(p.RefreshExpiresAt),
			Login:            login,
		})
	}
	return SessionBody{Portals: portals, GestioneReady: status.GestioneReady}
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
