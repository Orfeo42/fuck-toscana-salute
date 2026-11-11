package httpapi

import (
	"errors"
	"fmt"
	"time"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/loginflow"
	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
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

type AppointmentsBody struct {
	Appointments []AppointmentBody `json:"appointments"`
	Failed       []FailedBody      `json:"failed"`
	Unbooked     []string          `json:"unbooked"`
}

type FailedBody struct {
	NRE    string `json:"nre"`
	Reason string `json:"reason"`
}

type AppointmentBody struct {
	ID        string               `json:"id"`
	BookingID string               `json:"bookingId,omitempty"`
	NRE       string               `json:"nre"`
	Status    string               `json:"status"`
	At        *time.Time           `json:"at,omitempty"`
	Unit      UnitBody             `json:"unit"`
	CUP       *PrescriptionCUPBody `json:"cup,omitempty"`
	FSE       *PrescriptionFSEBody `json:"fse,omitempty"`
}

type UnitBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Address     string `json:"address"`
	City        string `json:"city"`
	Province    string `json:"province"`
}

type PrescriptionCUPBody struct {
	NRE          string     `json:"nre"`
	PrescribedOn *time.Time `json:"prescribedOn,omitempty"`
	Kind         string     `json:"kind"`
	Status       int        `json:"status"`
	Items        []string   `json:"items"`
}

type PrescriptionFSEBody struct {
	ID              string     `json:"id"`
	NRE             string     `json:"nre"`
	Description     string     `json:"description"`
	Kind            string     `json:"kind"`
	Status          int        `json:"status"`
	Date            *time.Time `json:"date,omitempty"`
	DocumentRef     string     `json:"documentRef"`
	ExpiresOn       *time.Time `json:"expiresOn,omitempty"`
	ExpiryEstimated bool       `json:"expiryEstimated,omitempty"`
}

type PrescriptionsBody struct {
	Prescriptions []RowBody `json:"prescriptions"`
}

type RowBody struct {
	NRE string               `json:"nre"`
	CUP *PrescriptionCUPBody `json:"cup,omitempty"`
	FSE *PrescriptionFSEBody `json:"fse,omitempty"`
}

func failureReason(err error) string {
	var statusErr *upstream.StatusError
	switch {
	case errors.Is(err, session.ErrSessionExpired), errors.Is(err, session.ErrNotLoggedIn):
		return "session_expired"
	case errors.Is(err, cup.ErrGestioneNotReady):
		return "gestione_not_ready"
	case errors.As(err, &statusErr):
		return fmt.Sprintf("upstream_%d", statusErr.Status)
	}
	return "error"
}

func toAppointmentsBody(result agenda.Result) AppointmentsBody {
	appointments := make([]AppointmentBody, 0, len(result.Entries))
	for _, e := range result.Entries {
		appointments = append(appointments, AppointmentBody{
			ID:        e.Appointment.ID,
			BookingID: e.Appointment.BookingID,
			NRE:       e.NRE,
			Status:    e.Appointment.Status,
			At:        timePtr(e.Appointment.At),
			Unit: UnitBody{
				Name:        e.Appointment.Unit.Name,
				Description: e.Appointment.Unit.Description,
				Address:     e.Appointment.Unit.Address,
				City:        e.Appointment.Unit.City,
				Province:    e.Appointment.Unit.Province,
			},
			CUP: toPrescriptionCUPBodyPtr(e.CUP),
			FSE: toPrescriptionFSEBodyPtr(e.FSE),
		})
	}
	failed := make([]FailedBody, 0, len(result.Failed))
	for _, f := range result.Failed {
		failed = append(failed, FailedBody{NRE: f.NRE, Reason: failureReason(f.Err)})
	}
	unbooked := make([]string, 0, len(result.Unbooked))
	unbooked = append(unbooked, result.Unbooked...)
	return AppointmentsBody{Appointments: appointments, Failed: failed, Unbooked: unbooked}
}

func toPrescriptionsBody(rows []agenda.Row) PrescriptionsBody {
	prescriptions := make([]RowBody, 0, len(rows))
	for _, r := range rows {
		row := RowBody{NRE: r.NRE}
		row.CUP = toPrescriptionCUPBodyPtr(r.CUP)
		row.FSE = toPrescriptionFSEBodyPtr(r.FSE)
		prescriptions = append(prescriptions, row)
	}
	return PrescriptionsBody{Prescriptions: prescriptions}
}

func toPrescriptionCUPBodyPtr(p *cup.Prescription) *PrescriptionCUPBody {
	if p == nil {
		return nil
	}
	body := toPrescriptionCUPBody(*p)
	return &body
}

func toPrescriptionFSEBodyPtr(p *fse.Prescription) *PrescriptionFSEBody {
	if p == nil {
		return nil
	}
	return &PrescriptionFSEBody{
		ID:              p.ID,
		NRE:             p.NRE,
		Description:     p.Description,
		Kind:            string(p.Kind),
		Status:          p.Status,
		Date:            timePtr(p.Date),
		DocumentRef:     p.DocumentRef,
		ExpiresOn:       timePtr(p.ExpiresAt),
		ExpiryEstimated: p.ExpiryEstimated,
	}
}

func toPrescriptionCUPBody(p cup.Prescription) PrescriptionCUPBody {
	items := make([]string, 0, len(p.Items))
	items = append(items, p.Items...)
	return PrescriptionCUPBody{
		NRE:          p.NRE,
		PrescribedOn: timePtr(p.PrescribedOn),
		Kind:         p.Kind,
		Status:       int(p.Status),
		Items:        items,
	}
}
