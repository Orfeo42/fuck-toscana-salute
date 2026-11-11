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
| Appointment lookup (dates) | `GET {cupBe}/prescrizione?numbersTeam=<tessera>&accessType=<type>&accessCode=<code uppercase>` headers `cf`, `Authorization` | Verified 200 for a booking made in the portal. `response.listaPrescrizioni.prescrizione[].listaAppuntamenti.appuntamento[]{idAppuntamento, statoAppuntamento ("PRENOTATO", "DISDETTO"...), dataAppuntamento, oraAppuntamento, unita{descUnita, nomeUnita, indirizzoUnita, nomeComuneUnita, provinciaUnita, erogatore{...}, sede{...}}}`. NRE with no CUP booking → HTTP 500 + Java exception ("Non esistono richieste per il paziente" in the UI). accessType values: PIN / numero prenotazione CUP / numero ricetta — exact codes TBD. |
| Promemoria PDF | `GET {cupBe}/promemoria?idPrescrizione=<id>` | untested |
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

- `GET /api/reports` → `{reports[]{kind, kindLabel, abstractId, detailId, repCode, date, updatedAt, provider, unread, hidden}, failures[]{kind, kindLabel, message}, updated}`; a kind that fails is reported in `failures` and does not hide the others.
- `POST /api/reports/documents` body `{kind, abstractId, detailId, repCode}` → `{documents[]{position, label, acceptedAt?, provider?}}`.
- `POST /api/reports/pdf` body `{kind, abstractId, detailId, repCode, position}` → `application/pdf`.

### Google settings

The OAuth client is configured from the UI, not from a file: `GET /api/google/settings` → `{clientId, calendarId, driveFolderId, hasSecret, redirectUri}` (the secret is never returned); `PUT /api/google/settings` body `{clientId, clientSecret?, calendarId?, driveFolderId?}` (an empty secret keeps the stored one; changing the client id drops the stored token) is stored encrypted in the `secrets` table. The redirect URI is built from the request (`X-Forwarded-Proto` or `http`, plus `Host`) with path `/api/google/callback` and must be registered in the Google client.

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
- `accessType=3` = Numero Ricetta. PIN / Numero prenotazione CUP codes still unknown (likely 1 / 2).
- Conclusion: backend HTTP client + in-memory tokens + refresh loop works for both portals. The login window is needed only for SPID and the Gestione prenotazioni captcha/tessera step; it can close afterwards.

## Open questions

- Session max lifetime (Keycloak SSO max) — how long the refresh chain keeps working.
- Whether `/prescrizione` from the backend still works in a new session where the captcha step was skipped (not tested on purpose: the app always lets the user do the captcha step).
- `accessType` codes for PIN and Numero prenotazione CUP.
- How to enumerate appointments: per-NRE lookup over all prescriptions (FSE + CUP lists), and handling the HTTP 500 = "no booking".
- ZeroCode auth and endpoints with a real session.

## Stack

- Backend: Go (stdlib `net/http`, `log/slog`), `playwright-go` for the login window, Postgres via `pgx`/`sqlx`.
- Frontend: React + Vite + TypeScript, served by the backend on localhost.
