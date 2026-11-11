import type { JSX } from "react";

import type { AppointmentBody } from "../../api/client";
import { PdfLink } from "../../components/PdfLink";
import { formatDateTime } from "../../lib/format";
import { appointmentStatusLabel } from "../../lib/status";

interface Props {
  appointment: AppointmentBody;
}

export function PastRow({ appointment }: Props): JSX.Element {
  const { at, unit, fse, status } = appointment;
  return (
    <li className="appointment-row">
      <time dateTime={at}>{formatDateTime(at)}</time>
      <span>
        {fse !== undefined && fse.description !== ""
          ? fse.description
          : "Prestazione"}
      </span>
      <span>{unit.description !== "" ? unit.description : unit.name}</span>
      <span className="appointment-status">
        {appointmentStatusLabel(status)}
      </span>
      <PdfLink documentRef={fse?.documentRef} />
    </li>
  );
}
