import type { JSX } from "react";

import type { AppointmentBody } from "../../api/client";
import { PdfLink } from "../../components/PdfLink";
import { formatDate, formatTime } from "../../lib/format";
import { appointmentStatusLabel } from "../../lib/status";
import { mapUrl } from "./mapUrl";
import { relativeDay } from "./relativeDay";

interface Props {
  appointment: AppointmentBody;
  now: Date;
}

export function AppointmentCard({ appointment, now }: Props): JSX.Element {
  const { at, unit, fse, status } = appointment;
  const place = [unit.address, unit.city, unit.province]
    .filter((part) => part !== "")
    .join(", ");
  return (
    <li className="card appointment-card">
      <time className="appointment-when" dateTime={at}>
        <span className="appointment-date">{formatDate(at)}</span>
        <span className="appointment-time">{formatTime(at)}</span>
      </time>
      <div className="appointment-body">
        {at !== undefined && (
          <p className="appointment-relative">{relativeDay(at, now)}</p>
        )}
        <p className="appointment-service">
          {fse !== undefined && fse.description !== ""
            ? fse.description
            : "Prestazione"}
        </p>
        <p className="appointment-unit">
          {unit.description !== "" ? unit.description : unit.name}
        </p>
        <p className="appointment-place">
          {place}{" "}
          <a
            href={mapUrl(unit.address, unit.city)}
            target="_blank"
            rel="noopener noreferrer"
          >
            Mappa
          </a>
        </p>
        <p className="appointment-meta">
          <span className="appointment-status">
            {appointmentStatusLabel(status)}
          </span>
          <PdfLink documentRef={fse?.documentRef} />
        </p>
      </div>
    </li>
  );
}
