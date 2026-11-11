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
  reconnect: boolean;
  configure: boolean;
}

const notConfiguredMessage = "Google non è configurato";

const genericFailure: Failure = {
  message: "Google o il portale regionale non rispondono. Riprova tra poco.",
  reconnect: false,
  configure: false,
};

function failureFor(
  status: number | undefined,
  detail: string | undefined,
): Failure {
  if (status === 401) {
    return {
      message: "Collegamento Google scaduto: ricollega.",
      reconnect: true,
      configure: false,
    };
  }
  if (status === 404) {
    return {
      message: "Promemoria non disponibile: riprova senza allegato.",
      reconnect: false,
      configure: false,
    };
  }
  if (status === 409 && detail === "google_not_configured") {
    return {
      message: notConfiguredMessage,
      reconnect: false,
      configure: true,
    };
  }
  if (status === 409) {
    return {
      message: "Serve l'accesso a Gestione Prenotazioni: premi Accedi.",
      reconnect: false,
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
  const { status, connect, openSettings } = useGoogle();
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

  const handleConnect = (): void => {
    void connect();
  };

  const handleAdd = async (): Promise<void> => {
    if (at === undefined) {
      return;
    }
    setBusy(true);
    setFailure(undefined);
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
    <Button variant="default" size="compact-sm" onClick={openSettings}>
      Configura
    </Button>
  );

  const connectButton = (
    <Button variant="default" size="compact-sm" onClick={handleConnect}>
      Collega Google Calendar
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
        {configured && !connected ? <Group>{connectButton}</Group> : null}
        {failure === undefined ? null : (
          <Alert color="red" role="alert">
            <Group gap="xs">
              <Text size="sm">{failure.message}</Text>
              {failure.reconnect ? connectButton : null}
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
                disabled={!connected}
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
