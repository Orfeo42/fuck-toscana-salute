import type { JSX, ReactNode } from "react";

import type { RowBody } from "../api/client";
import { HealthDataContext, type Resource } from "../data/healthContext";

interface Props {
  rows: RowBody[];
  children: ReactNode;
}

function resource<T>(data: T): Resource<T> {
  return { data, error: undefined, loading: false, reload: () => undefined };
}

export function WithHealth({ rows, children }: Props): JSX.Element {
  return (
    <HealthDataContext
      value={{
        appointments: resource({
          appointments: [],
          unbooked: [],
          failed: [],
          verdicts: [],
          updated: { lists: null, appointments: {}, verdicts: {} },
        }),
        prescriptions: resource({ prescriptions: rows, updated: null }),
        bookingByNre: new Map(),
        unbooked: new Set(),
        prescriptionNres: new Set(rows.map((row) => row.nre)),
        verdictByNre: new Map(),
        appointmentCheckedAtByNre: new Map(),
        refreshing: false,
        refresh: () => Promise.resolve(),
      }}
    >
      {children}
    </HealthDataContext>
  );
}
