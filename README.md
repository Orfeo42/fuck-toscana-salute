# fuck-toscana-salute

A usable, local-only dashboard over the Regione Toscana CUP and FSE portals: visits, exams and drug prescriptions in sortable tables, with appointments, expiry, PDFs and Google Calendar events. Responses are cached in an encrypted Postgres database (see the spec) and the session lives in its encrypted secrets table. Design notes are in [docs/spec.md](docs/spec.md).

## Run

```sh
just up
```

This starts the app and its Postgres (`docker compose`), creating the encryption key and the database password under `~/.config/fuck-toscana-salute/` on first run. Open http://127.0.0.1:8800 and press **Accedi**: the SPID login runs in a separate browser window owned by the app.

## Google Calendar (optional)

Adding an appointment to Google Calendar, with the booking promemoria PDF attached, needs your own OAuth client. Google does not allow shipping one in a public repository.

1. Create a project at https://console.cloud.google.com.
2. In **APIs & Services → Library**, enable **Google Calendar API** and **Google Drive API**.
3. In **Google Auth Platform**, configure the consent screen as **External** and add the scopes `https://www.googleapis.com/auth/calendar.events` and `https://www.googleapis.com/auth/drive.file`. `drive.file` only grants access to files this app creates.
4. In **Audience**, press **Publish app**. While the app is in *Testing*, Google expires the refresh token after 7 days. The first login then shows an "unverified app" warning: continue through *Advanced*.
5. In **Clients**, create a client of type **Web application** and add the redirect URI shown by `GET /api/google/settings` (`<app origin>/api/google/callback`). Keep the client secret private.
6. Enter the client id and secret through `PUT /api/google/settings`, then press **Collega Google Calendar** in the session bar.

The settings and the Google token are stored encrypted in the database. **Scollega** removes the token and revokes it at Google.
