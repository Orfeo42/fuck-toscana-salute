import {
  Alert,
  Anchor,
  Badge,
  Group,
  List,
  Loader,
  Modal,
  PasswordInput,
  Stack,
  Tabs,
  TagsInput,
  Text,
  TextInput,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { Check, LogIn, LogOut, X } from "lucide-react";
import {
  useEffect,
  useState,
  type JSX,
  type SubmitEvent as FormSubmitEvent,
} from "react";

import {
  api,
  type GoogleSettingsBody,
  type GoogleSettingsRequest,
} from "../api/client";
import { CopyButton } from "../components/CopyButton";
import { IconAction } from "../components/IconAction";
import { driveFolderId } from "../lib/driveFolder";
import {
  googleConsoleLinkLabels,
  googleConsoleLinks,
} from "./googleConsoleLinks";
import { useGoogle } from "./useGoogle";

const secretPlaceholder = "Salvato — lascia vuoto per non cambiarlo";
const saveFailed = "Salvataggio non riuscito";
const loadFailed = "Impossibile leggere le impostazioni Google";

interface ProblemBody {
  detail?: string;
  errors?: { message?: string }[] | null;
}

function problemMessage(problem: ProblemBody | undefined): string {
  const messages = (problem?.errors ?? []).flatMap((item) =>
    item.message === undefined ? [] : [item.message],
  );
  if (messages.length > 0) {
    return messages.join(". ");
  }
  return problem?.detail ?? saveFailed;
}

interface FormProps {
  settings: GoogleSettingsBody;
  onClose: () => void;
}

interface StatusView {
  label: string;
  color: string;
}

const guestPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const maxGuests = 10;

function GoogleSettingsForm({ settings, onClose }: FormProps): JSX.Element {
  const {
    status,
    error: googleError,
    refresh,
    signIn,
    disconnect,
  } = useGoogle();
  const [tab, setTab] = useState<string | null>("account");
  const [clientId, setClientId] = useState(settings.clientId);
  const [clientSecret, setClientSecret] = useState("");
  const [calendarId, setCalendarId] = useState(
    settings.calendarId === "" ? "primary" : settings.calendarId,
  );
  const [guests, setGuests] = useState<string[]>(settings.guests);
  const [driveFolder, setDriveFolder] = useState(settings.driveFolderId);
  const [apiKey, setApiKey] = useState(settings.apiKey);
  const [clientIdError, setClientIdError] = useState<string | undefined>(
    undefined,
  );
  const [secretError, setSecretError] = useState<string | undefined>(undefined);
  const [formError, setFormError] = useState<string | undefined>(undefined);
  const [saving, setSaving] = useState(false);

  const guestsError = guests.every((guest) => guestPattern.test(guest))
    ? undefined
    : "Email non valida";
  const folderId = driveFolderId(driveFolder);
  const statusView: StatusView = !status?.configured
    ? { label: "Non configurato", color: "yellow" }
    : status.connected
      ? { label: "Collegato", color: "teal" }
      : { label: "Non collegato", color: "gray" };

  const submit = async (): Promise<void> => {
    const trimmedClientId = clientId.trim();
    const trimmedSecret = clientSecret.trim();
    const missingClientId = trimmedClientId === "";
    const missingSecret = trimmedSecret === "" && !settings.hasSecret;
    setClientIdError(
      missingClientId ? "Il Client ID è obbligatorio" : undefined,
    );
    setSecretError(
      missingSecret ? "Il client secret è obbligatorio" : undefined,
    );
    setFormError(undefined);
    if (missingClientId || missingSecret) {
      setTab("account");
      return;
    }
    const trimmedCalendarId = calendarId.trim();
    const trimmedApiKey = apiKey.trim();
    const body: GoogleSettingsRequest = {
      clientId: trimmedClientId,
      guests,
      ...(trimmedSecret === "" ? {} : { clientSecret: trimmedSecret }),
      ...(trimmedCalendarId === "" ? {} : { calendarId: trimmedCalendarId }),
      ...(folderId === "" ? {} : { driveFolderId: folderId }),
      ...(trimmedApiKey === "" ? {} : { apiKey: trimmedApiKey }),
    };
    setSaving(true);
    try {
      const { error } = await api.PUT("/api/google/settings", { body });
      if (error !== undefined) {
        setFormError(problemMessage(error));
        return;
      }
      setClientSecret("");
      refresh();
      notifications.show({
        message: "Impostazioni Google salvate",
        color: "teal",
      });
      onClose();
    } catch {
      setFormError(saveFailed);
    } finally {
      setSaving(false);
    }
  };

  const handleSubmit = (event: FormSubmitEvent<HTMLFormElement>): void => {
    event.preventDefault();
    void submit();
  };

  return (
    <form onSubmit={handleSubmit} noValidate>
      <Stack gap="sm">
        <Stack gap={4}>
          <Group justify="space-between">
            <Badge color={statusView.color}>{statusView.label}</Badge>
            {status?.configured && !status.connected ? (
              <IconAction
                label="Accedi con Google"
                icon={LogIn}
                tone="primary"
                onClick={() => {
                  void signIn();
                }}
              />
            ) : null}
            {status?.connected ? (
              <IconAction
                label="Scollega"
                icon={LogOut}
                onClick={() => {
                  void disconnect();
                }}
              />
            ) : null}
          </Group>
          {googleError === undefined ? null : (
            <Text size="sm" c="red">
              {googleError}
            </Text>
          )}
        </Stack>
        <Tabs value={tab} onChange={setTab}>
          <Tabs.List>
            <Tabs.Tab value="account">Account Google</Tabs.Tab>
            <Tabs.Tab value="calendar">Calendario</Tabs.Tab>
            <Tabs.Tab value="drive">Drive</Tabs.Tab>
            <Tabs.Tab value="guide">Guida</Tabs.Tab>
          </Tabs.List>
          <Tabs.Panel value="account" pt="sm">
            <Stack gap="sm">
              <Group gap="xs" role="group" aria-label="Link utili">
                <Text size="xs" c="dimmed">
                  Link utili:
                </Text>
                {googleConsoleLinkLabels.map((link) => (
                  <Anchor
                    key={link.key}
                    size="xs"
                    href={googleConsoleLinks[link.key]}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    {link.label}
                  </Anchor>
                ))}
              </Group>
              <TextInput
                label="Client ID"
                value={clientId}
                error={clientIdError}
                onChange={(event) => {
                  setClientId(event.currentTarget.value);
                }}
              />
              <PasswordInput
                label="Client secret"
                placeholder={settings.hasSecret ? secretPlaceholder : undefined}
                autoComplete="off"
                value={clientSecret}
                error={secretError}
                onChange={(event) => {
                  setClientSecret(event.currentTarget.value);
                }}
              />
              <Stack gap={4}>
                <Text size="sm">
                  Su Google Cloud → Clients crea un client Applicazione web e
                  aggiungi questa origine JavaScript autorizzata:
                </Text>
                <CopyButton
                  value={settings.javascriptOrigin}
                  label="Copia origine JavaScript"
                />
              </Stack>
              <Anchor
                size="sm"
                href={googleConsoleLinks.clients}
                target="_blank"
                rel="noopener noreferrer"
              >
                Apri Google Cloud Console
              </Anchor>
            </Stack>
          </Tabs.Panel>
          <Tabs.Panel value="calendar" pt="sm">
            <Stack gap="sm">
              <TextInput
                label="Calendario"
                description="ID del calendario, primary = calendario principale"
                value={calendarId}
                onChange={(event) => {
                  setCalendarId(event.currentTarget.value);
                }}
              />
              <TagsInput
                label="Invitati predefiniti"
                description="Aggiunti a ogni evento senza email; possono aprire il promemoria allegato"
                maxTags={maxGuests}
                splitChars={[",", " ", ";"]}
                value={guests}
                error={guestsError}
                onChange={setGuests}
              />
            </Stack>
          </Tabs.Panel>
          <Tabs.Panel value="drive" pt="sm">
            <Stack gap="sm">
              <TextInput
                label="Cartella Drive"
                description="Incolla il link della cartella o il suo ID; vuoto = cartella principale"
                value={driveFolder}
                onChange={(event) => {
                  setDriveFolder(event.currentTarget.value);
                }}
              />
              {folderId !== "" && folderId !== driveFolder ? (
                <Text size="xs" c="dimmed">
                  ID: {folderId}
                </Text>
              ) : null}
              <TextInput
                label="API key (Picker)"
                description="Chiave API per scegliere i referti da Drive (facoltativa)"
                autoComplete="off"
                value={apiKey}
                onChange={(event) => {
                  setApiKey(event.currentTarget.value);
                }}
              />
              <Anchor
                size="sm"
                href={googleConsoleLinks.credentials}
                target="_blank"
                rel="noopener noreferrer"
              >
                Crea chiave API
              </Anchor>
            </Stack>
          </Tabs.Panel>
          <Tabs.Panel value="guide" pt="sm">
            <List type="ordered" size="sm" spacing="xs">
              <List.Item>Crea un progetto su Google Cloud.</List.Item>
              <List.Item>
                Abilita Google Calendar API, Google Drive API e Google Picker
                API dalla{" "}
                <Anchor
                  size="sm"
                  href={googleConsoleLinks.library}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  libreria
                </Anchor>
                .
              </List.Item>
              <List.Item>
                Google Auth Platform → Get started, tipo External.
              </List.Item>
              <List.Item>
                <Anchor
                  size="sm"
                  href={googleConsoleLinks.scopes}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Accesso ai dati
                </Anchor>
                : aggiungi gli ambiti calendar.events e drive.file.
              </List.Item>
              <List.Item>
                <Anchor
                  size="sm"
                  href={googleConsoleLinks.audience}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Pubblico
                </Anchor>
                : aggiungi il tuo account come utente di test, oppure pubblica
                l&apos;app (altrimenti Google risponde 403 access_denied).
              </List.Item>
              <List.Item>
                <Anchor
                  size="sm"
                  href={googleConsoleLinks.clients}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Client
                </Anchor>
                : crea un client Applicazione web con l&apos;origine JavaScript
                mostrata in Account Google.
              </List.Item>
              <List.Item>
                <Anchor
                  size="sm"
                  href={googleConsoleLinks.credentials}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Credenziali
                </Anchor>
                : crea una chiave API limitata a Google Picker API e ai siti
                &lt;origine&gt;/* e https://docs.google.com/*.
              </List.Item>
              <List.Item>
                Incolla qui client ID, secret e chiave API, poi premi Salva.
              </List.Item>
            </List>
          </Tabs.Panel>
        </Tabs>
        {formError === undefined ? null : (
          <Text size="sm" c="red" role="alert">
            {formError}
          </Text>
        )}
        <Group justify="flex-end" gap="xs">
          <IconAction label="Annulla" icon={X} onClick={onClose} />
          <IconAction
            label="Salva"
            icon={Check}
            tone="primary"
            type="submit"
            loading={saving}
            disabled={guestsError !== undefined}
          />
        </Group>
      </Stack>
    </form>
  );
}

function GoogleSettingsLoader({
  onClose,
}: {
  onClose: () => void;
}): JSX.Element {
  const [settings, setSettings] = useState<GoogleSettingsBody | undefined>(
    undefined,
  );
  const [loadError, setLoadError] = useState<string | undefined>(undefined);

  useEffect(() => {
    let cancelled = false;
    const load = async (): Promise<void> => {
      try {
        const { data } = await api.GET("/api/google/settings");
        if (cancelled) {
          return;
        }
        if (data === undefined) {
          setLoadError(loadFailed);
          return;
        }
        setSettings(data);
      } catch {
        if (!cancelled) {
          setLoadError(loadFailed);
        }
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, []);

  if (loadError !== undefined) {
    return (
      <Alert color="red" role="alert">
        {loadError}
      </Alert>
    );
  }
  if (settings === undefined) {
    return <Loader size="sm" aria-label="Caricamento impostazioni" />;
  }
  return <GoogleSettingsForm settings={settings} onClose={onClose} />;
}

interface Props {
  opened: boolean;
  onClose: () => void;
}

export function GoogleSettingsModal({ opened, onClose }: Props): JSX.Element {
  return (
    <Modal opened={opened} onClose={onClose} title="Configura Google">
      <GoogleSettingsLoader onClose={onClose} />
    </Modal>
  );
}
