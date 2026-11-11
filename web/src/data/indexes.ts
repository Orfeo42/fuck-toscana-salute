import type {
  AppointmentBody,
  AppointmentsBody,
  PrescriptionsBody,
} from "../api/client";

export interface Indexes {
  bookingByNre: ReadonlyMap<string, AppointmentBody>;
  unbooked: ReadonlySet<string>;
  prescriptionNres: ReadonlySet<string>;
}

function time(appointment: AppointmentBody): number {
  return appointment.at === undefined
    ? Number.NEGATIVE_INFINITY
    : new Date(appointment.at).getTime();
}

function isBetter(
  candidate: AppointmentBody,
  current: AppointmentBody,
): boolean {
  const candidateLive = candidate.status !== "DISDETTO";
  const currentLive = current.status !== "DISDETTO";
  if (candidateLive !== currentLive) {
    return candidateLive;
  }
  return time(candidate) > time(current);
}

export function buildIndexes(
  appointments: AppointmentsBody | undefined,
  prescriptions: PrescriptionsBody | undefined,
): Indexes {
  const bookingByNre = new Map<string, AppointmentBody>();
  for (const appointment of appointments?.appointments ?? []) {
    const key = appointment.nre.toUpperCase();
    const current = bookingByNre.get(key);
    if (current === undefined || isBetter(appointment, current)) {
      bookingByNre.set(key, appointment);
    }
  }

  const unbooked = new Set<string>();
  for (const nre of appointments?.unbooked ?? []) {
    unbooked.add(nre.toUpperCase());
  }

  const prescriptionNres = new Set<string>();
  for (const row of prescriptions?.prescriptions ?? []) {
    prescriptionNres.add(row.nre.toUpperCase());
  }

  return { bookingByNre, unbooked, prescriptionNres };
}
