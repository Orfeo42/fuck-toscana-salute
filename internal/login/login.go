package login

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
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
	cupLoginURL     = "https://auth.regione.toscana.it/auth/realms/rt/protocol/openid-connect/auth?client_id=r_toscan__portale_cup2_0&redirect_uri=https%3A%2F%2Fprenota.sanita.toscana.it&response_type=code&scope=default"
	fseLoginURL     = "https://auth.regione.toscana.it/auth/realms/rt/protocol/openid-connect/auth?client_id=r_toscan__fse_web&redirect_uri=https%3A%2F%2Ffse.sanita.toscana.it%2F&response_type=code&scope=profile+sdm-rs%2Fcittadino+sisorchestrator-rs%2Fcittadino+vaccini-rs%2Fcittadino"
	cupGestioneURL  = "https://prenota.sanita.toscana.it/#/gestione-prenotazioni"
	gestioneLink    = `a[href="#/gestione-prenotazioni"]`
	textFieldSel    = `input[type="text"]:not([disabled])`
	actionTimeoutMs = 5000
	verificaTsPath  = "/sis-portale-prenotazione/verificaTs"
	pollInterval    = time.Second
	storageScript   = `() => ({access: sessionStorage.getItem("access_token") || "", refresh: sessionStorage.getItem("refresh_token") || "", cf: sessionStorage.getItem("fcode") || sessionStorage.getItem("fseCf") || ""})`
	bannerScript    = `() => { const d = document.createElement("div"); d.style.cssText = "position:fixed;top:0;left:0;right:0;z-index:2147483647;background:#fff;color:#000;border:4px solid red;padding:12px;font:16px sans-serif;text-align:center"; d.textContent = "fuck-toscana-salute: inserisci codice fiscale e tessera sanitaria, risolvi il captcha e premi Login. La finestra si chiudera' da sola."; document.body.appendChild(d); }`
	tesseraQueryKey = "numbersTeam"
)

type storageSnapshot struct {
	Access, Refresh, CF string
}

type Launcher struct {
	channel    string
	profileDir string
	timeout    time.Duration
}

func New(browserChannel, profileDir string, timeout time.Duration) *Launcher {
	return &Launcher{channel: browserChannel, profileDir: profileDir, timeout: timeout}
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

func (l *Launcher) openGestione(ctx context.Context, page playwright.Page) {
	clickErr := page.Locator(gestioneLink).First().Click(playwright.LocatorClickOptions{Timeout: playwright.Float(actionTimeoutMs)})
	if clickErr != nil {
		if _, gotoErr := page.Goto(cupGestioneURL); gotoErr != nil {
			slog.WarnContext(ctx, "login gestione navigation failed", slog.String("error", gotoErr.Error()))
		}
	}
	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{State: playwright.LoadStateDomcontentloaded}); err != nil {
		slog.WarnContext(ctx, "login gestione load wait failed", slog.String("error", err.Error()))
	}
}

func (l *Launcher) Login(ctx context.Context, portal session.Portal) (res Result, err error) {
	var target string
	switch portal {
	case session.PortalCUP:
		target = cupLoginURL
	case session.PortalFSE:
		target = fseLoginURL
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

	if err := os.MkdirAll(l.profileDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("login: create profile dir: %w", err)
	}
	bctx, err := pw.Chromium.LaunchPersistentContext(l.profileDir, playwright.BrowserTypeLaunchPersistentContextOptions{
		Channel:  playwright.String(l.channel),
		Headless: playwright.Bool(false),
	})
	if err != nil {
		return Result{}, fmt.Errorf("login: launch browser: %w", err)
	}
	defer func() {
		err = errors.Join(err, bctx.Close())
	}()

	var page playwright.Page
	if pages := bctx.Pages(); len(pages) > 0 {
		page = pages[0]
	} else {
		page, err = bctx.NewPage()
		if err != nil {
			return Result{}, fmt.Errorf("login: new page: %w", err)
		}
	}

	closed := make(chan struct{}, 2)
	page.OnClose(func(playwright.Page) {
		select {
		case closed <- struct{}{}:
		default:
		}
	})
	bctx.OnClose(func(playwright.BrowserContext) {
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

	l.openGestione(ctx, page)

	if _, bannerErr := page.Evaluate(bannerScript); bannerErr != nil {
		slog.WarnContext(ctx, "login banner injection failed", slog.String("error", bannerErr.Error()))
	}

	if focusErr := page.Locator(textFieldSel).First().Focus(playwright.LocatorFocusOptions{Timeout: playwright.Float(actionTimeoutMs)}); focusErr != nil {
		slog.WarnContext(ctx, "login field focus failed", slog.String("error", focusErr.Error()))
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
