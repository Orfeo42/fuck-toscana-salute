import {
  Alert,
  Anchor,
  Checkbox,
  Group,
  Modal,
  Loader,
  Stack,
  TagsInput,
  Text,
} from "@mantine/core";
import {
  CalendarPlus,
  ExternalLink,
  RefreshCw,
  Settings,
  X,
} from "lucide-react";
import { useEffect, useState, type JSX } from "react";

import {
  api,
  type AppointmentBody,
  type CalendarEventBody,
} from "../../api/client";
import { IconAction } from "../../components/IconAction";
import { formatDate, formatTime } from "../../lib/format";
import { useGoogle } from "../../google/useGoogle";
import { calendarEventFields } from "./googleCalendar";

interface Props {
  appointment: AppointmentBody;
  title: string;
  onClose: () => void;
}

interface Failure {
  message: string;
  configure: boolean;
}

const notConfiguredMessage = "Google non è configurato";

const genericFailure: Failure = {
  message: "Google o il portale regionale non rispondono. Riprova tra poco.",
  configure: false,
};

function failureFor(
  status: number | undefined,
  detail: string | undefined,
): Failure {
  if (status === 401) {
    return {
      message: "Accesso a Google scaduto: premi di nuovo Aggiungi",
      configure: false,
    };
  }
  if (status === 422 && detail !== undefined) {
    return { message: detail, configure: false };
  }
  if (status === 404) {
    return {
      message: "Promemoria non disponibile: riprova senza allegato.",
      configure: false,
    };
  }
  if (status === 409 && detail === "google_not_configured") {
    return {
      message: notConfiguredMessage,
      configure: true,
    };
  }
  if (status === 409) {
    return {
      message: "Serve l'accesso a Gestione Prenotazioni: premi Accedi.",
      configure: false,
    };
  }
  return genericFailure;
}

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

type Lookup =
  | { state: "checking" }
  | { state: "none" }
  | { state: "exists"; htmlLink: string };

export function CalendarDialog({
  appointment,
  title,
  onClose,
}: Props): JSX.Element {
  const {
    status,
    error: googleError,
    signIn,
    refresh,
    openSettings,
  } = useGoogle();
  const { at, bookingId } = appointment;
  const fields = calendarEventFields(appointment, title);
  const hasBooking = bookingId !== undefined && bookingId !== "";
  const configured = status?.configured === true;
  const connected = configured && status.connected;
  const initialLookup: Lookup = connected
    ? { state: "checking" }
    : { state: "none" };
  const [attach, setAttach] = useState(true);
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<Failure | undefined>(undefined);
  const [result, setResult] = useState<CalendarEventBody | undefined>(
    undefined,
  );
  const [lookup, setLookup] = useState<Lookup>(initialLookup);
  const [guests, setGuests] = useState<string[]>([]);
  const [guestsLoaded, setGuestsLoaded] = useState(false);

  useEffect(() => {
    if (!connected) {
      return;
    }
    const controller = new AbortController();
    const { signal } = controller;
    const checkEvent = async (): Promise<void> => {
      try {
        const { data, response } = await api.POST(
          "/api/calendar/events/lookup",
          { body: { appointmentId: appointment.id }, signal },
        );
        if (signal.aborted) {
          return;
        }
        if (data === undefined) {
          if (response.status === 401) {
            refresh();
          }
          setLookup({ state: "none" });
          return;
        }
        setLookup(
          data.exists
            ? { state: "exists", htmlLink: data.htmlLink }
            : { state: "none" },
        );
      } catch {
        if (!signal.aborted) {
          setLookup({ state: "none" });
        }
      }
    };
    const loadGuests = async (): Promise<void> => {
      try {
        const { data } = await api.GET("/api/google/settings", { signal });
        if (signal.aborted || data === undefined) {
          return;
        }
        setGuests(data.guests);
        setGuestsLoaded(true);
      } catch {
        return;
      }
    };
    void checkEvent();
    void loadGuests();
    return () => {
      controller.abort();
    };
  }, [connected, appointment.id, refresh]);

  const handleConfigure = (): void => {
    onClose();
    openSettings();
  };

  const handleAdd = async (regenerate: boolean): Promise<void> => {
    if (at === undefined) {
      return;
    }
    setBusy(true);
    setFailure(undefined);
    if (!connected) {
      const signedIn = await signIn();
      if (!signedIn) {
        setBusy(false);
        return;
      }
    }
    try {
      const { data, error, response } = await api.POST("/api/calendar/events", {
        body: {
          appointmentId: appointment.id,
          title: fields.title,
          start: at,
          durationMinutes: 60,
          location: fields.location,
          details: fields.details,
          ...(hasBooking ? { bookingId } : {}),
          attachPromemoria: hasBooking && attach,
          ...(guestsLoaded ? { guests } : {}),
          ...(regenerate ? { regenerate: true } : {}),
        },
      });
      if (data === undefined) {
        if (response.status === 401) {
          refresh();
        }
        setFailure(failureFor(response.status, error.detail));
        return;
      }
      setResult(data);
    } catch {
      setFailure(genericFailure);
    } finally {
      setBusy(false);
    }
  };

  const checking = lookup.state === "checking";
  const existingLink = lookup.state === "exists" ? lookup.htmlLink : undefined;
  const guestsInvalid = guests.some((guest) => !emailPattern.test(guest));

  const configureButton = (
    <IconAction label="Configura" icon={Settings} onClick={handleConfigure} />
  );

  return (
    <Modal opened onClose={onClose} title="Aggiungi a Google Calendar" centered>
      <Stack gap="sm">
        <Stack gap={4}>
          <Text>
            {formatDate(at)} {formatTime(at)}
          </Text>
          <Text>{fields.title}</Text>
          <Text>{fields.location}</Text>
        </Stack>
        {hasBooking ? (
          <Checkbox
            label="Allega promemoria (PDF)"
            checked={attach}
            onChange={(event) => {
              setAttach(event.currentTarget.checked);
            }}
          />
        ) : null}
        <Stack gap={4}>
          <TagsInput
            label="Invitati"
            placeholder="email e Invio"
            value={guests}
            onChange={setGuests}
            maxTags={10}
            splitChars={[",", " ", ";"]}
            error={guestsInvalid ? "Email non valida" : undefined}
          />
          <Text size="xs" c="dimmed">
            Gli invitati non ricevono email; possono aprire il promemoria
            allegato.
          </Text>
        </Stack>
        {checking ? (
          <Group gap="xs">
            <Loader size="xs" />
            <Text size="sm">Controllo il calendario…</Text>
          </Group>
        ) : null}
        {existingLink === undefined || result !== undefined ? null : (
          <Alert color="teal" role="status" title="Già nel calendario" />
        )}
        {status !== undefined && !configured ? (
          <Group gap="xs">
            <Text size="sm">{notConfiguredMessage}</Text>
            {configureButton}
          </Group>
        ) : null}
        {googleError === undefined ||
        connected ||
        failure !== undefined ? null : (
          <Alert color="red" role="alert">
            <Text size="sm">{googleError}</Text>
          </Alert>
        )}
        {failure === undefined ? null : (
          <Alert color="red" role="alert">
            <Group gap="xs">
              <Text size="sm">{failure.message}</Text>
              {failure.configure ? configureButton : null}
            </Group>
          </Alert>
        )}
        {result === undefined ? null : (
          <Alert color="teal" role="status">
            <Text size="sm">
              {result.created
                ? "Aggiunto al calendario"
                : "Già presente nel calendario"}{" "}
              <Anchor
                href={result.htmlLink}
                target="_blank"
                rel="noopener noreferrer"
              >
                Apri in Google Calendar
              </Anchor>
            </Text>
          </Alert>
        )}
        <Group justify="flex-end" gap="xs">
          {result !== undefined ? (
            <IconAction label="Chiudi" icon={X} onClick={onClose} />
          ) : existingLink !== undefined ? (
            <>
              <IconAction
                label="Apri in Google Calendar"
                icon={ExternalLink}
                href={existingLink}
                external
              />
              <IconAction
                label="Rigenera"
                icon={RefreshCw}
                disabled={guestsInvalid}
                loading={busy}
                onClick={() => {
                  void handleAdd(true);
                }}
              />
              <IconAction label="Chiudi" icon={X} onClick={onClose} />
            </>
          ) : (
            <>
              <IconAction label="Annulla" icon={X} onClick={onClose} />
              <IconAction
                label="Aggiungi"
                icon={CalendarPlus}
                tone="primary"
                disabled={!configured || checking || guestsInvalid}
                loading={busy}
                onClick={() => {
                  void handleAdd(false);
                }}
              />
            </>
          )}
        </Group>
      </Stack>
    </Modal>
  );
}
