import { CalendarCheck, MapPin } from "lucide-react";
import type { JSX } from "react";

import type { AppointmentBody, RowBody } from "../../api/client";
import { CopyButton } from "../../components/CopyButton";
import { IconAction } from "../../components/IconAction";
import { PdfLink } from "../../components/PdfLink";
import { StatusBadge } from "../../components/StatusBadge";
import { formatDate, formatTime } from "../../lib/format";
import { useFocusTarget } from "../../lib/useFocusTarget";
import { BookAction } from "./BookAction";
import { unitName, rowDate, rowTitle, type EffectiveState } from "./filter";
import { googleCalendarUrl } from "./googleCalendar";
import { mapUrl } from "./mapUrl";
import { PromemoriaLink } from "./PromemoriaLink";
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
  const bookingId = state.kind === "booked" ? booking?.bookingId : undefined;
  const expiresOn = row.fse?.expiresOn;
  let expiryNote: string | null = null;
  if (expiresOn !== undefined && state.kind === "expired") {
    expiryNote = `scaduta il ${formatDate(expiresOn)}`;
  } else if (
    expiresOn !== undefined &&
    state.kind === "official" &&
    state.status === 3
  ) {
    expiryNote = `scade il ${formatDate(expiresOn)}`;
  }
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
        {expiryNote === null ? null : (
          <small className="vs-sub">{expiryNote}</small>
        )}
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
            <IconAction
              label="Apri su mappa"
              icon={MapPin}
              href={mapUrl(unit.address, unit.city)}
              external
            />
          </>
        )}
      </td>
      <td data-label="NRE" className="col-nowrap">
        <CopyButton value={row.nre} label="Copia NRE" />
      </td>
      <td data-label="Azioni">
        <div className="col-actions">
          <PromemoriaLink bookingId={bookingId} />
          {calendarUrl === undefined ? null : (
            <IconAction
              label="Aggiungi a Google Calendar"
              icon={CalendarCheck}
              href={calendarUrl}
              external
            />
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
