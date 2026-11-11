package login

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"

	"fuck-toscana-salute/internal/jwtclaims"
	"fuck-toscana-salute/internal/session"
)

type Result struct {
	Portal   session.Portal
	Tokens   session.Tokens
	Identity session.Identity
}

type loginError string

func (e loginError) Error() string { return string(e) }

const (
	ErrLoginTimeout  = loginError("login: timed out waiting for the user")
	ErrWindowClosed  = loginError("login: window closed before completion")
	ErrTokensMissing = loginError("login: tokens missing from page storage")
	ErrNoTessera     = loginError("login: tessera number not found")
	ErrUnknownPortal = loginError("login: unknown portal")
)

const (
	cupURL          = "https://prenota.sanita.toscana.it/"
	fseURL          = "https://fse.sanita.toscana.it/"
	verificaTsPath  = "/sis-portale-prenotazione/verificaTs"
	pollInterval    = time.Second
	storageScript   = `() => ({access: sessionStorage.getItem("access_token") || "", refresh: sessionStorage.getItem("refresh_token") || "", cf: sessionStorage.getItem("fcode") || sessionStorage.getItem("fseCf") || ""})`
	bannerScript    = `() => { const d = document.createElement("div"); d.style.cssText = "position:fixed;top:0;left:0;right:0;z-index:2147483647;background:#fff;color:#000;border:4px solid red;padding:12px;font:16px sans-serif;text-align:center"; d.textContent = "fuck-toscana-salute: accesso OK. Ora apri \"Gestione Prenotazioni\", inserisci codice fiscale e tessera sanitaria, risolvi il captcha e premi Login. La finestra si chiudera' da sola."; document.body.appendChild(d); }`
	tesseraQueryKey = "numbersTeam"
)

type storageSnapshot struct {
	Access, Refresh, CF string
}

type Launcher struct {
	channel string
	timeout time.Duration
}

func New(browserChannel string, timeout time.Duration) *Launcher {
	return &Launcher{channel: browserChannel, timeout: timeout}
}

func resultFromStorage(portal session.Portal, s storageSnapshot) (Result, error) {
	if s.Access == "" || s.Refresh == "" {
		return Result{}, ErrTokensMissing
	}
	accessExp, err := jwtclaims.ExpiresAt(s.Access)
	if err != nil {
		return Result{}, fmt.Errorf("login: access token expiry: %w", err)
	}
	refreshExp, err := jwtclaims.ExpiresAt(s.Refresh)
	if err != nil {
		return Result{}, fmt.Errorf("login: refresh token expiry: %w", err)
	}
	return Result{
		Portal: portal,
		Tokens: session.Tokens{
			Access:           s.Access,
			Refresh:          s.Refresh,
			AccessExpiresAt:  accessExp,
			RefreshExpiresAt: refreshExp,
		},
		Identity: session.Identity{CodiceFiscale: strings.ToUpper(s.CF)},
	}, nil
}

func tesseraFromURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", ErrNoTessera
	}
	tessera := u.Query().Get(tesseraQueryKey)
	if tessera == "" {
		return "", ErrNoTessera
	}
	return tessera, nil
}

func ctxError(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ErrLoginTimeout
	}
	return ctx.Err()
}

func readSnapshot(page playwright.Page) (storageSnapshot, error) {
	raw, err := page.Evaluate(storageScript)
	if err != nil {
		return storageSnapshot{}, fmt.Errorf("login: read storage: %w", err)
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return storageSnapshot{}, ErrTokensMissing
	}
	access, okA := m["access"].(string)
	refresh, okR := m["refresh"].(string)
	cf, okC := m["cf"].(string)
	if !okA || !okR || !okC {
		return storageSnapshot{}, ErrTokensMissing
	}
	return storageSnapshot{Access: access, Refresh: refresh, CF: cf}, nil
}

func (l *Launcher) Login(ctx context.Context, portal session.Portal) (res Result, err error) {
	var target string
	switch portal {
	case session.PortalCUP:
		target = cupURL
	case session.PortalFSE:
		target = fseURL
	default:
		return Result{}, ErrUnknownPortal
	}

	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	if err := playwright.Install(&playwright.RunOptions{SkipInstallBrowsers: true}); err != nil {
		return Result{}, fmt.Errorf("login: install driver: %w", err)
	}
	pw, err := playwright.Run()
	if err != nil {
		return Result{}, fmt.Errorf("login: start driver: %w", err)
	}
	defer func() {
		err = errors.Join(err, pw.Stop())
	}()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Channel:  playwright.String(l.channel),
		Headless: playwright.Bool(false),
	})
	if err != nil {
		return Result{}, fmt.Errorf("login: launch browser: %w", err)
	}
	defer func() {
		err = errors.Join(err, browser.Close())
	}()

	bctx, err := browser.NewContext()
	if err != nil {
		return Result{}, fmt.Errorf("login: new context: %w", err)
	}
	page, err := bctx.NewPage()
	if err != nil {
		return Result{}, fmt.Errorf("login: new page: %w", err)
	}

	closed := make(chan struct{}, 2)
	page.OnClose(func(playwright.Page) {
		select {
		case closed <- struct{}{}:
		default:
		}
	})
	browser.OnDisconnected(func(playwright.Browser) {
		select {
		case closed <- struct{}{}:
		default:
		}
	})

	tesseraCh := make(chan string, 1)
	if portal == session.PortalCUP {
		page.OnResponse(func(r playwright.Response) {
			if r.Status() != 200 || !strings.Contains(r.URL(), verificaTsPath) {
				return
			}
			tessera, tErr := tesseraFromURL(r.URL())
			if tErr != nil {
				return
			}
			select {
			case tesseraCh <- tessera:
			default:
			}
		})
	}

	if _, err := page.Goto(target); err != nil {
		return Result{}, fmt.Errorf("login: open portal: %w", err)
	}

	var snapshot storageSnapshot
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for snapshot.Access == "" || snapshot.Refresh == "" {
		select {
		case <-ctx.Done():
			return Result{}, ctxError(ctx)
		case <-closed:
			return Result{}, ErrWindowClosed
		case <-ticker.C:
		}
		polled, pollErr := readSnapshot(page)
		if pollErr != nil {
			continue
		}
		snapshot = polled
	}

	if portal == session.PortalFSE {
		return resultFromStorage(portal, snapshot)
	}

	if _, bannerErr := page.Evaluate(bannerScript); bannerErr != nil {
		slog.WarnContext(ctx, "login banner injection failed", slog.String("error", bannerErr.Error()))
	}

	var tessera string
	select {
	case <-ctx.Done():
		return Result{}, ctxError(ctx)
	case <-closed:
		return Result{}, ErrWindowClosed
	case tessera = <-tesseraCh:
	}

	if latest, readErr := readSnapshot(page); readErr == nil && latest.Access != "" && latest.Refresh != "" {
		snapshot = latest
	}

	res, err = resultFromStorage(portal, snapshot)
	if err != nil {
		return Result{}, err
	}
	res.Identity.TesseraSanitaria = tessera
	return res, nil
}
