import { useState } from "react";
import type { JSX } from "react";

import { PrescriptionCard } from "./prescriptions/PrescriptionCard";
import {
  countByStatus,
  filterRows,
  sortRows,
  type KindFilter,
  type StatusFilter,
} from "./prescriptions/filter";
import "./prescriptions/prescriptions.css";
import { usePrescriptions } from "./prescriptions/usePrescriptions";

const statusOptions: readonly { value: StatusFilter; label: string }[] = [
  { value: "bookable", label: "Da prenotare" },
  { value: "open", label: "Aperte" },
  { value: "all", label: "Tutte" },
];

const kindOptions: readonly { value: KindFilter; label: string }[] = [
  { value: "all", label: "Tutti" },
  { value: "visit", label: "Visite ed esami" },
  { value: "drug", label: "Farmaci" },
];

function isKindFilter(value: string): value is KindFilter {
  return kindOptions.some((option) => option.value === value);
}

export function PrescriptionsView(): JSX.Element {
  const { data, error, loading, reload } = usePrescriptions();
  const [status, setStatus] = useState<StatusFilter>("open");
  const [kind, setKind] = useState<KindFilter>("all");
  const [search, setSearch] = useState("");

  const counts = countByStatus(data, kind, search);
  const visible = sortRows(filterRows(data, { status, kind, search }));
  const summary = `${String(visible.length)} ${visible.length === 1 ? "ricetta" : "ricette"}`;

  return (
    <section aria-labelledby="ricette-title" aria-busy={loading}>
      <h2 id="ricette-title">Ricette</h2>
      <div className="rx-controls">
        <fieldset className="rx-segments">
          <legend>Mostra</legend>
          {statusOptions.map((option) => (
            <label key={option.value} className="rx-segment">
              <input
                type="radio"
                name="rx-status"
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
        <label className="rx-field">
          Cerca
          <input
            type="search"
            value={search}
            onChange={(event) => {
              setSearch(event.target.value);
            }}
          />
        </label>
        <label className="rx-field">
          Tipo
          <select
            value={kind}
            onChange={(event) => {
              if (isKindFilter(event.target.value)) {
                setKind(event.target.value);
              }
            }}
          >
            {kindOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </label>
        <div className="rx-actions">
          <p aria-live="polite">{summary}</p>
          <button type="button" onClick={reload}>
            Aggiorna
          </button>
        </div>
      </div>
      {loading ? <p>Carico le ricette…</p> : null}
      {!loading && error === "unauthorized" ? (
        <p role="alert" className="rx-error">
          Accedi per vedere le ricette.
        </p>
      ) : null}
      {!loading && error === "unavailable" ? (
        <p role="alert" className="rx-error">
          Il portale regionale non risponde. Riprova tra poco.
        </p>
      ) : null}
      {!loading && error === undefined && visible.length === 0 ? (
        <p>Nessuna ricetta in questa vista.</p>
      ) : null}
      {!loading && error === undefined && visible.length > 0 ? (
        <ul className="rx-list">
          {visible.map((row) => (
            <PrescriptionCard key={row.nre} row={row} />
          ))}
        </ul>
      ) : null}
    </section>
  );
}
