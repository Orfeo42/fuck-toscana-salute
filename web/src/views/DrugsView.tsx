import { useState } from "react";
import type { JSX } from "react";

import { CopyButton } from "../components/CopyButton";
import { PdfLink } from "../components/PdfLink";
import { SortHeader } from "../components/SortHeader";
import { StatusBadge } from "../components/StatusBadge";
import { useHealthData } from "../data/useHealthData";
import { formatDate } from "../lib/format";
import { useRoute } from "../lib/route";
import { useFocusTarget } from "../lib/useFocusTarget";
import { useSort } from "../lib/useSort";
import {
  countDrugs,
  drugRows,
  drugStatusLabel,
  drugStatusTone,
  filterDrugs,
  sortDrugs,
  type DrugFilter,
  type DrugRowData,
  type DrugSortKey,
} from "./drugs/filter";
import "./drugs/drugs.css";

const descFirst: readonly DrugSortKey[] = ["data"];

const filterOptions: readonly { value: DrugFilter; label: string }[] = [
  { value: "pickup", label: "Da ritirare" },
  { value: "all", label: "Tutte" },
];

interface RowProps {
  row: DrugRowData;
}

function DrugRow({ row }: RowProps): JSX.Element {
  const { id, ref, tabIndex, focused } = useFocusTarget<HTMLTableRowElement>(
    "farmaci",
    row.nre,
  );
  return (
    <tr
      id={id}
      ref={ref}
      tabIndex={tabIndex}
      className={focused ? "is-focused" : undefined}
    >
      <td data-label="Data" className="col-nowrap">
        {formatDate(row.fse.date)}
      </td>
      <td data-label="Farmaco">{row.fse.description || "Farmaco"}</td>
      <td data-label="Stato">
        <StatusBadge tone={drugStatusTone(row.fse.status)}>
          {drugStatusLabel(row.fse.status)}
        </StatusBadge>
      </td>
      <td data-label="NRE" className="col-nowrap">
        <span className="drugs-nre">{row.nre}</span>
        <CopyButton value={row.nre} label="Copia NRE" />
      </td>
      <td data-label="Azioni" className="col-actions">
        <PdfLink documentRef={row.fse.documentRef} />
      </td>
    </tr>
  );
}

export function DrugsView(): JSX.Element {
  const { data: body, error, loading, reload } = useHealthData().prescriptions;
  const route = useRoute();
  const focus = route.tab === "farmaci" ? route.focus : undefined;
  const [handledFocus, setHandledFocus] = useState<string | undefined>(
    undefined,
  );
  const [filter, setFilter] = useState<DrugFilter>("pickup");
  const [search, setSearch] = useState("");
  const sort = useSort<DrugSortKey>("data", "desc", descFirst);

  const drugs = drugRows(body?.prescriptions ?? []);
  const counts = countDrugs(drugs, search);
  const visible = sortDrugs(
    filterDrugs(drugs, filter, search),
    sort.key,
    sort.dir,
  );

  if (body !== undefined && focus !== handledFocus) {
    setHandledFocus(focus);
    const exists = drugs.some((row) => row.nre.toUpperCase() === focus);
    const shown = visible.some((row) => row.nre.toUpperCase() === focus);
    if (exists && !shown) {
      setFilter("all");
      setSearch("");
    }
  }

  const ready = !loading && error === undefined;

  return (
    <section
      className="drugs-view"
      aria-labelledby="drugs-title"
      aria-busy={loading}
    >
      <h2 id="drugs-title">Farmaci</h2>
      <div className="drugs-controls">
        <fieldset className="drugs-segments">
          <legend>Mostra</legend>
          {filterOptions.map((option) => (
            <label key={option.value} className="drugs-segment">
              <input
                type="radio"
                name="drugs-filter"
                value={option.value}
                checked={filter === option.value}
                onChange={() => {
                  setFilter(option.value);
                }}
              />
              {`${option.label} (${String(counts[option.value])})`}
            </label>
          ))}
        </fieldset>
        <label className="drugs-field">
          Cerca
          <input
            type="search"
            value={search}
            onChange={(event) => {
              setSearch(event.target.value);
            }}
          />
        </label>
        <div className="drugs-actions">
          <p aria-live="polite">{`${String(visible.length)} farmaci`}</p>
          <button type="button" onClick={reload}>
            Aggiorna
          </button>
        </div>
      </div>
      {loading ? <p>Carico i farmaci…</p> : null}
      {!loading && error === "unauthorized" ? (
        <p role="alert">Accedi per vedere i farmaci.</p>
      ) : null}
      {!loading && error !== undefined && error !== "unauthorized" ? (
        <p role="alert">Il portale regionale non risponde. Riprova tra poco.</p>
      ) : null}
      {ready && visible.length === 0 ? (
        <p>Nessun farmaco in questa vista.</p>
      ) : null}
      {ready && visible.length > 0 ? (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <SortHeader label="Data" sortKey="data" sort={sort} />
                <SortHeader label="Farmaco" sortKey="farmaco" sort={sort} />
                <SortHeader label="Stato" sortKey="stato" sort={sort} />
                <SortHeader label="NRE" sortKey="nre" sort={sort} />
                <th scope="col">Azioni</th>
              </tr>
            </thead>
            <tbody>
              {visible.map((row) => (
                <DrugRow key={row.nre} row={row} />
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </section>
  );
}
