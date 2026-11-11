import { useState } from "react";
import type { JSX } from "react";

import type { RowBody } from "../../api/client";
import { PdfLink } from "../../components/PdfLink";
import { formatDate } from "../../lib/format";
import { prescriptionStatusLabel } from "../../lib/status";
import { rowDate, rowTitle } from "./filter";

const cupUrl = "https://prenota.sanita.toscana.it/";
const toBookStatus = 3;

type CopyState = "idle" | "copied" | "failed";

const copyMessages: Readonly<Record<CopyState, string>> = {
  idle: "",
  copied: "Copiato",
  failed: "Copia non riuscita",
};

interface Props {
  row: RowBody;
}

export function PrescriptionCard({ row }: Props): JSX.Element {
  const [copy, setCopy] = useState<CopyState>("idle");
  const bookable = row.cup !== undefined;
  const status = row.fse?.status ?? toBookStatus;
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
    <li className={bookable ? "rx-card rx-card-bookable" : "rx-card"}>
      <h3 className="rx-title">{rowTitle(row)}</h3>
      <div className="rx-badges">
        <span className="rx-badge">{kindLabel}</span>
        <span className="rx-badge">{prescriptionStatusLabel(status)}</span>
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
        {bookable ? (
          <a href={cupUrl} target="_blank" rel="noopener noreferrer">
            Prenota sul CUP
          </a>
        ) : null}
      </div>
    </li>
  );
}
