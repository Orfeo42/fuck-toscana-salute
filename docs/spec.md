# fuck-toscana-salute — spec

Personal dashboard over the Regione Toscana health portals (CUP 2.0, Fascicolo Sanitario Elettronico, ZeroCode). One place to answer: which prescriptions are booked, when and where is the appointment, what results/documents exist.

## Goals

- Cached live data: views read through a Postgres-backed response cache (see "Data storage") that falls back to the regional APIs when a record is missing or older than its TTL. Everything health-related is stored encrypted.
- Task-oriented UI, not a clone of the portals: upcoming appointments first, unbooked prescriptions with expiry, results, medicines/vaccines, search across all of it.
- Merge CUP + FSE + ZeroCode into one view.
- Runs locally only (health data is GDPR special-category). Never deployed or published.

## Non-goals / hard constraints

- No CAPTCHA bypass or automation. The user solves the reCAPTCHA in the app-owned login window.
- No credential handling: SPID/CIE login and the CF/tessera form are filled by the user in the login window. Tokens live in backend memory and in the encrypted secrets table (so a restart does not force a new login).
- No writes to bookings (book/move/cancel) until each endpoint is verified with explicit user approval.

## Architecture

- Local backend (required): the regional APIs answer CORS only for their own origins (`access-control-allow-origin: https://prenota.sanita.toscana.it` / `https://fse.sanita.toscana.it`), so a browser page on localhost cannot call them. The backend proxies calls, holds tokens in memory, refreshes them.
- App-owned login window (Playwright, headed): the user logs in to each portal there; the backend reads `access_token` / `refresh_token` from the page `sessionStorage` after login, then keeps the session alive with the refresh grant.
- Frontend served by the backend on localhost.

## Data storage

- Postgres runs as the `db` compose service; the backend reads `FTS_DATABASE_HOST` (required), `FTS_DATABASE_PORT` (5432), `FTS_DATABASE_NAME` (`fts`), `FTS_DATABASE_USER` (`fts`) and `FTS_DATABASE_PASSWORD_FILE` (required). `FTS_SECRET_KEY_FILE` (base64 of 32 bytes, mode 0600) is the encryption key and is also required.
- Encryption: HKDF-SHA256 derives an AES-256-GCM key and an HMAC-SHA256 key from the secret key. Every sensitive column (subject, payload, secret value) is sealed with AES-GCM, the source and the subject MAC bound as AAD. Lookups use the HMAC of the subject (NRE or secret key), so no identifier is stored in clear. A wrong key fails at startup when any stored row cannot be decrypted.
- `responses` table (history, append-only): source, subject MAC, encrypted subject, `fetched_at`, status, encrypted payload. Sources: `cup.prescriptions`, `cup.appointments`, `cup.verdict`, `fse.prescriptions`, `fse.reports`. The CUP verdict is stored as the raw upstream JSON with the `paziente` block removed. Reads take the latest row per source and subject.
- `secrets` table: encrypted session snapshot and Google token / settings (service `fuck-toscana-salute`, users `session`, `google`, `google-settings`). The previous `secrets.enc` file in the data directory is copied into the table on first start (existing table entries win) and renamed `secrets.enc.migrated`. The system keyring backend was removed.
- Cache TTLs: lists (prescriptions, reports) 1 h, appointments per NRE 1 h, verdicts per NRE 24 h. An expired or missing record is fetched live and stored; when the live call fails the stale record is served. Freshness is exposed: `GET /api/appointments`, `/api/prescriptions` and `/api/reports` return `updated` timestamps.
- Refresh: `POST /api/refresh` with `{all: true}` forces lists, appointments, verdicts and the reports list; with `{nres: [...]}` forces only those NREs (appointments and verdicts). Same-origin only (`Origin` must match `Host`).

## Authentication

Keycloak realm `rt` at `https://auth.regione.toscana.it/auth/realms/rt/protocol/openid-connect/{auth,token,userinfo,logout}`. SPID/CIE brokered by Keycloak. Public clients (no secret), authorization-code flow.

| Portal | client_id | redirect_uri | scope |
| --- | --- | --- | --- |
| CUP 2.0 | `r_toscan__portale_cup2_0` | `https://prenota.sanita.toscana.it` | `profile rtroles` |
| FSE | `r_toscan__fse_web` | `https://fse.sanita.toscana.it/` | (see `config.js`) |

- Token lifetimes (observed): `expires_in: 300`, `refresh_expires_in: 1800`.
- Refresh: `POST /token` form `grant_type=refresh_token&client_id=<client>&refresh_token=<rt>`.
- The two portals do NOT share the SSO session in practice: logging into FSE after CUP sent the user back to the SPID IdP. Plan for two logins per session.
- Redirect URIs are fixed to the portal origins, so the app cannot receive the code itself — hence the login window + `sessionStorage` read.
- After login the CUP page stores `access_token`, `refresh_token`, `fcode` (codice fiscale) in `sessionStorage`; FSE stores `access_token`, `refresh_token`, `fseCf`.
- API calls send `Authorization: Bearer <access_token>`. CUP calls also send header `cf: <CODICE FISCALE uppercase>`.

### "Gestione prenotazioni" extra step (needed for appointment dates)

1. `POST {cupBe}/validateCaptcha` body `{codeCapctha: <reCAPTCHA token>}` — user solves the reCAPTCHA in the login window.
2. `GET {cupBe}/verificaTs?numbersTeam=<tessera number>` with header `cf`.
3. Then `GET {cupBe}/prescrizione?...` lookups. Observed: several lookups in a row work after one captcha.

Open: whether `/prescrizione` called from the backend (outside the page) is accepted after the captcha was solved in the window — i.e. whether captcha state is server-side and tied to the token/CF.

## Endpoints

Bases:

- `cupBe` = `https://prenota.sanita.toscana.it/sis-portalepren/sis-portale-prenotazione/sis-portale-prenotazione`
- `cupList` = `https://api.regione.toscana.it/sanita/C06/nre-prenotazione-api/v1/sis-portale-prenotazione/api`
- `selfAcc` = `https://api.regione.toscana.it/sanita/C06/SelfAccettazione/v2/`
- FSE bases from `https://fse.sanita.toscana.it/config.js` (`api.regione.toscana.it/...`, `apit.sanita.toscana.it/...`).

Full redacted request/response shapes: `docs/api-skeleton.json` (structure only, values blanked).

### CUP

| Purpose | Call | Notes |
| --- | --- | --- |
| Unbooked prescriptions | `POST {cupList}/nre` body `{dataInizio:"yyyy-MM-dd", dataFine:"yyyy-MM-dd", page, size:10, cf}` | `infoRicetta[]{nre, dataPrescrizione, tipoRicetta, stato, jsonPrescrizione}`; `jsonPrescrizione` is a JSON string with `listaDettagli[]{descrizioneProdPrest,...}`. Paginate while `estrazione === 0`. Portal uses `dataInizio=2012-01-01`. |
| Prescription detail | `POST {cupList}/getPrescrizioneElettronicaSecured` body `{cf, nre}` | `response.{paziente, listaPrescrizioni.prescrizione[], esito}`. Appointment fields here stay null; do not use for dates. |
| Appointment lookup (dates) | `GET {cupBe}/prescrizione?numbersTeam=<tessera>&accessType=<type>&accessCode=<code uppercase>` headers `cf`, `Authorization` | Verified 200 for a booking made in the portal. `response.listaPrescrizioni.prescrizione[].listaAppuntamenti.appuntamento[]{idAppuntamento, statoAppuntamento ("PRENOTATO", "DISDETTO"...), dataAppuntamento, oraAppuntamento, unita{descUnita, nomeUnita, indirizzoUnita, nomeComuneUnita, provinciaUnita, erogatore{...}, sede{...}}}`. NRE with no CUP booking → HTTP 500 + Java exception ("Non esistono richieste per il paziente" in the UI). `accessType`: `1` PIN, `2` numero prenotazione CUP, `3` numero ricetta (NRE). Codes from the portal code; `3` verified live. |
| Promemoria PDF | `GET {cupBe}/promemoria?idPrescrizione=<id>` | Verified. `idPrescrizione` comes with the appointment lookup. |
| Other docs | `{cupBe}/allegati`, `/documentiDisponibili?idPrescrizione=`, `/moduloPagamento?idPrescrizione=`, `/ricevutaDisdetta` | untested |
| Write ops (do not call without approval) | `getListaDisponibilitaPost`, `setRiserva`, `setPrenota`, `setRilascia`, `setSpostaAppuntamento`, `verificaDisdetta`, `DELETE /disdetta?idPrestazione=&idAppuntamento=` | untested |
| Self check-in list | `POST {selfAcc}api/SelfAccettazione/ListaAppuntamenti` headers `X_CAST_ID_APPLICATIVO: self-accettazione-api-r_toscan__portale_cup2_0`, `Destinazione: <IdAzienda>` body `{Paziente:{Identificativo:<cf>}, DaData, AData}` (`YYYY-MM-DDTHH:mm:ss`) | `Esito 0` ok / `1` error. Returned "no appointment" everywhere so far; only near-term, self-check-in-eligible appointments. IdAzienda: 090904 AOUM, 090903 AOUC, 090201 ASL TC, 090203 ASL TSE, 090902 AOUS (active); 090202 ASL TNO, 090901 AOUP inactive (HTTP 400). |

CUP prescription `stato` map (from portal code): `3` Prescritta, `30` Prenotata, `4` Annullata, `5` Presa in carico, `6` Sospesa, `7` Erogata parziale, `8` Erogata, `9` Erogata dopo annullamento.

### FSE

All `POST` with body `{}` unless noted, `Authorization: Bearer`.

| Purpose | Call |
| --- | --- |
| Prescriptions (visits) | `apit.sanita.toscana.it/C06/fse-ricette-visite/v1/listanew` → `{content[]{id, nre, description, stato, dataPrestazione (dd/MM/yyyy), actInternalIdAbstract, actInternalIdDettaglio, repCode,...}, totalElements,...}` |
| Prescriptions (drugs) | `apit.sanita.toscana.it/C06/fse-ricette-farmaci/v1/listanew` (same shape) |
| Prescription PDF | `{ricettevisite\|ricettefarmaci}Endpoint/nre/download` (see `downloadEpre` in `js/common/dataService.js`) → `risposta.contenutoRisposta.datoSingolo[]` with `ricettaBase64` |
| Lab | `api.regione.toscana.it/C04/fse-lab/v1/lab/lista` |
| Lab transfusion | `.../C06/fse-labtrasf/v1/labtrasf/lista` |
| Radiology | `.../C07/fse-ris/v1/ris/lista` |
| ER | `.../C06/fse-ps/v1/ps/lista` |
| Discharge letters | `.../C06/fse-ldo/v1/ldo/lista` |
| RSA | `.../C07/RSA/v1/rsa/lista` |
| RAP | `apit.sanita.toscana.it/C06/fse-rap/v1/lista` |
| Vaccines | `.../C06/fse-vacc/v1/vacc/lista` |
| Medicines | `.../sanita/C06/Farmaci/v1/farmaci/lista` |
| User info | `GET .../sanita/C06/ANAGFSE/v1/getInfoUtenteFromCf/<cf>` |
| Document PDF | `POST <serviceEndpoint>/download` body `{actAbstractInternalId, actDettaglioInternalId, repCode, posizione}` → `{refertoCodificatoBase64}`. Verified for lab with `posizione: "999"`. ER uses `/downloadVerbale`; RSA/RAP body is `{internalId, repCode}`. |

List items share: `id, dataPrestazione, dataAggiornamentoPrestazione, actInternalIdAbstract, actInternalIdDettaglio, repCode, aziendaErogante, statoConsultazione, statoVisibilita, idActConsent`.

FSE `stato` on prescriptions is NOT a reliable "booked" flag: a prescription showed `30` before it was booked in the CUP. Booking truth = CUP appointment lookup.

FSE "Diario → Appuntamenti" is a user-maintained diary, not CUP bookings.

The gestione appointment lookup answers HTTP 500 with JSON `{"message":"Non esistono richieste per il paziente"}` when the NRE has no bookings. That body is ~11 KB with a Java `stackTrace` before `message`, so the marker must be matched on the parsed `message` of the full body, never on a truncated prefix (a 4 KB prefix looked like a non-JSON text error). Any other HTTP 500 is an upstream fault, not a missing booking.

A prescription with FSE `stato` 30 and a confirmed empty CUP appointment lookup is ambiguous: some are still bookable, others are refused by the CUP booking flow as "già completamente gestita tramite prenotazioni o accettazioni" (handled outside the online CUP). No read-only endpoint distinguishes them, so the UI shows "Da verificare" and offers the CUP link.

FSE `nre/dettaglio` `dataScadenza` is sometimes the placeholder year 9999 (e.g. `9999-12-31T00:00:00.000`). The app then estimates the expiry as prescription date + 180 days (observed on 6/6 prescriptions with a real expiry) and flags it as estimated. A 2024 placeholder-expiry prescription is refused by the CUP as "Ricetta non erogabile".

The CUP booking flow's `GET {cupBe}/getPrescrizioneElettronica?cf&nre&numbersTeam` performs the "presa in carico" (state-changing on the national prescription system, paired with `/setRilascia`). The app must never call it automatically.

### Reports (referti)

Read-only: the backend lists the reports of the seven FSE kinds, lists the documents of a report and downloads a PDF. It never calls the endpoints that mark a report as read or hide it ("oscura"), so `unread` and `hidden` are only mirrored from `statoConsultazione` / `statoVisibilita`. Identifiers of health documents travel in POST bodies, never in URLs.

- `GET /api/reports` → `{reports[]{kind, kindLabel, abstractId, detailId, repCode, date, updatedAt, provider, unread, hidden, origin, title?, nres[]}, failures[]{kind, kindLabel, message}, updated}`; a kind that fails is reported in `failures` and does not hide the others. Remote (FSE) and local (uploaded) referti come in the same list; `origin` is `remote` or `local`, `nres` are the visite the referto is linked to. When the whole FSE list is unavailable (not logged in, upstream down) the local referti are still returned with a `failures` entry of kind `fse`.
- `POST /api/reports/documents` body `{kind, abstractId, detailId, repCode}` → `{documents[]{position, label, acceptedAt?, provider?}}`.
- `POST /api/reports/pdf` body `{kind, abstractId, detailId, repCode, position}` → the file with its real `Content-Type` (`application/pdf`, `image/jpeg` or `image/png`).

The merge lives in `internal/reports` (composite over the cached FSE client and `internal/localrecords`).

### Local referti, links and status overrides

Uploaded referti are referti like the remote ones: a local referto has `kind: "local"`, its opaque id in `abstractId`, one document at position `1`, and is read through the same three endpoints above. Accepted types are PDF, JPEG and PNG, detected from the file bytes, max 20 MB.

- `POST /api/reports/local` (multipart: `file`, `title?`, `date?` `YYYY-MM-DD`, `provider?`, `nre?`) → 201 `ReportBody`. With `nre`, the referto is linked to that visita and the visita's local status is set to `8` Erogata, in one transaction.
- `POST /api/reports/local/drive` body `{fileId, title?, date?, provider?, nre?}`: same, with the file downloaded from Google Drive (picked in the Google Picker, see Google settings).
- `PUT /api/reports/local` body `{id, title, date?, provider?}` edits the metadata; `DELETE /api/reports/local` body `{id}` deletes the referto and its links.
- `PUT /api/links` / `DELETE /api/links` body `{nre, report{kind, abstractId, detailId, repCode}}` links or unlinks a visita and a referto (remote or local). Idempotent. Linking also sets the visita's local status to `8` Erogata in the same transaction; unlinking leaves the status as is.
- `PUT /api/visits/status` body `{nre, status}` sets a local status; `DELETE /api/visits/status` body `{nre}` removes it. Allowed values are the FSE prescription codes `3, 30, 5, 6, 7, 8, 9, 4`; the derived states (Scaduta, Gestita altrove, Da verificare) cannot be set by hand.
- `GET /api/prescriptions` rows carry `overridden` and `reports[]` (linked report refs). When a local status exists the backend puts it in place of the remote one (`fse.status`, or `cup.status` for a row without FSE data) and sets `overridden: true`; the UI shows one status with a "manual" icon and skips the booking/expiry/verdict derivation for that row. The remote status is never overwritten: it stays in the `responses` cache, the local one in `visit_overrides`, and the backend logs how many overridden rows differ from the remote status.

Storage (encrypted like the rest, see "Data storage"): `local_reports` (opaque id, encrypted metadata and file), `report_links` (HMAC of the NRE + HMAC of the report key, encrypted payload), `visit_overrides` (HMAC of the NRE, encrypted `{nre, status}`). Document text extraction is a future column.

### Google settings

The OAuth client is configured from the UI, not from a file: `GET /api/google/settings` → `{clientId, calendarId, driveFolderId, apiKey, guests, hasSecret, javascriptOrigin}` (the secret is never returned); `PUT /api/google/settings` body `{clientId, clientSecret?, calendarId?, driveFolderId?, apiKey?, guests?}` (an empty secret keeps the stored one; changing the client id drops the stored token) is stored encrypted in the `secrets` table. The client id must have the form `<project number>-<id>.apps.googleusercontent.com`. `javascriptOrigin` is built from the request (`X-Forwarded-Proto` or `http`, plus `Host`) and must be registered as an Authorized JavaScript origin of the Google client.

Sign-in is on demand, with the Google Identity Services popup code flow: `GET /api/google` → `{configured, connected, clientId, drive}` (`drive` = an API key is set). When Google is configured the page loads `accounts.google.com/gsi/client` and prepares a code client (`ux_mode: popup`, scopes `calendar.events` and `drive.file`). A click on "Aggiungi" (Calendar), "Scegli da Google Drive" or "Accedi con Google" while not connected opens the Google popup synchronously in the click handler (otherwise the browser blocks it), then posts the one-time code to `POST /api/google/code` body `{code}` with header `X-Requested-With: XmlHttpRequest` (same-origin only). The backend exchanges it with the client secret and the page origin as redirect URI (no PKCE in popup mode) and stores the token encrypted. No refresh token in the reply (the grant already existed) → the backend revokes the token, which resets the grant, and answers 409 `google_grant_reset`; the next consent returns a refresh token. Missing scope in the reply → revoke and 403 `google_scopes_missing`. A revoked or expired token (401 / `invalid_grant` from Google) deletes the stored token and answers 401 `google_not_connected`; the page then asks for the click again. The redirect flow (`/api/google/connect`, `/api/google/callback`) was removed.

Calendar events: `POST /api/calendar/events/lookup` body `{appointmentId}` → `{exists, htmlLink}` looks the event up by its deterministic id without creating anything (a cancelled event counts as missing). `POST /api/calendar/events` creates the event when it does not exist and leaves an existing one untouched (`created: false`). With `regenerate: true` an existing event is rewritten in place: a fresh promemoria PDF is uploaded and attached, the event is updated, then the old promemoria files (attachment titles ending in `promemoria.pdf` or starting with `promemoria-`, with a Drive file id) are deleted from Drive on a best-effort basis; other attachments are kept. If the update fails the newly uploaded file is deleted.

Default guests: `guests` in the Google settings (at most 10 bare email addresses, trimmed, deduplicated case-insensitively; display-name forms are rejected with 422) are used as event attendees. The event request may override them with `guests` (omitted: use the settings guests; empty array: no guests). Attendees are added silently: event insert and update use `sendUpdates=none`, and the promemoria PDF is shared with each guest as a read-only Drive permission created with `sendNotificationEmail=false`; if sharing fails the uploaded file is deleted and the event is not written. Guest emails are stored encrypted with the Google settings and are never logged (the log only carries the guest count).

Drive import uses the Google Picker with the existing `drive.file` scope: `POST /api/google/picker` → `{accessToken, apiKey, appId}` (a short-lived access token from the stored refresh token; 409 when the API key is missing). `appId` is the numeric prefix of the client id, which is the Cloud project number: `drive.file` only grants access to the picked file when it matches the OAuth client's project. The page opens the Picker (script from `apis.google.com`, iframe from `docs.google.com`, allowed in the CSP), the user picks a file, and the backend downloads it with `files.get?alt=media`. The page sends `Referrer-Policy: strict-origin-when-cross-origin` so a website-restricted API key sees the app origin. CSP additions for sign-in: `script-src https://accounts.google.com/gsi/client`, `frame-src` and `connect-src https://accounts.google.com/gsi/`, `style-src https://accounts.google.com/gsi/style`. `connect-src` also allows `blob:`: report PDFs reach pdf.js as blob URLs created by the page, and pdf.js fetches them (without it the in-app viewer failed and only "open in a new tab" worked).

### ZeroCode (blood draws)

Separate vendor app (`cdn.dromedian.com/zerocode`), own backend at `https://zerocode.sanita.toscana.it/api/*`, own SPID login (likely cookie session, not the `rt` Keycloak). Known paths: `/api/profile`, `/api/memo/<id>` (promemoria PDF), `/api/payment-notice/<id>`, `/api/cube`. Not tested with a real session yet.

## Screens

- Upcoming: appointments sorted by date, each with prescription, place, and promemoria one click away.
- To book: unbooked prescriptions with prescription date / expiry and the CUP verdict (`bookable`, `handled`, `expired`, `other`) fetched per unbooked NRE via `getPrescrizioneElettronicaSecured` (read-only).
- Results: lab, radiology, discharge letters, ER, newest first, PDF inline.
- Medicines & vaccines.
- Search across everything.

## Phase 0 results (verified 2026-11-11, live, from a non-browser client without `Origin`)

- CUP: `/prescrizione` from the backend with the page token → 200 with appointment date. Refresh grant from the backend → 200 (`expires_in 300`, `refresh_expires_in 1800`); refreshed token also works on `/prescrizione`; a second chained refresh → 200. Captcha was solved once in the browser before these calls.
- FSE: `fse-ricette-visite/listanew` from the backend → 200; refresh grant → 200; refreshed token → 200.
- `accessType=3` = Numero Ricetta, verified live. `1` = PIN and `2` = Numero prenotazione CUP, read from the portal code.
- Conclusion: backend HTTP client + in-memory tokens + refresh loop works for both portals. The login window is needed only for SPID and the Gestione prenotazioni captcha/tessera step; it can close afterwards.

## Open questions

- Session max lifetime (Keycloak SSO max) — how long the refresh chain keeps working.
- Whether `/prescrizione` from the backend still works in a new session where the captcha step was skipped (not tested on purpose: the app always lets the user do the captcha step).
- ZeroCode auth and endpoints with a real session.

## Stack

- Backend: Go (stdlib `net/http`, `log/slog`), `playwright-go` for the login window, Postgres via `pgx`/`sqlx`.
- Frontend: React + Vite + TypeScript, served by the backend on localhost.
