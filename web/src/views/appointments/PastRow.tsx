import type { JSX } from "react";

import type { AppointmentBody } from "../../api/client";
import { PdfLink } from "../../components/PdfLink";
import { useHealthData } from "../../data/useHealthData";
import { formatDateTime } from "../../lib/format";
import { prescriptionHref } from "../../lib/route";
import { useFocusTarget } from "../../lib/useFocusTarget";
import { appointmentStatusLabel } from "../../lib/status";

interface Props {
  appointment: AppointmentBody;
}

export function PastRow({ appointment }: Props): JSX.Element {
  const { at, unit, fse, status, nre } = appointment;
  const { prescriptionNres } = useHealthData();
  const { id, ref, tabIndex, focused } = useFocusTarget<HTMLLIElement>(
    "appuntamento",
    nre,
  );
  return (
    <li
      id={id}
      ref={ref}
      tabIndex={tabIndex}
      className={`appointment-row${focused ? " is-focused" : ""}`}
    >
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
      {prescriptionNres.has(nre.toUpperCase()) && (
        <a href={prescriptionHref(nre)}>Vai alla ricetta</a>
      )}
    </li>
  );
}
