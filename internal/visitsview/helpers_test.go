package visitsview

import (
	"context"
	"time"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/cup"
	"fuck-toscana-salute/internal/fse"
	"fuck-toscana-salute/internal/localrecords"
	"fuck-toscana-salute/internal/refresh"
	"fuck-toscana-salute/internal/reports"
)

var testNow = time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)

func at(value string) time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return t
}

func fseRow(nre string, kind fse.Kind, status int, date, description string) agenda.Row {
	return agenda.Row{NRE: nre, FSE: &fse.Prescription{
		ID: "f-" + nre, NRE: nre, Description: description, Kind: kind, Status: status, Date: at(date),
	}}
}

func expiring(r agenda.Row, expiresOn string, estimated bool) agenda.Row {
	r.FSE.ExpiresAt = at(expiresOn)
	r.FSE.ExpiryEstimated = estimated
	return r
}

func booking(nre, status, when, name string) cup.Appointment {
	a := cup.Appointment{
		ID: "a-" + nre, NRE: nre, Status: status,
		Unit: cup.Unit{Name: name, Address: "Via Esempio 1", City: "Firenze", Province: "FI"},
	}
	if when != "" {
		a.At = at(when)
	}
	return a
}

func checked(appointments ...cup.Appointment) agenda.Lookup {
	return agenda.Lookup{Candidate: true, Checked: true, CheckedAt: at("2026-11-10T08:00:00Z"), Appointments: appointments}
}

func verdict(reason cup.VerdictReason, message string) *agenda.VerdictEntry {
	return &agenda.VerdictEntry{Verdict: cup.Verdict{Reason: reason, Message: message}, VerifiedAt: at("2026-11-10T08:00:00Z")}
}

func freshSnapshot(rows []agenda.Row, lookups map[string]agenda.Lookup) agenda.Snapshot {
	if lookups == nil {
		lookups = map[string]agenda.Lookup{}
	}
	list := agenda.ListState{At: at("2026-11-10T08:30:00Z")}
	return agenda.Snapshot{Rows: rows, CUPList: list, FSEList: list, Lookups: lookups}
}

type fixture struct {
	svc       *Service
	agenda    *MockAgenda
	reports   *MockReports
	local     *MockLocal
	refresher *MockRefresher
}

func newFixture(snap agenda.Snapshot) *fixture {
	f := &fixture{
		agenda: &MockAgenda{SnapshotFunc: func(context.Context) (agenda.Snapshot, error) { return snap, nil }},
		reports: &MockReports{CachedListFunc: func(context.Context) (reports.CachedListing, error) {
			return reports.CachedListing{}, nil
		}},
		local: &MockLocal{OverridesFunc: func(context.Context) ([]localrecords.Override, error) { return nil, nil }},
		refresher: &MockRefresher{
			KickFunc:   func(refresh.Job) {},
			StatusFunc: func() refresh.Status { return refresh.Status{} },
		},
	}
	f.svc = New(f.agenda, f.reports, f.local, f.refresher)
	f.svc.now = func() time.Time { return testNow }
	return f
}

func nres(rows []Row) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.NRE)
	}
	return out
}
