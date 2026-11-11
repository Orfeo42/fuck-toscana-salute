import { useEffect, useRef, useState, type JSX } from "react";

import {
  api,
  type AppointmentBody,
  type CalendarEventBody,
} from "../../api/client";
import { formatDate, formatTime } from "../../lib/format";
import { notConfiguredHint, useGoogle } from "../../google/useGoogle";
import { calendarEventFields } from "./googleCalendar";

interface Props {
  appointment: AppointmentBody;
  title: string;
  onClose: () => void;
}

interface Failure {
  message: string;
  reconnect: boolean;
}

const genericFailure: Failure = {
  message: "Google o il portale regionale non rispondono. Riprova tra poco.",
  reconnect: false,
};

function failureFor(
  status: number | undefined,
  detail: string | undefined,
): Failure {
  if (status === 401) {
    return {
      message: "Collegamento Google scaduto: ricollega.",
      reconnect: true,
    };
  }
  if (status === 404) {
    return {
      message: "Promemoria non disponibile: riprova senza allegato.",
      reconnect: false,
    };
  }
  if (status === 409 && detail === "google_not_configured") {
    return { message: notConfiguredHint, reconnect: false };
  }
  if (status === 409) {
    return {
      message: "Serve l'accesso a Gestione Prenotazioni: premi Accedi.",
      reconnect: false,
    };
  }
  return genericFailure;
}

export function CalendarDialog({
  appointment,
  title,
  onClose,
}: Props): JSX.Element {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const { status, connect } = useGoogle();
  const [attach, setAttach] = useState(true);
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<Failure | undefined>(undefined);
  const [result, setResult] = useState<CalendarEventBody | undefined>(
    undefined,
  );

  useEffect(() => {
    dialogRef.current?.showModal();
  }, []);

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

  const connectButton = (
    <button type="button" onClick={handleConnect}>
      Collega Google Calendar
    </button>
  );

  return (
    <dialog
      ref={dialogRef}
      className="calendar-dialog"
      aria-labelledby="calendar-dialog-title"
      onClose={onClose}
    >
      <h2 id="calendar-dialog-title">Aggiungi a Google Calendar</h2>
      <p>
        {formatDate(at)} {formatTime(at)}
      </p>
      <p>{fields.title}</p>
      <p>{fields.location}</p>
      {hasBooking ? (
        <label>
          <input
            type="checkbox"
            checked={attach}
            onChange={(event) => {
              setAttach(event.target.checked);
            }}
          />{" "}
          Allega promemoria (PDF)
        </label>
      ) : null}
      {status !== undefined && !configured ? <p>{notConfiguredHint}</p> : null}
      {configured && !connected ? <p>{connectButton}</p> : null}
      {failure === undefined ? null : (
        <p role="alert">
          {failure.message}
          {failure.reconnect ? connectButton : null}
        </p>
      )}
      {result === undefined ? null : (
        <p>
          {result.created
            ? "Aggiunto al calendario"
            : "Già presente nel calendario"}{" "}
          <a href={result.htmlLink} target="_blank" rel="noopener noreferrer">
            Apri in Google Calendar
          </a>
        </p>
      )}
      <div className="calendar-dialog-actions">
        {result === undefined ? (
          <>
            <button
              type="button"
              disabled={!connected || busy}
              aria-busy={busy}
              onClick={() => {
                void handleAdd();
              }}
            >
              Aggiungi
            </button>
            <button type="button" className="secondary" onClick={onClose}>
              Annulla
            </button>
          </>
        ) : (
          <button type="button" onClick={onClose}>
            Chiudi
          </button>
        )}
      </div>
    </dialog>
  );
}
