package localrecords

import (
	"context"
	"time"

	"fuck-toscana-salute/internal/referti"
	"fuck-toscana-salute/internal/store"
)

type localError string

func (e localError) Error() string { return string(e) }

const (
	ErrNotFound          localError = "localrecords: not found"
	ErrUnsupportedType   localError = "localrecords: unsupported file type"
	ErrTooLarge          localError = "localrecords: file too large"
	ErrEmptyFile         localError = "localrecords: empty file"
	ErrInvalidStatus     localError = "localrecords: invalid status"
	ErrInvalidReportKind localError = "localrecords: report kind cannot be linked"
)

const MaxFileBytes = 20 << 20

const DoneStatus = 8

type Origin string

const (
	OriginUpload Origin = "upload"
	OriginDrive  Origin = "drive"
)

type Meta struct {
	Title    string
	Date     time.Time
	Provider string
}

type Report struct {
	ID        string
	Meta      Meta
	Filename  string
	MimeType  string
	Origin    Origin
	CreatedAt time.Time
}

type NewReport struct {
	Meta     Meta
	Filename string
	Origin   Origin
	Data     []byte
	NRE      string
}

type File struct {
	Data     []byte
	MimeType string
	Filename string
}

type Link struct {
	NRE string
	Ref referti.Ref
}

type Override struct {
	NRE       string
	Status    int
	UpdatedAt time.Time
}

//go:generate moq -out store_mock.go . Store:MockStore
type Store interface {
	InsertLocalReport(ctx context.Context, r store.LocalReport, links []store.Link, override *store.Override) error
	UpdateLocalReportMeta(ctx context.Context, id string, meta []byte) error
	DeleteLocalReport(ctx context.Context, id, reportKey string) error
	LocalReports(ctx context.Context) ([]store.LocalReportMeta, error)
	LocalReportData(ctx context.Context, id string) (store.LocalReport, error)
	PutLink(ctx context.Context, l store.Link) error
	DeleteLink(ctx context.Context, l store.Link) error
	Links(ctx context.Context) ([]store.Link, error)
	PutOverride(ctx context.Context, o store.Override) error
	DeleteOverride(ctx context.Context, nre string) error
	Overrides(ctx context.Context) ([]store.Override, error)
}

type Service struct {
	store Store
	now   func() time.Time
}

func New(st Store) *Service {
	return &Service{store: st, now: time.Now}
}

func LocalRef(id string) referti.Ref {
	return referti.Ref{Kind: referti.KindLocal, AbstractID: id}
}
