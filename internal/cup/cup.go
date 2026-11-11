package cup

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"fuck-toscana-salute/internal/dates"
	"fuck-toscana-salute/internal/session"
	"fuck-toscana-salute/internal/upstream"
)

type Status int

const (
	StatusPrescribed      Status = 3
	StatusBooked          Status = 30
	StatusCancelled       Status = 4
	StatusTakenInCharge   Status = 5
	StatusSuspended       Status = 6
	StatusPartiallyDone   Status = 7
	StatusDone            Status = 8
	StatusDoneAfterCancel Status = 9
)

func (s Status) Label() string {
	switch s {
	case StatusPrescribed:
		return "Prescritta"
	case StatusBooked:
		return "Prenotata"
	case StatusCancelled:
		return "Annullata"
	case StatusTakenInCharge:
		return "Presa in carico"
	case StatusSuspended:
		return "Sospesa"
	case StatusPartiallyDone:
		return "Erogata parziale"
	case StatusDone:
		return "Erogata"
	case StatusDoneAfterCancel:
		return "Erogata dopo annullamento"
	default:
		return "Sconosciuto"
	}
}

type Prescription struct {
	NRE          string
	PrescribedOn time.Time
	Kind         string
	Status       Status
	Items        []string
}

type Unit struct {
	Name        string
	Description string
	Place       string
	Address     string
	PostalCode  string
	City        string
	Province    string
}

type Appointment struct {
	ID        string
	BookingID string
	NRE       string
	Status    string
	At        time.Time
	Unit      Unit
}

type cupError string

func (e cupError) Error() string { return string(e) }

const (
	ErrGestioneNotReady = cupError("cup: tessera not captured, complete the Gestione Prenotazioni login step")
	ErrUpstreamOutcome  = cupError("cup: upstream outcome not ok")
	ErrNoDocument       = cupError("cup: document not found in response")
)

const (
	defaultCupBe     = "https://prenota.sanita.toscana.it/sis-portalepren/sis-portale-prenotazione/sis-portale-prenotazione"
	defaultCupList   = "https://api.regione.toscana.it/sanita/C06/nre-prenotazione-api/v1/sis-portale-prenotazione/api"
	listStartDate    = "2012-01-01"
	listPageSize     = 10
	listMaxPages     = 50
	outcomeOK        = "000"
	accessTypeNumRec = "3"
	noBookingMarker  = "Non esistono richieste"
	dateLayout       = "2006-01-02"
)

var sedeCodePrefix = regexp.MustCompile(`^\[\d+\]-`)

//go:generate moq -out tokensource_mock.go . TokenSource:MockTokenSource
type TokenSource interface {
	AccessToken(ctx context.Context, portal session.Portal) (string, error)
	Identity() session.Identity
	Status() session.Status
}

type Client struct {
	up      *upstream.Client
	tokens  TokenSource
	cupBe   string
	cupList string
	loc     *time.Location
	now     func() time.Time
}

func New(up *upstream.Client, tokens TokenSource) *Client {
	loc, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		loc = time.UTC
	}
	return &Client{
		up:      up,
		tokens:  tokens,
		cupBe:   defaultCupBe,
		cupList: defaultCupList,
		loc:     loc,
		now:     time.Now,
	}
}

type listRequest struct {
	DataInizio string `json:"dataInizio"`
	DataFine   string `json:"dataFine"`
	Page       int    `json:"page"`
	Size       int    `json:"size"`
	CF         string `json:"cf"`
}

type listResponse struct {
	InfoRicetta []infoRicetta `json:"infoRicetta"`
	CodiceEsito string        `json:"codiceEsito"`
	Estrazione  int           `json:"estrazione"`
}

type infoRicetta struct {
	NRE              string `json:"nre"`
	DataPrescrizione string `json:"dataPrescrizione"`
	TipoRicetta      string `json:"tipoRicetta"`
	Stato            string `json:"stato"`
	JSONPrescrizione string `json:"jsonPrescrizione"`
}

type prescriptionDetails struct {
	ListaDettagli []struct {
		DescrizioneProdPrest string `json:"descrizioneProdPrest"`
	} `json:"listaDettagli"`
}

type appointmentsResponse struct {
	Response *struct {
		ListaPrescrizioni *struct {
			Prescrizione []struct {
				IDPrescrizione    string `json:"idPrescrizione"`
				ListaAppuntamenti *struct {
					Appuntamento []appuntamento `json:"appuntamento"`
				} `json:"listaAppuntamenti"`
			} `json:"prescrizione"`
		} `json:"listaPrescrizioni"`
	} `json:"response"`
}

type appuntamento struct {
	IDAppuntamento    string `json:"idAppuntamento"`
	StatoAppuntamento string `json:"statoAppuntamento"`
	DataAppuntamento  string `json:"dataAppuntamento"`
	OraAppuntamento   string `json:"oraAppuntamento"`
	Unita             unita  `json:"unita"`
}

type unita struct {
	DescUnita       string `json:"descUnita"`
	NomeUnita       string `json:"nomeUnita"`
	IndirizzoUnita  string `json:"indirizzoUnita"`
	CapUnita        string `json:"capUnita"`
	NomeComuneUnita string `json:"nomeComuneUnita"`
	ProvinciaUnita  string `json:"provinciaUnita"`
	Sede            sede   `json:"sede"`
}

type sede struct {
	DescSede       string `json:"descSede"`
	IndirizzoSede  string `json:"indirizzoSede"`
	CapSede        string `json:"capSede"`
	NomeComuneSede string `json:"nomeComuneSede"`
	ProvinciaSede  string `json:"provinciaSede"`
}

type promemoriaResponse struct {
	Response *struct {
		File string `json:"file"`
	} `json:"response"`
}

func (c *Client) authHeader(ctx context.Context) (http.Header, error) {
	token, err := c.tokens.AccessToken(ctx, session.PortalCUP)
	if err != nil {
		return nil, fmt.Errorf("cup: access token: %w", err)
	}
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	return header, nil
}

func (c *Client) Prescriptions(ctx context.Context) ([]Prescription, error) {
	cf := c.tokens.Identity().CodiceFiscale
	if cf == "" {
		return nil, session.ErrNotLoggedIn
	}
	header, err := c.authHeader(ctx)
	if err != nil {
		return nil, err
	}
	body := listRequest{
		DataInizio: listStartDate,
		DataFine:   c.now().In(c.loc).AddDate(0, 0, 1).Format(dateLayout),
		Size:       listPageSize,
		CF:         cf,
	}
	prescriptions := []Prescription{}
	for page := 0; page < listMaxPages; page++ {
		body.Page = page
		resp, err := upstream.PostJSON[listRequest, listResponse](ctx, c.up, c.cupList+"/nre", header, body)
		if err != nil {
			return nil, fmt.Errorf("cup: list prescriptions: %w", err)
		}
		if resp.CodiceEsito != outcomeOK {
			return nil, fmt.Errorf("%w: %s", ErrUpstreamOutcome, resp.CodiceEsito)
		}
		for _, item := range resp.InfoRicetta {
			prescriptions = append(prescriptions, c.mapPrescription(item))
		}
		if resp.Estrazione != 0 {
			break
		}
	}
	return prescriptions, nil
}

func (c *Client) mapPrescription(item infoRicetta) Prescription {
	prescribedOn, err := dates.Parse(item.DataPrescrizione, c.loc)
	if err != nil {
		prescribedOn = time.Time{}
	}
	status, err := strconv.Atoi(item.Stato)
	if err != nil {
		status = 0
	}
	var items []string
	var details prescriptionDetails
	if err := json.Unmarshal([]byte(item.JSONPrescrizione), &details); err == nil {
		for _, d := range details.ListaDettagli {
			if d.DescrizioneProdPrest != "" {
				items = append(items, d.DescrizioneProdPrest)
			}
		}
	}
	return Prescription{
		NRE:          item.NRE,
		PrescribedOn: prescribedOn,
		Kind:         item.TipoRicetta,
		Status:       Status(status),
		Items:        items,
	}
}

func (c *Client) Appointments(ctx context.Context, nre string) ([]Appointment, error) {
	if !c.tokens.Status().GestioneReady {
		return nil, ErrGestioneNotReady
	}
	id := c.tokens.Identity()
	if id.CodiceFiscale == "" {
		return nil, session.ErrNotLoggedIn
	}
	header, err := c.authHeader(ctx)
	if err != nil {
		return nil, err
	}
	header.Set("cf", id.CodiceFiscale)
	nre = strings.ToUpper(nre)
	query := url.Values{}
	query.Set("numbersTeam", id.TesseraSanitaria)
	query.Set("accessType", accessTypeNumRec)
	query.Set("accessCode", nre)
	resp, err := upstream.GetJSON[appointmentsResponse](ctx, c.up, c.cupBe+"/prescrizione?"+query.Encode(), header)
	var statusErr *upstream.StatusError
	if errors.As(err, &statusErr) && statusErr.Status == http.StatusInternalServerError && statusErr.BodyContains(noBookingMarker) {
		return []Appointment{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cup: lookup appointments: %w", err)
	}
	appointments := []Appointment{}
	if resp.Response == nil || resp.Response.ListaPrescrizioni == nil {
		return appointments, nil
	}
	for _, p := range resp.Response.ListaPrescrizioni.Prescrizione {
		if p.ListaAppuntamenti == nil {
			continue
		}
		for _, a := range p.ListaAppuntamenti.Appuntamento {
			appointments = append(appointments, c.mapAppointment(nre, p.IDPrescrizione, a))
		}
	}
	return appointments, nil
}

func (c *Client) Promemoria(ctx context.Context, bookingID string) ([]byte, error) {
	if !c.tokens.Status().GestioneReady {
		return nil, ErrGestioneNotReady
	}
	id := c.tokens.Identity()
	if id.CodiceFiscale == "" {
		return nil, session.ErrNotLoggedIn
	}
	header, err := c.authHeader(ctx)
	if err != nil {
		return nil, err
	}
	header.Set("cf", id.CodiceFiscale)
	query := url.Values{}
	query.Set("idPrescrizione", bookingID)
	resp, err := upstream.GetJSON[promemoriaResponse](ctx, c.up, c.cupBe+"/promemoria?"+query.Encode(), header)
	if err != nil {
		return nil, fmt.Errorf("cup: promemoria: %w", err)
	}
	if resp.Response == nil || resp.Response.File == "" {
		return nil, fmt.Errorf("%w: empty file", ErrNoDocument)
	}
	pdf, err := base64.StdEncoding.DecodeString(resp.Response.File)
	if err != nil {
		return nil, fmt.Errorf("cup: decode promemoria: %w", err)
	}
	return pdf, nil
}

func (c *Client) mapAppointment(nre, bookingID string, a appuntamento) Appointment {
	at, err := dates.Parse(a.DataAppuntamento, c.loc)
	if err != nil {
		at = time.Time{}
	}
	if !at.IsZero() {
		at = c.withClock(at, a.OraAppuntamento)
	}
	return Appointment{
		ID:        a.IDAppuntamento,
		BookingID: bookingID,
		NRE:       nre,
		Status:    a.StatoAppuntamento,
		At:        at,
		Unit:      mapUnit(a.Unita),
	}
}

func mapUnit(u unita) Unit {
	unit := Unit{
		Name:        u.NomeUnita,
		Description: u.DescUnita,
		Place:       strings.TrimSpace(sedeCodePrefix.ReplaceAllString(u.Sede.DescSede, "")),
		Address:     u.IndirizzoUnita,
		PostalCode:  u.CapUnita,
		City:        u.NomeComuneUnita,
		Province:    u.ProvinciaUnita,
	}
	if u.Sede.IndirizzoSede == "" {
		return unit
	}
	unit.Address = u.Sede.IndirizzoSede
	unit.PostalCode = u.Sede.CapSede
	unit.City = u.Sede.NomeComuneSede
	unit.Province = u.Sede.ProvinciaSede
	return unit
}

func (c *Client) withClock(day time.Time, clock string) time.Time {
	parsed, err := time.Parse("15:04", clock)
	if err != nil {
		parsed, err = time.Parse("1504", clock)
	}
	if err != nil {
		return day
	}
	return time.Date(day.Year(), day.Month(), day.Day(), parsed.Hour(), parsed.Minute(), 0, 0, c.loc)
}
