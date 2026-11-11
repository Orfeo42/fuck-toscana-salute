# fuck-toscana-salute

A usable, local-only dashboard over the Regione Toscana CUP and FSE portals: visits, exams and drug prescriptions in sortable tables, with appointments, expiry, PDFs and Google Calendar events. It calls the regional APIs live and keeps no copy of health data; the session lives in the system keyring. Design notes are in [docs/spec.md](docs/spec.md).

## Run

```sh
just build
FTS_ADDR=127.0.0.1:8799 ./bin/fts
```

Open http://127.0.0.1:8799 and press **Accedi**: the SPID login runs in a separate browser window owned by the app. The server only listens on loopback.

## Google Calendar (optional)

Adding an appointment to Google Calendar, with the booking promemoria PDF attached, needs your own OAuth client. Google does not allow shipping one in a public repository.

1. Create a project at https://console.cloud.google.com.
2. In **APIs & Services → Library**, enable **Google Calendar API** and **Google Drive API**.
3. In **Google Auth Platform**, configure the consent screen as **External** and add the scopes `https://www.googleapis.com/auth/calendar.events` and `https://www.googleapis.com/auth/drive.file`. `drive.file` only grants access to files this app creates.
4. In **Audience**, press **Publish app**. While the app is in *Testing*, Google expires the refresh token after 7 days. The first login then shows an "unverified app" warning: continue through *Advanced*.
5. In **Clients**, create a client of type **Desktop app**, download its JSON and save it as `~/.config/fuck-toscana-salute/google-client.json` (or point `FTS_GOOGLE_CLIENT_FILE` at it). Keep it private and out of the repository.
6. Restart the app and press **Collega Google Calendar** in the session bar.

The Google token is stored in the system keyring. **Scollega** removes it and revokes it at Google.
