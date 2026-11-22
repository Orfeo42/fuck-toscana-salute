package httpapi

import (
	"time"

	"fuck-toscana-salute/internal/localrecords"
	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/reports"
	"fuck-toscana-salute/internal/session"
)

type SessionBody struct {
	Portals       []PortalBody `json:"portals"`
	GestioneReady bool         `json:"gestioneReady"`
	LoginMode     string       `json:"loginMode" enum:"window,bookmarklet"`
	Viewer        bool         `json:"viewer"`
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

func toSessionBody(status session.Status, states map[session.Portal]loginflow.State, loginMode string, viewer bool) SessionBody {
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
	return SessionBody{Portals: portals, GestioneReady: status.GestioneReady, LoginMode: loginMode, Viewer: viewer}
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func toLocalReportBody(r localrecords.Report, nre string) ReportBody {
	nres := []string{}
	if nre != "" {
		nres = append(nres, nre)
	}
	return ReportBody{
		Kind:       string(referti.KindLocal),
		KindLabel:  referti.KindLocal.Label(),
		AbstractID: r.ID,
		Date:       timePtr(r.Meta.Date),
		UpdatedAt:  timePtr(r.CreatedAt),
		Provider:   r.Meta.Provider,
		Origin:     string(reports.OriginLocal),
		Title:      r.Meta.Title,
		NREs:       nres,
	}
}

type ReportRefBody struct {
	Kind       string `json:"kind"`
	AbstractID string `json:"abstractId"`
	DetailID   string `json:"detailId"`
	RepCode    string `json:"repCode"`
}
