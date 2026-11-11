import {
  Alert,
  Button,
  Group,
  Loader,
  Modal,
  PasswordInput,
  Stack,
  Text,
  TextInput,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
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
import { useGoogle } from "./useGoogle";

const secretPlaceholder = "Già salvato — lascia vuoto per non cambiarlo";
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

function GoogleSettingsForm({ settings, onClose }: FormProps): JSX.Element {
  const { refresh } = useGoogle();
  const [clientId, setClientId] = useState(settings.clientId);
  const [clientSecret, setClientSecret] = useState("");
  const [calendarId, setCalendarId] = useState(
    settings.calendarId === "" ? "primary" : settings.calendarId,
  );
  const [driveFolderId, setDriveFolderId] = useState(settings.driveFolderId);
  const [clientIdError, setClientIdError] = useState<string | undefined>(
    undefined,
  );
  const [secretError, setSecretError] = useState<string | undefined>(undefined);
  const [formError, setFormError] = useState<string | undefined>(undefined);
  const [saving, setSaving] = useState(false);

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
      return;
    }
    const trimmedCalendarId = calendarId.trim();
    const trimmedFolder = driveFolderId.trim();
    const body: GoogleSettingsRequest = {
      clientId: trimmedClientId,
      ...(trimmedSecret === "" ? {} : { clientSecret: trimmedSecret }),
      ...(trimmedCalendarId === "" ? {} : { calendarId: trimmedCalendarId }),
      ...(trimmedFolder === "" ? {} : { driveFolderId: trimmedFolder }),
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
        <Text size="sm" c="dimmed">
          Su Google Cloud Console crea un client OAuth di tipo
          &quot;Applicazione web&quot;, abilita le API Calendar e Drive e
          aggiungi il redirect URI indicato qui sotto.
        </Text>
        <Stack gap={4}>
          <Text size="sm" fw={500}>
            Redirect URI da registrare su Google Cloud
          </Text>
          <CopyButton value={settings.redirectUri} label="Copia redirect URI" />
        </Stack>
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
        <TextInput
          label="Calendario"
          description="ID del calendario, primary = calendario principale"
          value={calendarId}
          onChange={(event) => {
            setCalendarId(event.currentTarget.value);
          }}
        />
        <TextInput
          label="Cartella Drive"
          description="ID della cartella dove salvare i PDF; vuoto = cartella principale"
          value={driveFolderId}
          onChange={(event) => {
            setDriveFolderId(event.currentTarget.value);
          }}
        />
        {formError === undefined ? null : (
          <Text size="sm" c="red" role="alert">
            {formError}
          </Text>
        )}
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Annulla
          </Button>
          <Button type="submit" loading={saving}>
            Salva
          </Button>
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
