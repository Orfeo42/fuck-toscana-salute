package referti

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"

	"fuck-toscana-salute/internal/dates"
	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
)

type refertiError string

func (e refertiError) Error() string { return string(e) }

const (
	ErrUnknownKind = refertiError("referti: unknown kind")
	ErrAllFailed   = refertiError("referti: every report list failed")
	ErrNoDocument  = refertiError("referti: document not found in response")
	ErrNotPDF      = refertiError("referti: document is not a PDF")
)

const (
	defaultRepCode = "HDIS-RT:REP-900"
	unreadState    = "0000"
	hiddenState    = "0002"
	pdfMagic       = "%PDF"
)

var defaultBases = map[Kind]string{
	KindLab:         "https://api.regione.toscana.it/C04/fse-lab/v1/lab",
	KindTransfusion: "https://api.regione.toscana.it/C06/fse-labtrasf/v1/labtrasf",
	KindRadiology:   "https://api.regione.toscana.it/C07/fse-ris/v1/ris",
	KindDischarge:   "https://api.regione.toscana.it/C06/fse-ldo/v1/ldo",
	KindER:          "https://api.regione.toscana.it/C06/fse-ps/v1/ps",
	KindSpecialist:  "https://api.regione.toscana.it/C07/RSA/v1/rsa",
	KindPathology:   "https://apit.sanita.toscana.it/C06/fse-rap/v1",
}

type route struct {
	documents    string
	download     string
	byInternalID bool
}

var routes = map[Kind]route{
	KindLab:         {documents: "/dettaglio", download: "/download"},
	KindTransfusion: {documents: "/dettaglio", download: "/download"},
	KindRadiology:   {documents: "/abstract", download: "/download"},
	KindDischarge:   {documents: "/dettaglio", download: "/download"},
	KindER:          {documents: "/abstract", download: "/downloadVerbale"},
	KindSpecialist:  {documents: "/dettaglio", download: "/download", byInternalID: true},
	KindPathology:   {documents: "/dettaglio", download: "/download", byInternalID: true},
}

type Ref struct {
	Kind       Kind
	AbstractID string
	DetailID   string
	RepCode    string
}

type Report struct {
	Ref       Ref
	Date      time.Time
	UpdatedAt time.Time
	Provider  string
	Unread    bool
	Hidden    bool
}

type Failure struct {
	Kind Kind
	Err  error
}

type Listing struct {
	Reports  []Report
	Failures []Failure
}

type Document struct {
	Position   string
	Label      string
	AcceptedAt time.Time
	Provider   string
}

//go:generate moq -out tokensource_mock.go . TokenSource:MockTokenSource
type TokenSource interface {
	AccessToken(ctx context.Context, portal session.Portal) (string, error)
}

type Client struct {
	up     *upstream.Client
	tokens TokenSource
	bases  map[Kind]string
	loc    *time.Location
}

type listRequest struct{}

type listItem struct {
	AbstractID string `json:"actInternalIdAbstract"`
	DetailID   string `json:"actInternalIdDettaglio"`
	RepCode    string `json:"repCode"`
	Provider   string `json:"aziendaErogante"`
	Date       string `json:"dataPrestazione"`
	UpdatedAt  string `json:"dataAggiornamentoPrestazione"`
	ReadState  string `json:"statoConsultazione"`
	Visibility string `json:"statoVisibilita"`
}

type actRequest struct {
	AbstractID string `json:"actAbstractInternalId"`
	DetailID   string `json:"actDettaglioInternalId"`
	RepCode    string `json:"repCode"`
}

type downloadActRequest struct {
	AbstractID string `json:"actAbstractInternalId"`
	DetailID   string `json:"actDettaglioInternalId"`
	RepCode    string `json:"repCode"`
	Position   string `json:"posizione"`
}

type internalRequest struct {
	InternalID string `json:"internalId"`
	RepCode    string `json:"repCode"`
}

type documentItem struct {
	Position   string `json:"posizione"`
	AcceptedAt string `json:"dataAccettazione"`
	Provider   string `json:"aziendaErogante"`
	ERProvider string `json:"azienda"`
	ERDate     string `json:"data"`
	List       []struct {
		Position string `json:"posizione"`
		Label    string `json:"tipoVerbale"`
	} `json:"lista"`
}

type downloadResponse struct {
	Report  string `json:"refertoCodificatoBase64"`
	Verbale string `json:"verbaleCodificatoBase64"`
}

func New(up *upstream.Client, tokens TokenSource) *Client {
	loc, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		loc = time.UTC
	}
	bases := make(map[Kind]string, len(defaultBases))
	for k, v := range defaultBases {
		bases[k] = v
	}
	return &Client{up: up, tokens: tokens, bases: bases, loc: loc}
}

func (c *Client) header(ctx context.Context) (http.Header, error) {
	token, err := c.tokens.AccessToken(ctx, session.PortalFSE)
	if err != nil {
		return nil, fmt.Errorf("referti access token: %w", err)
	}
	return http.Header{"Authorization": {"Bearer " + token}}, nil
}

func (c *Client) actBase(kind Kind) string {
	if kind == KindTransfusion {
		return c.bases[KindLab]
	}
	return c.bases[kind]
}

func (c *Client) parseDate(value string) time.Time {
	date, err := dates.Parse(value, c.loc)
	if err != nil {
		return time.Time{}
	}
	return date
}

func (c *Client) List(ctx context.Context) (Listing, error) {
	header, err := c.header(ctx)
	if err != nil {
		return Listing{}, err
	}
	reports := make([][]Report, len(allKinds))
	errs := make([]error, len(allKinds))
	var wg sync.WaitGroup
	for i, kind := range allKinds {
		wg.Go(func() {
			reports[i], errs[i] = c.list(ctx, header, kind)
		})
	}
	wg.Wait()

	var listing Listing
	for i, kind := range allKinds {
		if errs[i] != nil {
			slog.WarnContext(ctx, "referti list failed", slog.String("kind", string(kind)), slog.Any("error", errs[i]))
			listing.Failures = append(listing.Failures, Failure{Kind: kind, Err: errs[i]})
			continue
		}
		listing.Reports = append(listing.Reports, reports[i]...)
	}
	if len(listing.Failures) == len(allKinds) {
		return Listing{}, ErrAllFailed
	}
	slices.SortStableFunc(listing.Reports, func(a, b Report) int { return b.Date.Compare(a.Date) })
	return listing, nil
}

func (c *Client) list(ctx context.Context, header http.Header, kind Kind) ([]Report, error) {
	items, err := upstream.PostJSON[listRequest, []listItem](ctx, c.up, c.bases[kind]+"/lista", header, listRequest{})
	if err != nil {
		return nil, fmt.Errorf("referti list %s: %w", kind, err)
	}
	out := make([]Report, 0, len(items))
	for _, item := range items {
		out = append(out, Report{
			Ref:       Ref{Kind: kind, AbstractID: item.AbstractID, DetailID: item.DetailID, RepCode: item.RepCode},
			Date:      c.parseDate(item.Date),
			UpdatedAt: c.parseDate(item.UpdatedAt),
			Provider:  item.Provider,
			Unread:    item.ReadState == unreadState,
			Hidden:    item.Visibility == hiddenState,
		})
	}
	return out, nil
}

func (c *Client) Documents(ctx context.Context, ref Ref) ([]Document, error) {
	rt, ok := routes[ref.Kind]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownKind, ref.Kind)
	}
	header, err := c.header(ctx)
	if err != nil {
		return nil, err
	}
	url := c.actBase(ref.Kind) + rt.documents
	var raw json.RawMessage
	if rt.byInternalID {
		raw, err = upstream.PostJSON[internalRequest, json.RawMessage](ctx, c.up, url, header, internalRequest{InternalID: ref.DetailID, RepCode: ref.RepCode})
	} else {
		raw, err = upstream.PostJSON[actRequest, json.RawMessage](ctx, c.up, url, header, actRequest{AbstractID: ref.AbstractID, DetailID: ref.DetailID, RepCode: ref.RepCode})
	}
	if err != nil {
		return nil, fmt.Errorf("referti documents %s: %w", ref.Kind, err)
	}
	items, err := decodeItems(raw)
	if err != nil {
		return nil, fmt.Errorf("referti documents %s: %w", ref.Kind, err)
	}
	return c.toDocuments(ref.Kind, rt, items), nil
}

func decodeItems(raw json.RawMessage) ([]documentItem, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] == 'n' {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var items []documentItem
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return nil, fmt.Errorf("decode documents: %w", err)
		}
		return items, nil
	}
	var item documentItem
	if err := json.Unmarshal(trimmed, &item); err != nil {
		return nil, fmt.Errorf("decode documents: %w", err)
	}
	return []documentItem{item}, nil
}

func (c *Client) toDocuments(kind Kind, rt route, items []documentItem) []Document {
	var docs []Document
	for _, item := range items {
		if kind == KindER {
			for _, entry := range item.List {
				docs = append(docs, Document{
					Position:   entry.Position,
					Label:      entry.Label,
					AcceptedAt: c.parseDate(item.ERDate),
					Provider:   item.ERProvider,
				})
			}
			continue
		}
		if item.Position == "" && !rt.byInternalID {
			continue
		}
		docs = append(docs, Document{
			Position:   item.Position,
			AcceptedAt: c.parseDate(item.AcceptedAt),
			Provider:   item.Provider,
		})
	}
	if len(docs) == 0 && rt.byInternalID {
		docs = append(docs, Document{})
	}
	return docs
}

func (c *Client) Download(ctx context.Context, ref Ref, position string) ([]byte, error) {
	rt, ok := routes[ref.Kind]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownKind, ref.Kind)
	}
	header, err := c.header(ctx)
	if err != nil {
		return nil, err
	}
	url := c.actBase(ref.Kind) + rt.download
	var resp downloadResponse
	if rt.byInternalID {
		resp, err = upstream.PostJSON[internalRequest, downloadResponse](ctx, c.up, url, header, internalRequest{InternalID: ref.DetailID, RepCode: ref.RepCode})
	} else {
		repCode := ref.RepCode
		if repCode == "" {
			repCode = defaultRepCode
		}
		resp, err = upstream.PostJSON[downloadActRequest, downloadResponse](ctx, c.up, url, header, downloadActRequest{AbstractID: ref.AbstractID, DetailID: ref.DetailID, RepCode: repCode, Position: position})
	}
	if err != nil {
		return nil, fmt.Errorf("referti download %s: %w", ref.Kind, err)
	}
	encoded := resp.Report
	if ref.Kind == KindER {
		encoded = resp.Verbale
	}
	if encoded == "" {
		return nil, ErrNoDocument
	}
	pdf, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("referti decode document: %w", err)
	}
	if !strings.HasPrefix(string(pdf), pdfMagic) {
		return nil, ErrNotPDF
	}
	return pdf, nil
}
