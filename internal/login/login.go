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
	ErrLoginTimeout        = loginError("login: timed out waiting for the user")
	ErrWindowClosed        = loginError("login: window closed before completion")
	ErrTokensMissing       = loginError("login: tokens missing from page storage")
	ErrNoTessera           = loginError("login: tessera number not found")
	ErrUnknownPortal       = loginError("login: unknown portal")
	ErrInteractionRequired = loginError("login: identity provider requires user interaction")
	ErrTokenMismatch       = loginError("login: token does not belong to this portal")
	ErrTokenExpired        = loginError("login: access token already expired")
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
	silentTimeout   = 45 * time.Second
	realmIssuer     = "https://auth.regione.toscana.it/auth/realms/rt"
	cupClientID     = "r_toscan__portale_cup2_0"
	fseClientID     = "r_toscan__fse_web"
)

var interactiveRestoredArgs = []string{"--disable-extensions", "--password-store=basic"}

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

func ResultFromStorage(portal session.Portal, access, refresh, cf string) (Result, error) {
	if access == "" || refresh == "" {
		return Result{}, ErrTokensMissing
	}
	accessExp, err := jwtclaims.ExpiresAt(access)
	if err != nil {
		return Result{}, fmt.Errorf("login: access token expiry: %w", err)
	}
	refreshExp, err := jwtclaims.ExpiresAt(refresh)
	if err != nil {
		return Result{}, fmt.Errorf("login: refresh token expiry: %w", err)
	}
	return Result{
		Portal: portal,
		Tokens: session.Tokens{
			Access:           access,
			Refresh:          refresh,
			AccessExpiresAt:  accessExp,
			RefreshExpiresAt: refreshExp,
		},
		Identity: session.Identity{CodiceFiscale: strings.ToUpper(cf)},
	}, nil
}

func Validate(res Result, now time.Time) error {
	clientID := cupClientID
	switch res.Portal {
	case session.PortalCUP:
	case session.PortalFSE:
		clientID = fseClientID
	default:
		return ErrUnknownPortal
	}

	for _, token := range []string{res.Tokens.Access, res.Tokens.Refresh} {
		iss, err := jwtclaims.Issuer(token)
		if err != nil {
			return fmt.Errorf("login: token issuer: %w", err)
		}
		azp, err := jwtclaims.AuthorizedParty(token)
		if err != nil {
			return fmt.Errorf("login: token authorized party: %w", err)
		}
		if iss != realmIssuer || azp != clientID {
			return ErrTokenMismatch
		}
	}

	if !res.Tokens.AccessExpiresAt.After(now) {
		return ErrTokenExpired
	}
	return nil
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

type browser struct {
	pw     *playwright.Playwright
	bctx   playwright.BrowserContext
	page   playwright.Page
	closed chan struct{}
}

func (b *browser) close() error {
	return errors.Join(b.bctx.Close(), b.pw.Stop())
}

func (l *Launcher) open(headless bool) (*browser, error) {
	if err := playwright.Install(&playwright.RunOptions{SkipInstallBrowsers: true}); err != nil {
		return nil, fmt.Errorf("login: install driver: %w", err)
	}
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("login: start driver: %w", err)
	}
	if err := os.MkdirAll(l.profileDir, 0o700); err != nil {
		return nil, errors.Join(fmt.Errorf("login: create profile dir: %w", err), pw.Stop())
	}
	opts := playwright.BrowserTypeLaunchPersistentContextOptions{
		Channel:  playwright.String(l.channel),
		Headless: playwright.Bool(headless),
	}
	if !headless {
		opts.IgnoreDefaultArgs = interactiveRestoredArgs
	}
	bctx, err := pw.Chromium.LaunchPersistentContext(l.profileDir, opts)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("login: launch browser: %w", err), pw.Stop())
	}
	b := &browser{pw: pw, bctx: bctx, closed: make(chan struct{}, 2)}
	if pages := bctx.Pages(); len(pages) > 0 {
		b.page = pages[0]
	} else {
		b.page, err = bctx.NewPage()
		if err != nil {
			return nil, errors.Join(fmt.Errorf("login: new page: %w", err), b.close())
		}
	}
	b.page.OnClose(func(playwright.Page) {
		select {
		case b.closed <- struct{}{}:
		default:
		}
	})
	bctx.OnClose(func(playwright.BrowserContext) {
		select {
		case b.closed <- struct{}{}:
		default:
		}
	})
	return b, nil
}

func (b *browser) openGestione(ctx context.Context) {
	clickErr := b.page.Locator(gestioneLink).First().Click(playwright.LocatorClickOptions{Timeout: playwright.Float(actionTimeoutMs)})
	if clickErr != nil {
		if _, gotoErr := b.page.Goto(cupGestioneURL); gotoErr != nil {
			slog.WarnContext(ctx, "login gestione navigation failed", slog.String("error", gotoErr.Error()))
		}
	}
	if err := b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{State: playwright.LoadStateDomcontentloaded}); err != nil {
		slog.WarnContext(ctx, "login gestione load wait failed", slog.String("error", err.Error()))
	}
}

func (b *browser) waitTokens(ctx context.Context) (storageSnapshot, error) {
	var snapshot storageSnapshot
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for snapshot.Access == "" || snapshot.Refresh == "" {
		select {
		case <-ctx.Done():
			return storageSnapshot{}, ctxError(ctx)
		case <-b.closed:
			return storageSnapshot{}, ErrWindowClosed
		case <-ticker.C:
		}
		polled, pollErr := readSnapshot(b.page)
		if pollErr != nil {
			continue
		}
		snapshot = polled
	}
	return snapshot, nil
}

func (b *browser) tokensFlow(ctx context.Context, portal session.Portal, loginURL string) (Result, error) {
	if _, err := b.page.Goto(loginURL); err != nil {
		return Result{}, fmt.Errorf("login: open portal: %w", err)
	}
	snapshot, err := b.waitTokens(ctx)
	if err != nil {
		return Result{}, err
	}
	return ResultFromStorage(portal, snapshot.Access, snapshot.Refresh, snapshot.CF)
}

func (b *browser) fseFlow(ctx context.Context) (Result, error) {
	return b.tokensFlow(ctx, session.PortalFSE, fseLoginURL)
}

func (b *browser) cupFlow(ctx context.Context) (Result, error) {
	tesseraCh := make(chan string, 1)
	b.page.OnResponse(func(r playwright.Response) {
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

	if _, err := b.page.Goto(cupLoginURL); err != nil {
		return Result{}, fmt.Errorf("login: open portal: %w", err)
	}
	snapshot, err := b.waitTokens(ctx)
	if err != nil {
		return Result{}, err
	}

	b.openGestione(ctx)

	if _, bannerErr := b.page.Evaluate(bannerScript); bannerErr != nil {
		slog.WarnContext(ctx, "login banner injection failed", slog.String("error", bannerErr.Error()))
	}

	if focusErr := b.page.Locator(textFieldSel).First().Focus(playwright.LocatorFocusOptions{Timeout: playwright.Float(actionTimeoutMs)}); focusErr != nil {
		slog.WarnContext(ctx, "login field focus failed", slog.String("error", focusErr.Error()))
	}

	var tessera string
	select {
	case <-ctx.Done():
		return Result{}, ctxError(ctx)
	case <-b.closed:
		return Result{}, ErrWindowClosed
	case tessera = <-tesseraCh:
	}

	if latest, readErr := readSnapshot(b.page); readErr == nil && latest.Access != "" && latest.Refresh != "" {
		snapshot = latest
	}

	res, err := ResultFromStorage(session.PortalCUP, snapshot.Access, snapshot.Refresh, snapshot.CF)
	if err != nil {
		return Result{}, err
	}
	res.Identity.TesseraSanitaria = tessera
	res.Tokens.GestioneVerified = true
	return res, nil
}

func (l *Launcher) Login(ctx context.Context, portal session.Portal) (res Result, err error) {
	if portal != session.PortalCUP && portal != session.PortalFSE {
		return Result{}, ErrUnknownPortal
	}

	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	b, err := l.open(false)
	if err != nil {
		return Result{}, err
	}
	defer func() {
		err = errors.Join(err, b.close())
	}()

	if portal == session.PortalCUP {
		return b.cupFlow(ctx)
	}
	return b.fseFlow(ctx)
}

func (l *Launcher) LoginAll(ctx context.Context) (results []Result, err error) {
	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	b, err := l.open(false)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, b.close())
	}()

	cup, err := b.cupFlow(ctx)
	if err != nil {
		return nil, err
	}
	fse, err := b.fseFlow(ctx)
	if err != nil {
		return []Result{cup}, err
	}
	return []Result{cup, fse}, nil
}

func (l *Launcher) Silent(ctx context.Context, portal session.Portal) (res Result, err error) {
	var loginURL string
	switch portal {
	case session.PortalCUP:
		loginURL = cupLoginURL
	case session.PortalFSE:
		loginURL = fseLoginURL
	default:
		return Result{}, ErrUnknownPortal
	}

	ctx, cancel := context.WithTimeout(ctx, silentTimeout)
	defer cancel()

	b, err := l.open(true)
	if err != nil {
		return Result{}, err
	}
	defer func() {
		err = errors.Join(err, b.close())
	}()

	res, err = b.tokensFlow(ctx, portal, loginURL)
	if errors.Is(err, ErrLoginTimeout) {
		return Result{}, ErrInteractionRequired
	}
	return res, err
}
