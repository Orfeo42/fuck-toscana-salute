import type {
  AppointmentBody,
  AppointmentsBody,
  PrescriptionsBody,
  VerdictBody,
} from "../api/client";

export interface Verdict {
  reason: VerdictBody["reason"];
  message: string;
  verifiedAt: string | undefined;
}

export interface Indexes {
  bookingByNre: ReadonlyMap<string, AppointmentBody>;
  unbooked: ReadonlySet<string>;
  prescriptionNres: ReadonlySet<string>;
  verdictByNre: ReadonlyMap<string, Verdict>;
  appointmentCheckedAtByNre: ReadonlyMap<string, string>;
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

  const upperKeys = (
    record: Readonly<Record<string, string>>,
  ): Map<string, string> =>
    new Map(Object.entries(record).map(([nre, at]) => [nre.toUpperCase(), at]));

  const verifiedAt = upperKeys(appointments?.updated.verdicts ?? {});
  const verdictByNre = new Map<string, Verdict>();
  for (const verdict of appointments?.verdicts ?? []) {
    const key = verdict.nre.toUpperCase();
    verdictByNre.set(key, {
      reason: verdict.reason,
      message: verdict.message,
      verifiedAt: verifiedAt.get(key),
    });
  }

  const appointmentCheckedAtByNre = upperKeys(
    appointments?.updated.appointments ?? {},
  );

  return {
    bookingByNre,
    unbooked,
    prescriptionNres,
    verdictByNre,
    appointmentCheckedAtByNre,
  };
}
