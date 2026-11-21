package localrecords

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fuck-toscana-salute/internal/store"
)

const (
	dateLayout   = "2006-01-02"
	defaultTitle = "Referto"
)

type metaBlob struct {
	Title    string `json:"title"`
	Date     string `json:"date"`
	Provider string `json:"provider"`
	Filename string `json:"filename"`
	MimeType string `json:"mimeType"`
	Origin   string `json:"origin"`
}

func encodeMeta(r Report) ([]byte, error) {
	b := metaBlob{
		Title:    r.Meta.Title,
		Provider: r.Meta.Provider,
		Filename: r.Filename,
		MimeType: r.MimeType,
		Origin:   string(r.Origin),
	}
	if !r.Meta.Date.IsZero() {
		b.Date = r.Meta.Date.Format(dateLayout)
	}
	data, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("localrecords: encode meta: %w", err)
	}
	return data, nil
}

func decodeReport(id string, meta []byte, createdAt time.Time) (Report, error) {
	var b metaBlob
	if err := json.Unmarshal(meta, &b); err != nil {
		return Report{}, fmt.Errorf("localrecords: decode meta: %w", err)
	}
	r := Report{
		ID:        id,
		Meta:      Meta{Title: b.Title, Provider: b.Provider},
		Filename:  b.Filename,
		MimeType:  b.MimeType,
		Origin:    Origin(b.Origin),
		CreatedAt: createdAt,
	}
	if b.Date == "" {
		return r, nil
	}
	date, err := time.Parse(dateLayout, b.Date)
	if err != nil {
		return Report{}, fmt.Errorf("localrecords: decode meta date: %w", err)
	}
	r.Meta.Date = date
	return r, nil
}

func normalizeMeta(m Meta, filename string) Meta {
	m.Title = strings.TrimSpace(m.Title)
	m.Provider = strings.TrimSpace(m.Provider)
	if m.Title != "" {
		return m
	}
	m.Title = strings.TrimSpace(strings.TrimSuffix(filename, filepath.Ext(filename)))
	if m.Title == "" {
		m.Title = defaultTitle
	}
	return m
}

func normalizeNRE(nre string) string {
	return strings.ToUpper(strings.TrimSpace(nre))
}

func toStoreLocal(r Report, data []byte) (store.LocalReport, error) {
	meta, err := encodeMeta(r)
	if err != nil {
		return store.LocalReport{}, err
	}
	return store.LocalReport{ID: r.ID, Meta: meta, Data: data, CreatedAt: r.CreatedAt}, nil
}
