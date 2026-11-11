package fse

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"fuck-toscana-salute/internal/dates"
	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
)

type Kind string

const (
	KindVisit Kind = "visit"
	KindDrug  Kind = "drug"
)

type Prescription struct {
	ID          string
	NRE         string
	Description string
	Kind        Kind
	Status      int
	Date        time.Time
	DocumentRef string
}

type fseError string

func (e fseError) Error() string { return string(e) }

const ErrNoDocument = fseError("fse: document not found in response")

const (
	defaultVisitBase = "https://apit.sanita.toscana.it/C06/fse-ricette-visite/v1"
	defaultDrugBase  = "https://apit.sanita.toscana.it/C06/fse-ricette-farmaci/v1"
	drugRefMarker    = "_PREFARM"
)

//go:generate moq -out tokensource_mock.go . TokenSource:MockTokenSource
type TokenSource interface {
	AccessToken(ctx context.Context, portal session.Portal) (string, error)
}

type Client struct {
	up        *upstream.Client
	tokens    TokenSource
	visitBase string
	drugBase  string
	loc       *time.Location
}

type listRequest struct{}

type listResponse struct {
	Content []listItem `json:"content"`
}

type listItem struct {
	ID                    string `json:"id"`
	NRE                   string `json:"nre"`
	Description           string `json:"description"`
	Stato                 string `json:"stato"`
	DataPrestazione       string `json:"dataPrestazione"`
	ActInternalIDAbstract string `json:"actInternalIdAbstract"`
}

type downloadRequest struct {
	CFDelegante *string `json:"cfDelegante"`
	NRE         string  `json:"nre"`
	CFAssistito *string `json:"cfAssistito"`
}

type downloadResponse struct {
	Risposta struct {
		ContenutoRisposta struct {
			DatoSingolo []datoSingolo `json:"datoSingolo"`
		} `json:"contenutoRisposta"`
	} `json:"risposta"`
}

type datoSingolo struct {
	NomeCampo   string `json:"nomeCampo"`
	ValoreCampo string `json:"valoreCampo"`
}

func New(up *upstream.Client, tokens TokenSource) *Client {
	loc, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		loc = time.UTC
	}
	return &Client{
		up:        up,
		tokens:    tokens,
		visitBase: defaultVisitBase,
		drugBase:  defaultDrugBase,
		loc:       loc,
	}
}

func (c *Client) header(ctx context.Context) (http.Header, error) {
	token, err := c.tokens.AccessToken(ctx, session.PortalFSE)
	if err != nil {
		return nil, fmt.Errorf("fse access token: %w", err)
	}
	return http.Header{"Authorization": {"Bearer " + token}}, nil
}

func (c *Client) Prescriptions(ctx context.Context) ([]Prescription, error) {
	visits, err := c.list(ctx, c.visitBase, KindVisit)
	if err != nil {
		return nil, err
	}
	drugs, err := c.list(ctx, c.drugBase, KindDrug)
	if err != nil {
		return nil, err
	}
	return append(visits, drugs...), nil
}

func (c *Client) list(ctx context.Context, base string, kind Kind) ([]Prescription, error) {
	header, err := c.header(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := upstream.PostJSON[listRequest, listResponse](ctx, c.up, base+"/listanew", header, listRequest{})
	if err != nil {
		return nil, fmt.Errorf("fse list %s: %w", kind, err)
	}
	out := make([]Prescription, 0, len(resp.Content))
	for _, item := range resp.Content {
		status, err := strconv.Atoi(item.Stato)
		if err != nil {
			status = 0
		}
		date, err := dates.Parse(item.DataPrestazione, c.loc)
		if err != nil {
			date = time.Time{}
		}
		out = append(out, Prescription{
			ID:          item.ID,
			NRE:         item.NRE,
			Description: item.Description,
			Kind:        kind,
			Status:      status,
			Date:        date,
			DocumentRef: item.ActInternalIDAbstract,
		})
	}
	return out, nil
}

func (c *Client) PrescriptionPDF(ctx context.Context, documentRef string) ([]byte, error) {
	if documentRef == "" {
		return nil, ErrNoDocument
	}
	base := c.visitBase
	if strings.Contains(documentRef, drugRefMarker) {
		base = c.drugBase
	}
	nre, _, _ := strings.Cut(documentRef, "_")
	header, err := c.header(ctx)
	if err != nil {
		return nil, err
	}
	body := downloadRequest{NRE: nre}
	resp, err := upstream.PostJSON[downloadRequest, downloadResponse](ctx, c.up, base+"/nre/download", header, body)
	if err != nil {
		return nil, fmt.Errorf("fse download: %w", err)
	}
	for _, field := range resp.Risposta.ContenutoRisposta.DatoSingolo {
		if field.NomeCampo != "ricettaBase64" && field.NomeCampo != "refertoCodificatoBase64" {
			continue
		}
		pdf, err := base64.StdEncoding.DecodeString(field.ValoreCampo)
		if err != nil {
			return nil, fmt.Errorf("fse decode document: %w", err)
		}
		return pdf, nil
	}
	return nil, ErrNoDocument
}
