import type { JSX } from "react";

import type { FailedBody } from "../../api/client";
import { useHealthData } from "../../data/useHealthData";
import { prescriptionHref } from "../../lib/route";
import { failureLabel } from "../../lib/status";
import { useFocusTarget } from "../../lib/useFocusTarget";

interface Props {
  failed: FailedBody;
}

export function FailedRow({ failed }: Props): JSX.Element {
  const { prescriptionNres } = useHealthData();
  const { id, ref, tabIndex, focused } = useFocusTarget<HTMLLIElement>(
    "appuntamento",
    failed.nre,
  );
  return (
    <li
      id={id}
      ref={ref}
      tabIndex={tabIndex}
      className={focused ? "is-focused" : undefined}
    >
      <strong>{failed.nre}</strong>: {failureLabel(failed.reason)}
      <p className="appointments-hint">
        Controlla l&apos;SMS o il promemoria ricevuto alla prenotazione.
      </p>
      {prescriptionNres.has(failed.nre.toUpperCase()) && (
        <a href={prescriptionHref(failed.nre)}>Vai alla ricetta</a>
      )}
    </li>
  );
}
