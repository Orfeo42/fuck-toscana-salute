package httpapi

import (
	"time"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/cup"
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

type AppointmentsBody struct {
	Appointments []AppointmentBody `json:"appointments"`
}

type AppointmentBody struct {
	ID           string              `json:"id"`
	NRE          string              `json:"nre"`
	Status       string              `json:"status"`
	At           *time.Time          `json:"at,omitempty"`
	Unit         UnitBody            `json:"unit"`
	Prescription PrescriptionCUPBody `json:"prescription"`
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
	ID          string     `json:"id"`
	NRE         string     `json:"nre"`
	Description string     `json:"description"`
	Kind        string     `json:"kind"`
	Status      int        `json:"status"`
	Date        *time.Time `json:"date,omitempty"`
	DocumentRef string     `json:"documentRef"`
}

type PrescriptionsBody struct {
	Prescriptions []RowBody `json:"prescriptions"`
}

type RowBody struct {
	NRE string               `json:"nre"`
	CUP *PrescriptionCUPBody `json:"cup,omitempty"`
	FSE *PrescriptionFSEBody `json:"fse,omitempty"`
}

func toAppointmentsBody(entries []agenda.Entry) AppointmentsBody {
	appointments := make([]AppointmentBody, 0, len(entries))
	for _, e := range entries {
		appointments = append(appointments, AppointmentBody{
			ID:     e.Appointment.ID,
			NRE:    e.Appointment.NRE,
			Status: e.Appointment.Status,
			At:     timePtr(e.Appointment.At),
			Unit: UnitBody{
				Name:        e.Appointment.Unit.Name,
				Description: e.Appointment.Unit.Description,
				Address:     e.Appointment.Unit.Address,
				City:        e.Appointment.Unit.City,
				Province:    e.Appointment.Unit.Province,
			},
			Prescription: toPrescriptionCUPBody(e.Prescription),
		})
	}
	return AppointmentsBody{Appointments: appointments}
}

func toPrescriptionsBody(rows []agenda.Row) PrescriptionsBody {
	prescriptions := make([]RowBody, 0, len(rows))
	for _, r := range rows {
		row := RowBody{NRE: r.NRE}
		if r.CUP != nil {
			body := toPrescriptionCUPBody(*r.CUP)
			row.CUP = &body
		}
		if r.FSE != nil {
			row.FSE = &PrescriptionFSEBody{
				ID:          r.FSE.ID,
				NRE:         r.FSE.NRE,
				Description: r.FSE.Description,
				Kind:        string(r.FSE.Kind),
				Status:      r.FSE.Status,
				Date:        timePtr(r.FSE.Date),
				DocumentRef: r.FSE.DocumentRef,
			}
		}
		prescriptions = append(prescriptions, row)
	}
	return PrescriptionsBody{Prescriptions: prescriptions}
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
