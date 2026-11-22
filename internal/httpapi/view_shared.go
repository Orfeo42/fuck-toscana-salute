package httpapi

import (
	"time"

	"fuck-toscana-salute/internal/listview"
	"fuck-toscana-salute/internal/referti"
)

type StatusOptionBody struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Tone     string `json:"tone" enum:"booked,done,tobook,pending,closed,error"`
	Count    int    `json:"count" minimum:"0"`
	Excluded bool   `json:"excluded"`
}

type UpdatedBody struct {
	At    *time.Time `json:"at" nullable:"true"`
	Label string     `json:"label"`
}

type FocusBody struct {
	NRE   string `json:"nre"`
	Found bool   `json:"found"`
}

type RowRequest struct {
	NRE string `json:"nre" required:"true" pattern:"^[0-9A-Za-z]{10,20}$"`
}

func toStatusOptionBodies(options []listview.StatusOption) []StatusOptionBody {
	bodies := make([]StatusOptionBody, 0, len(options))
	for _, o := range options {
		bodies = append(bodies, StatusOptionBody{Key: o.Key, Label: o.Label, Tone: o.Tone, Count: o.Count, Excluded: o.Excluded})
	}
	return bodies
}

func toUpdatedBody(u listview.Updated) UpdatedBody {
	return UpdatedBody{At: timePtr(u.At), Label: u.Label}
}

func toFocusBody(f *listview.Focus) *FocusBody {
	if f == nil {
		return nil
	}
	return &FocusBody{NRE: f.NRE, Found: f.Found}
}

func cloneStrings(in []string) []string {
	return append(make([]string, 0, len(in)), in...)
}

func toReportRefBody(ref referti.Ref) ReportRefBody {
	return ReportRefBody{Kind: string(ref.Kind), AbstractID: ref.AbstractID, DetailID: ref.DetailID, RepCode: ref.RepCode}
}
