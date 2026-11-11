import { useState, type JSX } from "react";

import type { AppointmentBody } from "../api/client";
import { failureLabel } from "../lib/status";
import { AppointmentCard } from "./appointments/AppointmentCard";
import { PastRow } from "./appointments/PastRow";
import {
  useAppointments,
  type AppointmentsError,
} from "./appointments/useAppointments";
import "./appointments/appointments.css";

interface Props {
  now?: Date;
}

const errorMessages: Readonly<Record<AppointmentsError, string>> = {
  unauthorized: "Accedi per vedere gli appuntamenti.",
  needs_gestione:
    "Per vedere date e orari serve l'accesso a Gestione Prenotazioni: premi Accedi.",
  unavailable: "Il portale regionale non risponde. Riprova tra poco.",
};

function time(appointment: AppointmentBody): number {
  return appointment.at === undefined
    ? Number.POSITIVE_INFINITY
    : new Date(appointment.at).getTime();
}

export function AppointmentsView({ now }: Props = {}): JSX.Element {
  const [current] = useState(() => now ?? new Date());
  const { data, error, loading, reload } = useAppointments();

  const all = data?.appointments ?? [];
  const failed = data?.failed ?? [];
  const isUpcoming = (a: AppointmentBody): boolean =>
    a.status !== "DISDETTO" && time(a) >= current.getTime();
  const upcoming = all.filter(isUpcoming).sort((a, b) => time(a) - time(b));
  const past = all
    .filter((a) => !isUpcoming(a))
    .sort((a, b) => time(b) - time(a));

  return (
    <section aria-busy={loading} aria-labelledby="appointments-title">
      <h2 id="appointments-title">Appuntamenti</h2>
      <button type="button" className="secondary" onClick={reload}>
        Aggiorna
      </button>
      {loading && (
        <p role="status">Cerco gli appuntamenti… (qualche secondo)</p>
      )}
      {error !== undefined && (
        <p role="alert" className="appointments-error">
          {errorMessages[error]}
        </p>
      )}
      {data !== undefined && (
        <>
          <div className="appointments-section">
            <h3>Prossimi appuntamenti ({upcoming.length})</h3>
            {upcoming.length === 0 ? (
              <p>Nessun appuntamento in programma.</p>
            ) : (
              <ul className="appointments-list">
                {upcoming.map((a) => (
                  <AppointmentCard key={a.id} appointment={a} now={current} />
                ))}
              </ul>
            )}
          </div>
          <div className="appointments-section">
            <details>
              <summary>Passati e disdetti ({past.length})</summary>
              <ul className="appointments-list">
                {past.map((a) => (
                  <PastRow key={a.id} appointment={a} />
                ))}
              </ul>
            </details>
          </div>
          {failed.length > 0 && (
            <div className="appointments-section">
              <details>
                <summary>
                  Prenotazioni non trovate nel CUP regionale ({failed.length})
                </summary>
                <ul className="appointments-list">
                  {failed.map((f) => (
                    <li key={f.nre}>
                      <strong>{f.nre}</strong>: {failureLabel(f.reason)}
                      <p className="appointments-hint">
                        Controlla l&apos;SMS o il promemoria ricevuto alla
                        prenotazione.
                      </p>
                    </li>
                  ))}
                </ul>
              </details>
            </div>
          )}
        </>
      )}
    </section>
  );
}
