package cup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
)

type VerdictReason string

const (
	VerdictBookable VerdictReason = "bookable"
	VerdictHandled  VerdictReason = "handled"
	VerdictExpired  VerdictReason = "expired"
	VerdictOther    VerdictReason = "other"
)

type Verdict struct {
	Reason     VerdictReason
	Message    string
	HTTPStatus int
	Raw        []byte
}

const (
	levelSuccess       = "SUCCESS"
	levelError         = "ERROR"
	markerHandled      = "completamente gestita"
	markerExpired      = "scaduta"
	boilerplateInfix   = ": Impossibile recuperare la prescrizione elettronica con codice "
	genericErrorPrefix = "Errore generico: "
	verdictPath        = "/getPrescrizioneElettronicaSecured"
)

type verdictRequest struct {
	CF  string `json:"cf"`
	NRE string `json:"nre"`
}

type verdictResponse struct {
	Message struct {
		Level       string `json:"level"`
		Description string `json:"description"`
	} `json:"message"`
}

func (c *Client) Verdict(ctx context.Context, nre string) (Verdict, error) {
	cf := c.tokens.Identity().CodiceFiscale
	if cf == "" {
		return Verdict{}, session.ErrNotLoggedIn
	}
	header, err := c.authHeader(ctx)
	if err != nil {
		return Verdict{}, err
	}
	nre = strings.ToUpper(nre)
	body := verdictRequest{CF: strings.ToUpper(cf), NRE: nre}
	raw, err := upstream.PostJSON[verdictRequest, json.RawMessage](ctx, c.up, c.cupList+verdictPath, header, body)
	if err != nil {
		return Verdict{}, fmt.Errorf("cup: verdict: %w", err)
	}
	var parsed verdictResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Verdict{}, fmt.Errorf("cup: decode verdict: %w", err)
	}
	description := parsed.Message.Description
	return Verdict{
		Reason:     classifyVerdict(parsed.Message.Level, description),
		Message:    cleanVerdictMessage(nre, description),
		HTTPStatus: http.StatusOK,
		Raw:        raw,
	}, nil
}

func classifyVerdict(level, description string) VerdictReason {
	if level == levelSuccess {
		return VerdictBookable
	}
	if level != levelError {
		return VerdictOther
	}
	if strings.Contains(description, markerHandled) {
		return VerdictHandled
	}
	if strings.Contains(description, markerExpired) {
		return VerdictExpired
	}
	return VerdictOther
}

func cleanVerdictMessage(nre, description string) string {
	boilerplate := nre + boilerplateInfix + nre + ": "
	cleaned := strings.TrimPrefix(description, boilerplate)
	cleaned = strings.TrimPrefix(cleaned, genericErrorPrefix)
	return strings.TrimSpace(cleaned)
}
