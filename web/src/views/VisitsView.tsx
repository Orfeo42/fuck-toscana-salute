import { useState } from "react";
import type { JSX } from "react";

import { SortHeader } from "../components/SortHeader";
import { useHealthData } from "../data/useHealthData";
import { useRoute } from "../lib/route";
import { useSort } from "../lib/useSort";
import { VisitRow } from "./visits/VisitRow";
import {
  buildRows,
  countByStatus,
  effectiveState,
  filterRows,
  sortRows,
  type StatusFilter,
  type VisitSortKey,
} from "./visits/filter";
import "./visits/visits.css";

const statusOptions: readonly { value: StatusFilter; label: string }[] = [
  { value: "upcoming", label: "Prossimi" },
  { value: "bookable", label: "Da prenotare" },
  { value: "open", label: "Aperte" },
  { value: "all", label: "Tutte" },
];

const descFirst: readonly VisitSortKey[] = ["prescritta"];

interface Props {
  now?: Date;
}

export function VisitsView({ now }: Props = {}): JSX.Element {
  const health = useHealthData();
  const { prescriptions, appointments, bookingByNre, notInCup } = health;
  const route = useRoute();
  const focus = route.tab === "visite" ? route.focus : undefined;
  const [mountedAt] = useState(() => new Date());
  const [handledFocus, setHandledFocus] = useState<string | undefined>(
    undefined,
  );
  const [status, setStatus] = useState<StatusFilter>("open");
  const [search, setSearch] = useState("");
  const sort = useSort<VisitSortKey>("appuntamento", "asc", descFirst);

  const currentNow = now ?? mountedAt;
  const context = { bookingByNre, notInCup, now: currentNow };
  const data = buildRows(
    prescriptions.data?.prescriptions ?? [],
    appointments.data?.appointments ?? [],
  );
  const counts = countByStatus(data, search, context);
  const visible = sortRows(
    filterRows(data, { status, search }, context),
    sort.key,
    sort.dir,
    context,
  );

  if (
    prescriptions.data !== undefined &&
    !appointments.loading &&
    focus !== handledFocus
  ) {
    setHandledFocus(focus);
    const exists = data.some((row) => row.nre.toUpperCase() === focus);
    const shown = visible.some((row) => row.nre.toUpperCase() === focus);
    if (exists && !shown) {
      setStatus("all");
      setSearch("");
    }
  }

  const reload = (): void => {
    prescriptions.reload();
    appointments.reload();
  };

  const summary = `${String(visible.length)} visite/esami`;
  const ready = !prescriptions.loading && prescriptions.error === undefined;

  return (
    <section aria-labelledby="visits-title" aria-busy={prescriptions.loading}>
      <h2 id="visits-title">Visite ed esami</h2>
      <div className="vs-controls">
        <fieldset className="vs-segments">
          <legend>Mostra</legend>
          {statusOptions.map((option) => (
            <label key={option.value} className="vs-segment">
              <input
                type="radio"
                name="vs-status"
                value={option.value}
                checked={status === option.value}
                onChange={() => {
                  setStatus(option.value);
                }}
              />
              {`${option.label} (${String(counts[option.value])})`}
            </label>
          ))}
        </fieldset>
        <label className="vs-field">
          Cerca
          <input
            type="search"
            value={search}
            onChange={(event) => {
              setSearch(event.target.value);
            }}
          />
        </label>
        <div className="vs-actions">
          <p aria-live="polite">{summary}</p>
          <button type="button" onClick={reload}>
            Aggiorna
          </button>
        </div>
      </div>
      {prescriptions.loading ? <p>Carico visite ed esami…</p> : null}
      {!prescriptions.loading && prescriptions.error === "unauthorized" ? (
        <p role="alert">Accedi per vedere visite ed esami.</p>
      ) : null}
      {!prescriptions.loading &&
      prescriptions.error !== undefined &&
      prescriptions.error !== "unauthorized" ? (
        <p role="alert">Il portale regionale non risponde. Riprova tra poco.</p>
      ) : null}
      {ready && appointments.error === "unauthorized" ? (
        <p role="alert" className="vs-notice">
          Accedi per vedere gli appuntamenti.
        </p>
      ) : null}
      {ready && appointments.error === "needs_gestione" ? (
        <p role="alert" className="vs-notice">
          Per vedere date e orari serve l&apos;accesso a Gestione Prenotazioni:
          premi Accedi.
        </p>
      ) : null}
      {ready && appointments.error === "unavailable" ? (
        <p role="alert" className="vs-notice">
          Appuntamenti non disponibili: il portale regionale non risponde.
        </p>
      ) : null}
      {ready && visible.length === 0 ? (
        <p>Nessuna visita o esame in questa vista.</p>
      ) : null}
      {ready && visible.length > 0 ? (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <SortHeader
                  label="Prescritta"
                  sortKey="prescritta"
                  sort={sort}
                />
                <SortHeader
                  label="Prestazione"
                  sortKey="prestazione"
                  sort={sort}
                />
                <SortHeader label="Stato" sortKey="stato" sort={sort} />
                <SortHeader
                  label="Appuntamento"
                  sortKey="appuntamento"
                  sort={sort}
                />
                <SortHeader label="Struttura" sortKey="struttura" sort={sort} />
                <SortHeader label="NRE" sortKey="nre" sort={sort} />
                <th scope="col">Azioni</th>
              </tr>
            </thead>
            <tbody>
              {visible.map((row) => (
                <VisitRow
                  key={row.nre}
                  row={row}
                  state={effectiveState(row, bookingByNre, notInCup)}
                  booking={bookingByNre.get(row.nre.toUpperCase())}
                  appointmentsLoading={appointments.loading}
                  appointmentsFailed={appointments.error !== undefined}
                  now={currentNow}
                />
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </section>
  );
}
