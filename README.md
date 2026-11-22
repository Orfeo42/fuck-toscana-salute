# fuck-toscana-salute

A usable, local-only dashboard over the Regione Toscana CUP and FSE portals: visits, exams and drug prescriptions in sortable tables, with appointments, expiry, PDFs and Google Calendar events. Responses are cached in an encrypted Postgres database (see the spec) and the session lives in its encrypted secrets table. Design notes are in [docs/spec.md](docs/spec.md).

## Run

```sh
just up
```

This starts the app and its Postgres (`docker compose`), creating the encryption key and the database password under `~/.config/fuck-toscana-salute/` on first run. Open http://127.0.0.1:8800 and press **Accedi**: the SPID login runs in a separate browser window owned by the app.

## Google Calendar and Drive import (optional)

Adding an appointment to Google Calendar, with the booking promemoria PDF attached, and importing reports from Google Drive through the Google Picker need your own OAuth client. Google does not allow shipping one in a public repository.

1. Create a project at https://console.cloud.google.com.
2. In **APIs & Services → Library**, enable **Google Calendar API**, **Google Drive API** and **Google Picker API**.
3. In **Google Auth Platform**, configure the consent screen as **External** and add the scopes `https://www.googleapis.com/auth/calendar.events` and `https://www.googleapis.com/auth/drive.file`. `drive.file` only grants access to files this app creates or that you pick in the Google Picker.
4. In **Audience**, either add your Google account as a test user (*Testing*: Google expires the authorization after 7 days and the app simply shows the Google popup again on the next click) or press **Publish app** (no expiry; the first sign-in shows an "unverified app" warning: continue through *Advanced*).
5. In **Clients**, create a client of type **Web application** and add the **Authorized JavaScript origin** shown in Impostazioni Google (`javascriptOrigin` from `GET /api/google/settings`, e.g. `http://127.0.0.1:8800`). No redirect URI is needed. Keep the client secret private.
6. In **APIs & Services → Credentials**, create an **API key** (only needed for Drive import). Set the application restriction to **Websites** with `http://127.0.0.1:8800/*` and `https://docs.google.com/*`, and the API restriction to **Google Picker API**.
7. Enter the client id and secret and the API key in Impostazioni Google (`PUT /api/google/settings`, fields `clientId`, `clientSecret`, `apiKey`). The Picker App ID is taken from the client id (its numeric prefix is the project number).

There is no separate connect step: the first click on **Aggiungi** (Calendar) or **Scegli da Google Drive** opens the Google sign-in popup, then the action continues. Allow popups for the app if the browser blocks them. **Accedi con Google** in the session menu signs in ahead of time.

The settings and the Google token are stored encrypted in the database. **Scollega** removes the token and revokes it at Google.
