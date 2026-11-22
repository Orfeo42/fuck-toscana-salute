package httpapi

import (
	"fuck-toscana-salute/internal/visitsview"
)

func toVisitsQueryBody(p visitsview.Page) VisitsQueryBody {
	rows := make([]VisitRowBody, 0, len(p.Rows))
	for _, r := range p.Rows {
		rows = append(rows, toVisitRowBody(r))
	}
	return VisitsQueryBody{
		Rows:             rows,
		Total:            p.Total,
		Page:             p.Page,
		PageSize:         p.PageSize,
		Statuses:         toStatusOptionBodies(p.Statuses),
		ExcludedStatuses: cloneStrings(p.ExcludedStatuses),
		Search:           p.Search,
		Focus:            toFocusBody(p.Focus),
		Refreshing:       p.Refreshing,
		Updated:          toUpdatedBody(p.Updated),
		Issues:           toVisitsIssuesBody(p.Issues),
	}
}

func toVisitsIssuesBody(i visitsview.Issues) VisitsIssuesBody {
	return VisitsIssuesBody{Lists: string(i.Lists), Appointments: string(i.Appointments)}
}

func toVisitRowBody(r visitsview.Row) VisitRowBody {
	reports := make([]LinkedReportBody, 0, len(r.Reports))
	for _, l := range r.Reports {
		reports = append(reports, LinkedReportBody{
			Ref:       toReportRefBody(l.Ref),
			Label:     l.Label,
			Date:      timePtr(l.Date),
			Provider:  l.Provider,
			Origin:    string(l.Origin),
			Available: l.Available,
		})
	}
	return VisitRowBody{
		NRE:               r.NRE,
		Title:             r.Title,
		PrescribedOn:      timePtr(r.PrescribedOn),
		DocumentRef:       r.DocumentRef,
		Status:            toVisitStatusBody(r.Status),
		Appointment:       toVisitAppointmentBody(r.Appointment),
		AppointmentLookup: string(r.AppointmentLookup),
		Reports:           reports,
		CheckedAt:         VisitCheckedBody{Appointments: timePtr(r.CheckedAt.Appointments), Verdict: timePtr(r.CheckedAt.Verdict)},
		Upload:            UploadDefaultsBody{Title: r.Upload.Title, Date: r.Upload.Date, Provider: r.Upload.Provider},
		Refreshing:        r.Refreshing,
	}
}

func toVisitStatusBody(s visitsview.Status) VisitStatusBody {
	return VisitStatusBody{
		Key:             s.Key,
		Label:           s.Label,
		Tone:            s.Tone,
		Kind:            string(s.Kind),
		Code:            s.Code,
		Manual:          s.Manual,
		Note:            s.Note,
		VerifiedAt:      timePtr(s.VerifiedAt),
		CanBook:         s.CanBook,
		ExpiresOn:       timePtr(s.ExpiresOn),
		ExpiryEstimated: s.ExpiryEstimated,
		Details:         cloneStrings(s.Details),
	}
}

func toVisitAppointmentBody(a *visitsview.Appointment) *VisitAppointmentBody {
	if a == nil {
		return nil
	}
	return &VisitAppointmentBody{
		ID:          a.ID,
		BookingID:   a.BookingID,
		Status:      a.Status,
		At:          timePtr(a.At),
		RelativeDay: a.RelativeDay,
		Upcoming:    a.Upcoming,
		Unit: VisitUnitBody{
			Name:              a.Unit.Name,
			Description:       a.Unit.Description,
			DisplayName:       a.Unit.DisplayName,
			Place:             a.Unit.Place,
			Address:           a.Unit.Address,
			PostalCode:        a.Unit.PostalCode,
			City:              a.Unit.City,
			Province:          a.Unit.Province,
			FullAddress:       a.Unit.FullAddress,
			MapsSearchURL:     a.Unit.MapsSearchURL,
			MapsDirectionsURL: a.Unit.MapsDirectionsURL,
		},
		Calendar: CalendarFieldsBody{Title: a.Calendar.Title, Location: a.Calendar.Location, Details: a.Calendar.Details},
	}
}

func toVisitRowResponseBody(r visitsview.RowResult) VisitRowResponseBody {
	options := make([]ManualStatusOptionBody, 0, len(r.ManualStatus.Options))
	for _, o := range r.ManualStatus.Options {
		options = append(options, ManualStatusOptionBody{Code: o.Code, Label: o.Label})
	}
	return VisitRowResponseBody{
		Row:          toVisitRowBody(r.Row),
		ManualStatus: ManualStatusBody{Current: r.ManualStatus.Current, Options: options},
		Refreshing:   r.Refreshing,
		Issues:       toVisitsIssuesBody(r.Issues),
	}
}

func toVisitReportOptionsBody(options []visitsview.ReportOption) VisitReportOptionsBody {
	bodies := make([]ReportOptionBody, 0, len(options))
	for _, o := range options {
		bodies = append(bodies, ReportOptionBody{
			Ref:      toReportRefBody(o.Ref),
			Label:    o.Label,
			Date:     timePtr(o.Date),
			Provider: o.Provider,
			Origin:   string(o.Origin),
		})
	}
	return VisitReportOptionsBody{Options: bodies}
}
