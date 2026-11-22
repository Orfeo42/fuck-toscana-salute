package visitsview

import (
	"slices"
	"strings"
	"time"

	"fuck-toscana-salute/internal/agenda"
	"fuck-toscana-salute/internal/itfmt"
	"fuck-toscana-salute/internal/refresh"
	"fuck-toscana-salute/internal/reports"
)

type inputs struct {
	snap      agenda.Snapshot
	overrides map[string]int
	linked    map[string][]LinkedReport
	forced    map[string]bool
	failed    map[string]refresh.Issue
	visitsRun bool
	now       time.Time
}

func linkedByNRE(listing reports.CachedListing) map[string][]LinkedReport {
	linked := map[string][]LinkedReport{}
	for _, r := range listing.Listing.Reports {
		for _, nre := range r.NREs {
			key := strings.ToUpper(nre)
			linked[key] = append(linked[key], LinkedReport{
				Ref:       r.Ref,
				Label:     reportLabel(r),
				Date:      r.Date,
				Provider:  r.Provider,
				Origin:    r.Origin,
				Available: true,
			})
		}
	}
	return linked
}

func (in inputs) rows() []Row {
	rows := make([]Row, 0, len(in.snap.Rows))
	for _, r := range in.snap.Rows {
		if !r.IsVisit() {
			continue
		}
		rows = append(rows, in.build(r))
	}
	return rows
}

func (in inputs) build(r agenda.Row) Row {
	key := strings.ToUpper(r.NRE)
	lookup := in.snap.Lookups[key]
	title := r.Title()
	booking, hasBooking := agenda.BestBooking(lookup.Appointments)
	live := hasBooking && booking.Status != cancelledBooking

	st := statusInput{
		status:   statusToBook,
		unbooked: lookup.Candidate && lookup.Checked && len(lookup.Appointments) == 0,
		verdict:  lookup.Verdict,
		booked:   live,
		now:      in.now,
	}
	row := Row{NRE: r.NRE, Title: title, PrescribedOn: r.PrescribedOn(), Reports: in.linked[key]}
	if row.Reports == nil {
		row.Reports = []LinkedReport{}
	}
	if r.FSE != nil {
		st.status = r.FSE.Status
		st.expiresOn = r.FSE.ExpiresAt
		st.estimated = r.FSE.ExpiryEstimated
		row.DocumentRef = r.FSE.DocumentRef
	}
	if code, ok := in.overrides[key]; ok {
		st.override = &code
	}
	row.Status = derive(st)

	row.Upload = UploadDefaults{Title: title, Date: itfmt.Day(in.now)}
	if live {
		row.Appointment = toAppointment(booking, title, in.now)
		row.Upload.Provider = row.Appointment.Unit.DisplayName
		if !booking.At.IsZero() {
			row.Upload.Date = itfmt.Day(booking.At)
		}
	}
	row.CheckedAt = Checked{}
	if lookup.Checked {
		row.CheckedAt.Appointments = lookup.CheckedAt
	}
	if lookup.Verdict != nil {
		row.CheckedAt.Verdict = lookup.Verdict.VerifiedAt
	}
	row.AppointmentLookup = in.lookupState(key, lookup)
	row.Refreshing = in.forced[key] || (in.visitsRun && lookup.Candidate && (!lookup.Checked || lookup.Stale))
	return row
}

func (in inputs) lookupState(key string, lookup agenda.Lookup) Lookup {
	if !lookup.Candidate {
		return LookupNone
	}
	if _, failed := in.failed[key]; failed {
		return LookupFailed
	}
	if lookup.Checked {
		return LookupDone
	}
	return LookupChecking
}

func findRow(rows []Row, nre string) int {
	return slices.IndexFunc(rows, func(r Row) bool { return strings.EqualFold(r.NRE, nre) })
}

func reportLabel(r reports.Report) string {
	if r.Title != "" {
		return r.Title
	}
	return r.Ref.Kind.Label()
}
