import { useState } from "react";
import type { JSX } from "react";

import type { RowBody } from "../../api/client";
import { PdfLink } from "../../components/PdfLink";
import { formatDate, formatDateTime } from "../../lib/format";
import { appointmentHref } from "../../lib/route";
import { useFocusTarget } from "../../lib/useFocusTarget";
import { prescriptionStatusLabel } from "../../lib/status";
import { rowDate, rowTitle, type EffectiveState } from "./filter";

const cupUrl = "https://prenota.sanita.toscana.it/";

type CopyState = "idle" | "copied" | "failed";

const copyMessages: Readonly<Record<CopyState, string>> = {
  idle: "",
  copied: "Copiato",
  failed: "Copia non riuscita",
};

interface Props {
  row: RowBody;
  state: EffectiveState;
}

export function PrescriptionCard({ row, state }: Props): JSX.Element {
  const [copy, setCopy] = useState<CopyState>("idle");
  const { id, ref, tabIndex, focused } = useFocusTarget<HTMLLIElement>(
    "ricetta",
    row.nre,
  );
  const bookable = row.cup !== undefined;
  const classes = ["rx-card"];
  if (bookable) {
    classes.push("rx-card-bookable");
  }
  if (focused) {
    classes.push("is-focused");
  }
  const kindLabel = row.fse?.kind === "drug" ? "Farmaco" : "Visita/esame";

  const copyNre = async (): Promise<void> => {
    try {
      await navigator.clipboard.writeText(row.nre);
      setCopy("copied");
    } catch {
      setCopy("failed");
    }
  };

  return (
    <li id={id} ref={ref} tabIndex={tabIndex} className={classes.join(" ")}>
      <h3 className="rx-title">{rowTitle(row)}</h3>
      <div className="rx-badges">
        <span className="rx-badge">{kindLabel}</span>
        {state.kind === "booked" ? (
          <span className="rx-badge">
            {state.at === undefined
              ? "Prenotata"
              : `Prenotata · ${formatDateTime(state.at)}`}
          </span>
        ) : null}
        {state.kind === "booked_elsewhere" ? (
          <span className="rx-badge">Prenotata fuori dal CUP regionale</span>
        ) : null}
        {state.kind === "official" ? (
          <span className="rx-badge">
            {prescriptionStatusLabel(state.status)}
          </span>
        ) : null}
        <span className="rx-date">{formatDate(rowDate(row))}</span>
      </div>
      <div className="rx-nre-row">
        <code className="rx-nre">{row.nre}</code>
        <button
          type="button"
          onClick={() => {
            void copyNre();
          }}
        >
          Copia
        </button>
        <span role="status" className="rx-copy-status">
          {copyMessages[copy]}
        </span>
      </div>
      <div className="rx-links">
        <PdfLink documentRef={row.fse?.documentRef} />
        {state.kind === "booked" ? (
          <a href={appointmentHref(row.nre)}>Vai all'appuntamento</a>
        ) : null}
        {bookable && state.kind === "official" ? (
          <a href={cupUrl} target="_blank" rel="noopener noreferrer">
            Prenota sul CUP
          </a>
        ) : null}
      </div>
    </li>
  );
}
