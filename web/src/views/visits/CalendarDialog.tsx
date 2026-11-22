import {
  Alert,
  Anchor,
  Button,
  Checkbox,
  Group,
  Modal,
  Stack,
  Text,
} from "@mantine/core";
import { useState, type JSX } from "react";

import {
  api,
  type AppointmentBody,
  type CalendarEventBody,
} from "../../api/client";
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
  const [attach, setAttach] = useState(true);
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<Failure | undefined>(undefined);
  const [result, setResult] = useState<CalendarEventBody | undefined>(
    undefined,
  );

  const { at, bookingId } = appointment;
  const fields = calendarEventFields(appointment, title);
  const hasBooking = bookingId !== undefined && bookingId !== "";
  const configured = status?.configured === true;
  const connected = configured && status.connected;

  const handleConfigure = (): void => {
    onClose();
    openSettings();
  };

  const handleAdd = async (): Promise<void> => {
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

  const configureButton = (
    <Button variant="default" size="compact-sm" onClick={handleConfigure}>
      Configura
    </Button>
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
        <Group justify="flex-end">
          {result === undefined ? (
            <>
              <Button variant="default" onClick={onClose}>
                Annulla
              </Button>
              <Button
                disabled={!configured}
                loading={busy}
                onClick={() => {
                  void handleAdd();
                }}
              >
                Aggiungi
              </Button>
            </>
          ) : (
            <Button onClick={onClose}>Chiudi</Button>
          )}
        </Group>
      </Stack>
    </Modal>
  );
}
