import { createContext } from "react";

import type {
  AppointmentBody,
  AppointmentsBody,
  PrescriptionsBody,
} from "../api/client";
import type { RefreshScope } from "./refresh";
import type { Verdict } from "./indexes";

export type HealthError = "unauthorized" | "needs_gestione" | "unavailable";

export interface Resource<T> {
  data: T | undefined;
  error: HealthError | undefined;
  loading: boolean;
  reload: () => void;
}

export interface HealthData {
  appointments: Resource<AppointmentsBody>;
  prescriptions: Resource<PrescriptionsBody>;
  bookingByNre: ReadonlyMap<string, AppointmentBody>;
  unbooked: ReadonlySet<string>;
  prescriptionNres: ReadonlySet<string>;
  verdictByNre: ReadonlyMap<string, Verdict>;
  appointmentCheckedAtByNre: ReadonlyMap<string, string>;
  refreshing: boolean;
  refresh: (scope: RefreshScope) => Promise<void>;
}

export const HealthDataContext = createContext<HealthData | undefined>(
  undefined,
);
