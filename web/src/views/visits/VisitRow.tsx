import type { JSX } from "react";

import type { AppointmentBody, RowBody } from "../../api/client";
import { CopyButton } from "../../components/CopyButton";
import { PdfLink } from "../../components/PdfLink";
import { StatusBadge } from "../../components/StatusBadge";
import { formatDate, formatTime } from "../../lib/format";
import { useFocusTarget } from "../../lib/useFocusTarget";
import { BookAction } from "./BookAction";
import { unitName, rowDate, rowTitle, type EffectiveState } from "./filter";
import { googleCalendarUrl } from "./googleCalendar";
import { mapUrl } from "./mapUrl";
import { relativeDay } from "./relativeDay";

interface Props {
  row: RowBody;
  state: EffectiveState;
  booking: AppointmentBody | undefined;
  appointmentsLoading: boolean;
  appointmentsFailed: boolean;
  now: Date;
}

export function VisitRow({
  row,
  state,
  booking,
  appointmentsLoading,
  appointmentsFailed,
  now,
}: Props): JSX.Element {
  const { id, ref, tabIndex, focused } = useFocusTarget<HTMLTableRowElement>(
    "visite",
    row.nre,
  );
  const at = state.kind === "booked" ? state.at : undefined;
  const unit = state.kind === "booked" ? booking?.unit : undefined;
  const calendarUrl =
    booking !== undefined &&
    at !== undefined &&
    new Date(at).getTime() > now.getTime()
      ? googleCalendarUrl(booking)
      : undefined;
  const place =
    unit === undefined
      ? ""
      : [unit.address, unit.city, unit.province]
          .filter((part) => part !== "")
          .join(", ");

  let appointmentCell: JSX.Element | null = null;
  if (at !== undefined) {
    appointmentCell = (
      <>
        {formatDate(at)} {formatTime(at)}
        <small className="vs-sub">{relativeDay(at, now)}</small>
      </>
    );
  } else if (appointmentsLoading) {
    appointmentCell = <>ricerca…</>;
  } else if (appointmentsFailed) {
    appointmentCell = <>—</>;
  }

  return (
    <tr
      id={id}
      ref={ref}
      tabIndex={tabIndex}
      className={focused ? "is-focused" : undefined}
    >
      <td data-label="Prescritta" className="col-nowrap">
        {formatDate(rowDate(row))}
      </td>
      <td data-label="Prestazione">{rowTitle(row)}</td>
      <td data-label="Stato">
        <StatusBadge tone={state.tone}>{state.label}</StatusBadge>
      </td>
      <td
        data-label="Appuntamento"
        className="col-nowrap"
        aria-busy={appointmentsLoading}
      >
        {appointmentCell}
      </td>
      <td data-label="Struttura">
        {unit === undefined ? null : (
          <>
            {unitName(unit)}
            {place === "" ? null : <small className="vs-sub">{place}</small>}
            <a
              href={mapUrl(unit.address, unit.city)}
              target="_blank"
              rel="noopener noreferrer"
            >
              Mappa
            </a>
          </>
        )}
      </td>
      <td data-label="NRE" className="col-nowrap">
        <span className="vs-mono">{row.nre}</span>
        <CopyButton value={row.nre} label="Copia NRE" />
      </td>
      <td data-label="Azioni">
        <div className="col-actions">
          {calendarUrl === undefined ? null : (
            <a
              href={calendarUrl}
              target="_blank"
              rel="noopener noreferrer"
              aria-label="Aggiungi a Google Calendar"
            >
              Calendar
            </a>
          )}
          <PdfLink documentRef={row.fse?.documentRef} />
          {state.kind === "official" && state.status === 3 ? (
            <BookAction />
          ) : null}
        </div>
      </td>
    </tr>
  );
}
