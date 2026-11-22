# CLAUDE.md

Local-only dashboard over the Regione Toscana health portals (CUP 2.0, FSE, ZeroCode). Design, endpoints, storage and cache TTLs live in [docs/spec.md](docs/spec.md); redacted API shapes in [docs/api-skeleton.json](docs/api-skeleton.json); run and Google setup in [README.md](README.md). This file holds what those do not: decisions, refusals, workflow rules and verified gotchas. Keep the spec corrected when a finding changes it.

The repo name is the owner's choice. Keep it.

## Hard rules

- **The GitHub remote is PUBLIC.** Before every push, scan the diff for a real codice fiscale, tessera number, NRE, CUP booking number, PIN, token, cookie, client secret, person name or address. Tests, fixtures, docs and commit messages use fake data only.
- **Never show health data in chat or context.** Inspect upstream APIs structure-only: blank values in-page or by script, keep allowlisted field names, report only counts and status codes. Never read a raw HAR file; reduce it to structure with a script first, then delete it. Never repeat identifiers that appear in screenshots or pasted text.
- **No CAPTCHA bypass.** Never solve or automate the reCAPTCHA, never hide or spoof the automation flag to evade bot detection. The user solves it in the login browser.
- **No credential handling.** Never auto-type the codice fiscale or tessera into portal forms (government IDs). Allowed: browser/password-manager autofill inside the login profile, or a copy button next to the value. Note for autofill: the CUP form fields have only `id="codiceFiscale"` and `id="numbersTeam"`, no `name`.
- **No booking writes.** `setRiserva`, `setPrenota`, `setRilascia`, `setSpostaAppuntamento`, `verificaDisdetta`, `DELETE /disdetta` are never called. Booking and moving go through the official CUP flow (decision: buttons that open the logged-in CUP page).
- **Never call `GET {cupBe}/getPrescrizioneElettronica` automatically or in bulk**: it performs the "presa in carico" on the national prescription system. The verdict uses the read-only `getPrescrizioneElettronicaSecured`.
- **Referti are read-only.** Never call the FSE endpoints that mark a report as read or hide it ("oscura").
- Health-document identifiers travel in POST bodies, never in URL paths or query strings. Upstream errors and logs never carry response bodies, tokens or query strings; log error type and response field names only.

## Workflow

- Commit and push after each completed step (owner's standing request for this repo), after the PII scan above. No Claude attribution in commits.
- Plan in the main thread; freeze shared contracts first (huma OpenAPI, generated TS types, shared type aliases, compiling stubs), then run implementers in parallel. Only one agent regenerates the API contract.
- After any API change: `just openapi` then `just web-gen` (`web/openapi.json` → `web/src/api/schema.d.ts`, openapi-typescript + openapi-fetch).
- Scoped checks: `just test ./internal/<pkg>/...`, `just lint ./internal/<pkg>/...`, `just web-test`, `just web-lint`, `npm --prefix web run typecheck`, `just web-build`. Build output is ground truth, IDE diagnostics went stale here.
- Verify live through the app's own API on the running container, structure-only. Say so explicitly when a page was not opened in a real browser.
- The owner runs the app with `just up` / `just down` / `just logs` on `http://127.0.0.1:8800`. Do not stop it as a diagnostic step.
- Frontend package manager is npm (`package-lock.json` tracked). Dependencies are exact pins. TypeScript is held at 5.9.3 because openapi-typescript and typescript-eslint do not support 7 yet.
- Mantine overlays (Modal, Drawer) mount only while open (`{open ? <X opened /> : null}`). All Mantine portals share one DOM node (`reuseTargetNode: true`), so an always-mounted overlay inserts its wrapper at page load and, once opened, renders behind overlays opened later at the same z-index and cannot be edited. Tests use `MantineProvider env="test"`, which disables portals, so stacking can only be tested with a plain `MantineProvider` (see the stacking test in `GoogleProvider.test.tsx`).
- pdf.js methods need their object: `task.cancel` stored detached threw on close and React unmounted the whole app (blank page). There is no error boundary yet.
- Go: playwright-go module path is `github.com/mxschmitt/playwright-go`. HTTP shutdown waiting ~5 s for idle browser connections is expected.

## Decisions and their reasons

- **Cache reversal (2026-11-11).** First requirement was live calls only, no local storage. Reversed the same day: per-NRE lookups take ~5 s each and no list API exists, and the owner wants response history to analyse the upstream shapes and find a simpler source of booking truth. Result: encrypted Postgres in its own compose service (see spec, "Data storage").
- **Correct status, not hedges.** The owner rejected "Da verificare" as a default answer. Booking truth comes from the CUP appointment lookup plus the daily per-NRE CUP verdict; "Da verificare" stays only for genuine upstream errors.
- **Login, three iterations.** Playwright window on the host → bookmarklet (judged unusable, still reachable at `/#/collega`) → remote browser in the container: Xvnc + matchbox + websockify + Playwright Chromium under supervisord, proxied at `/viewer/`, shown in the app's own noVNC viewer with a clipboard paste button and a "Testo da inviare al browser" field for phones. Kept despite its limits (no own browser session, no saved passwords, SPID QR not scannable from the same phone: use the PosteID push instead).
- **Session storage.** GNOME Keyring → encrypted file → Postgres `secrets` table. The keyring was dropped because the container has none. Stored: renewal tokens, CF, tessera, "captcha done" flag, session start time. Silent re-login runs every minute for expired sessions (at most once per 10 min per portal) and only works while the SSO cookie in the browser profile is valid; CUP dates still need the Gestione captcha again.
- **UI.** Task-oriented, not a portal clone. Italian labels. Mantine 9 + mantine-datatable (adopted after hand-rolled CSS misaligned icons). Full-width sortable tables, default sort prescription date newest first; one row per prescription with its appointment ("Visite ed esami"), "Farmaci" on its own page, separate "Referti" page. Icon actions with tooltips, NRE text is the copy button, status filter as checkboxes in the Stato header, row click opens the details as a right-side drawer (2026-11-22: the centered modal with stacked sections was too tall and unreadable) with a sticky header (title, status, key facts, main actions) and tabs with counts, lists capped at 5 with "Mostra altri N", delete separated at the bottom of Modifica, place reduced to a map icon, status notes as info icon with tooltip, PDFs (ricetta, promemoria) in an in-app pdf.js modal, expired prescriptions hidden unless asked. Header and sidebar navy (`#1b2a41`), page and tables neutral grey; the owner rejected both an all-blue and an all-grey version. Light/dark follow the system.
- **Google Calendar.** Add-to-calendar with the promemoria PDF attached by default (Drive `drive.file`). OAuth client configured from the UI, stored encrypted. Sign-in on demand (2026-11-22): the owner wanted the fileworld-style button that just opens Google login when needed, so the redirect connect flow became the Google Identity Services popup code flow (code posted to the backend, refresh token stored as before); the one-time Cloud project setup cannot be avoided because the public repo cannot ship a client. Event: 1 h duration (CUP gives none), reminders 1 day and 1 hour, deduplicated on re-add, recreated if the user deleted it, orphan Drive PDF removed if the event insert fails. An existing event is linked, not re-offered: the dialog looks it up on open and offers "Rigenera" (update in place, old promemoria deleted). Default guests (owner's choice, 2026-11-22): added silently (`sendUpdates=none`, no invitation emails) and given read-only access to the promemoria PDF without Google's share email.
- **Local referti, links, local status (2026-11-21).** Uploaded referti (PDF/JPEG/PNG, from file or Google Drive via the Picker) are referti like the remote ones: same read endpoints, `origin` only drives an icon. Visite and referti can be linked by hand. A visita can carry a local status that replaces the remote one in the API (`overridden` flag, one status shown, "manual" icon); the backend keeps both and only logs how many differ. Manual statuses are the FSE codes only; Scaduta, Gestita altrove, Da verificare stay derived. Uploading a referto from a visita links it and sets Erogata. Drive import uses the Picker to keep the narrow `drive.file` scope (no `drive.readonly`). Details in the spec.
- **Deployment.** Web app in a container on the owner's machine, published on 127.0.0.1 only. Raspberry Pi / k8s with ingress is a future item, not to be built now.

## Rejected, do not re-propose

- Browser extension for login (the app must stay independent, nothing installed anywhere).
- Iframe or modal login: auth, CUP and FSE send `X-Frame-Options: SAMEORIGIN` and `frame-ancestors 'self'`. A header-stripping proxy is refused (removes protection, routes SPID credentials through the app, reCAPTCHA is domain-bound).
- Evading reCAPTCHA detection, auto-typing CF/tessera (see Hard rules).
- In-memory speed-up tricks for the ~5 s lookups (superseded by the DB cache). Mobile-app traffic interception, Gmail confirmation parsing, official API access requests: offered, not chosen.
- A hand-written "Prenotata fuori dal CUP regionale" status: it came from a bug (truncated 500 body, see spec) and was removed.

## Verified upstream facts not in the spec

- Tokens: access 5 min, refresh 30 min (`refresh_expires_in 1800`). Backend refresh works, but renewing may invalidate the page's own token. SSO max lifetime still unmeasured; session start and age are logged on each renewal to measure it.
- The Keycloak auth URL can be opened directly (`.../realms/rt/protocol/openid-connect/auth?response_type=code&client_id=<client>&redirect_uri=<portal>&scope=default`) and shows the SPID/CIE choice. After login Keycloak always returns to the portal home (fixed redirect URI, `#/...` cannot be one); the app then navigates to `#/gestione-prenotazioni` itself.
- With a persistent browser profile the Regione SSO cookie may make the second portal login silent; plan for two logins anyway.
- No appointment-list API exists anywhere: FSE `#/timeline` and `#/schede` show only past events, FSE "Diario appuntamenti" is manual, ZeroCode "Prenotazioni" is an admin stats page, `SelfAccettazione/ListaAppuntamenti` returned nothing for all active authorities even with appointments within 7 days. Per-NRE lookup is the only path.
- CUP verdict (`getPrescrizioneElettronicaSecured`) replies seen: SUCCESS with procedures (bookable; contains a `paziente` block, stored without it), ERROR "già completamente gestita tramite prenotazioni o accettazioni" (handled elsewhere), ERROR "Visualizzazione non consentita - ricetta scaduta", "Presa in carico non consentita - Ricetta non erogabile". The number of procedures does not predict the verdict.
- FSE status `5` = "Presa in carico": it can already have a CUP booking. Any booking found wins over the FSE status.
- Expiry exists only in the per-prescription detail (CUP list `dataScadenza` is null, FSE list has none); validity ranges from 180 to ~725 days.
- Map links must use the building's full address (name, street and number, CAP, city, province from `unita` / `sede`); the department address lacks the house number and pins the wrong place.
- A referto cannot be linked to its prescription by id: FSE `/visite/nre/dettaglio` returns 404 for completed (status 8) prescriptions and referto lists carry no NRE. Listing, opening and downloading referti do not mark them read.
- CUP sometimes returns transient non-JSON error pages during a load; reruns succeed. Concurrency of 4 per portal is fine.
- reCAPTCHA re-challenges often in the automated window; a persistent profile helps somewhat. Neither portal sends a CSP header (that is why the bookmarklet works).
- CUP rules (only relevant if booking is ever built): cancel up to 48 h before, move up to 24 h before.

## Open items (as of 2026-11-11, HEAD 67d2cf1)

- Error verdicts are cached 24 h like successful ones, pinning a transient error all day: cache errors 1 h. The row "Aggiorna" button is the manual retry.
- **Chromium runs with the sandbox off in the container** (Playwright default), mitigated only by read-only rootfs, no capabilities and 127.0.0.1 binding. Enable it (custom seccomp profile) before any LAN or Pi exposure. The browser profile (portal cookies) is not encrypted.
- Not tried live: Referti page in a browser and a referto PDF download, Google Calendar/Drive against real Google, silent re-login, ZeroCode with a real session.
- Lock test: whether the verdict call locks a bookable prescription (three probes showed no FSE status change; assumed non-locking).
- Captcha-skip test: whether `/prescrizione` works with the saved tessera after a CUP login without the Gestione step. If yes, the Gestione step could be dropped.
- Link referti to visit rows by date + provider against saved appointments.
- Prenota / Sposta buttons: copy NRE, open the logged-in CUP page (`#/prenotazione`, `#/gestione-prenotazioni`), refresh on close. Agreed, not built.
- Cosmetic: some session times shown in UTC instead of local time.
- Google popup sign-in and Calendar add work end to end with real Google (2026-11-22): `http://127.0.0.1:8800` is accepted as an Authorized JavaScript origin and the code exchange with the page origin as redirect URI works. Testing-mode consent requires the account in Audience → Test users (403 `access_denied` otherwise). Drive import through the Picker still not tried live. Over WireGuard from the phone the origin is a private IP, which Google rejects: normal upload only there.
- Cosmetic: Drive download errors go through `google.classify` and carry its "add event" prefix.
- Ricetta and promemoria PDFs are fetched with the document reference in the query string (`/api/documents/prescription?ref=`, `/api/documents/booking?id=`), against the hard rule above; move them to POST bodies with blob URLs like the referti.
- Document text of uploaded referti (future): add a column to `local_reports`.
