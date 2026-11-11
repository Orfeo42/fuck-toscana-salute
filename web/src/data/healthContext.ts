import { createContext } from "react";

import type {
  AppointmentBody,
  AppointmentsBody,
  PrescriptionsBody,
} from "../api/client";

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
  notInCup: ReadonlySet<string>;
  prescriptionNres: ReadonlySet<string>;
}

export const HealthDataContext = createContext<HealthData | undefined>(
  undefined,
);
