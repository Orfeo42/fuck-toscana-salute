import type { AppointmentBody, RowBody, UnitBody } from "../../api/client";
import type { StatusTone } from "../../components/StatusBadge";
import type { Verdict } from "../../data/indexes";
import { prescriptionStatusLabel } from "../../lib/status";
import { compareValues, type SortDir } from "../../lib/useSort";
import { romeDay } from "./relativeDay";

const toBookStatus = 3;
const cancelledBooking = "DISDETTO";
const bookedLabel = "Prenotata";
const unverifiedLabel = "Da verificare";
const fseBookedNote =
  "FSE la segna come prenotata, ma nel CUP non c'è un tuo appuntamento: può essere già gestita altrove (prenotazione o accettazione) o ancora prenotabile";
const fseBookedStatus = 30;
const expiredLabel = "Scaduta";
const handledLabel = "Gestita altrove";
const noVerdicts: ReadonlyMap<string, Verdict> = new Map();

const officialTones: Readonly<Record<number, StatusTone>> = {
  3: "tobook",
  4: "closed",
  5: "pending",
  7: "pending",
  8: "done",
  9: "done",
  30: "booked",
};

export const visitSortKeys = [
  "prescritta",
  "prestazione",
  "stato",
  "appuntamento",
  "struttura",
  "nre",
] as const;

export type VisitSortKey = (typeof visitSortKeys)[number];

export const defaultExcludedLabels: readonly string[] = [
  prescriptionStatusLabel(8),
  prescriptionStatusLabel(9),
  prescriptionStatusLabel(4),
  expiredLabel,
];

const labelOrder: readonly string[] = [
  bookedLabel,
  prescriptionStatusLabel(3),
  unverifiedLabel,
  handledLabel,
  prescriptionStatusLabel(5),
  prescriptionStatusLabel(7),
  prescriptionStatusLabel(6),
  prescriptionStatusLabel(8),
  prescriptionStatusLabel(9),
  prescriptionStatusLabel(4),
  expiredLabel,
];

export interface StatusLabelCount {
  label: string;
  count: number;
}

export interface BookedState {
  kind: "booked";
  at: string | undefined;
  status: string;
  label: string;
  tone: StatusTone;
}

export interface OfficialState {
  kind: "official";
  status: number;
  label: string;
  tone: StatusTone;
  note: string | undefined;
  verifiedAt: string | undefined;
  canBook: boolean;
}

export interface ExpiredState {
  kind: "expired";
  label: string;
  tone: StatusTone;
  expiresOn: string | undefined;
  estimated: boolean;
  note: string | undefined;
  verifiedAt: string | undefined;
}

export type EffectiveState = BookedState | ExpiredState | OfficialState;

export interface BookingContext {
  bookingByNre: ReadonlyMap<string, AppointmentBody>;
  unbooked: ReadonlySet<string>;
  verdictByNre: ReadonlyMap<string, Verdict>;
  now: Date;
}

export function effectiveState(
  row: RowBody,
  bookingByNre: ReadonlyMap<string, AppointmentBody>,
  unbooked: ReadonlySet<string>,
  now: Date,
  verdictByNre: ReadonlyMap<string, Verdict> = noVerdicts,
): EffectiveState {
  const key = row.nre.toUpperCase();
  const booking = bookingByNre.get(key);
  if (booking !== undefined && booking.status !== cancelledBooking) {
    return {
      kind: "booked",
      at: booking.at,
      status: booking.status,
      label: bookedLabel,
      tone: "booked",
    };
  }
  const status = row.fse?.status ?? toBookStatus;
  const expiresOn = row.fse?.expiresOn;
  if (
    (status === toBookStatus || status === fseBookedStatus) &&
    expiresOn !== undefined &&
    romeDay(new Date(expiresOn)) < romeDay(now)
  ) {
    return {
      kind: "expired",
      label: expiredLabel,
      tone: "closed",
      expiresOn,
      estimated: row.fse?.expiryEstimated === true,
      note: undefined,
      verifiedAt: undefined,
    };
  }
  const notBooked =
    (status === toBookStatus || status === fseBookedStatus) &&
    unbooked.has(key);
  const verdict = notBooked ? verdictByNre.get(key) : undefined;
  if (verdict?.reason === "bookable") {
    return {
      kind: "official",
      status: toBookStatus,
      label: prescriptionStatusLabel(toBookStatus),
      tone: officialTones[toBookStatus] ?? "closed",
      note: undefined,
      verifiedAt: verdict.verifiedAt,
      canBook: true,
    };
  }
  if (verdict?.reason === "handled") {
    return {
      kind: "official",
      status,
      label: handledLabel,
      tone: "closed",
      note: verdict.message,
      verifiedAt: verdict.verifiedAt,
      canBook: false,
    };
  }
  if (verdict?.reason === "expired") {
    return {
      kind: "expired",
      label: expiredLabel,
      tone: "closed",
      expiresOn,
      estimated: row.fse?.expiryEstimated === true,
      note: verdict.message,
      verifiedAt: verdict.verifiedAt,
    };
  }
  if (status === fseBookedStatus && unbooked.has(key)) {
    return {
      kind: "official",
      status,
      label: unverifiedLabel,
      tone: "pending",
      note: verdict?.message ?? fseBookedNote,
      verifiedAt: verdict?.verifiedAt,
      canBook: true,
    };
  }
  return {
    kind: "official",
    status,
    label: prescriptionStatusLabel(status),
    tone: officialTones[status] ?? "closed",
    note: undefined,
    verifiedAt: undefined,
    canBook: status === toBookStatus,
  };
}

export function unitName(unit: UnitBody): string {
  return unit.description !== "" ? unit.description : unit.name;
}

export function buildRows(
  prescriptions: readonly RowBody[],
  appointments: readonly AppointmentBody[],
): RowBody[] {
  const rows = prescriptions.filter(
    (row) =>
      row.fse?.kind === "visit" ||
      (row.fse === undefined && row.cup !== undefined),
  );
  const seen = new Set(rows.map((row) => row.nre.toUpperCase()));
  for (const appointment of appointments) {
    const key = appointment.nre.toUpperCase();
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    rows.push({
      nre: appointment.nre,
      ...(appointment.cup === undefined ? {} : { cup: appointment.cup }),
      ...(appointment.fse === undefined ? {} : { fse: appointment.fse }),
    });
  }
  return rows;
}

export function rowDate(row: RowBody): string | undefined {
  return row.fse?.date ?? row.cup?.prescribedOn;
}

export function rowTitle(row: RowBody): string {
  if (row.fse !== undefined && row.fse.description !== "") {
    return row.fse.description;
  }
  const items = row.cup?.items ?? [];
  if (items.length > 0) {
    return items.join(", ");
  }
  return "Prescrizione";
}

function upcomingAt(row: RowBody, context: BookingContext): number | undefined {
  const state = effectiveState(
    row,
    context.bookingByNre,
    context.unbooked,
    context.now,
    context.verdictByNre,
  );
  if (state.kind !== "booked" || state.at === undefined) {
    return undefined;
  }
  const at = new Date(state.at);
  if (romeDay(at) < romeDay(context.now)) {
    return undefined;
  }
  return at.getTime();
}

function matchesSearch(
  row: RowBody,
  search: string,
  context: BookingContext,
): boolean {
  const needle = search.trim().toLowerCase();
  if (needle === "") {
    return true;
  }
  const unit = context.bookingByNre.get(row.nre.toUpperCase())?.unit;
  const haystack = [
    row.nre,
    rowTitle(row),
    unit?.name ?? "",
    unit?.description ?? "",
    unit?.city ?? "",
  ];
  return haystack.some((text) => text.toLowerCase().includes(needle));
}

export function searchRows(
  rows: readonly RowBody[],
  search: string,
  context: BookingContext,
): RowBody[] {
  return rows.filter((row) => matchesSearch(row, search, context));
}

export function rowLabel(row: RowBody, context: BookingContext): string {
  return effectiveState(
    row,
    context.bookingByNre,
    context.unbooked,
    context.now,
    context.verdictByNre,
  ).label;
}

export function excludeLabels(
  rows: readonly RowBody[],
  excluded: readonly string[],
  context: BookingContext,
): RowBody[] {
  return rows.filter((row) => !excluded.includes(rowLabel(row, context)));
}

export function labelCounts(
  rows: readonly RowBody[],
  context: BookingContext,
): StatusLabelCount[] {
  const counts = new Map<string, number>();
  for (const row of rows) {
    const label = rowLabel(row, context);
    counts.set(label, (counts.get(label) ?? 0) + 1);
  }
  return [...counts]
    .map(([label, count]) => ({ label, count }))
    .sort((a, b) => {
      const rankA = labelOrder.indexOf(a.label);
      const rankB = labelOrder.indexOf(b.label);
      if (rankA === -1 && rankB === -1) {
        return a.label.localeCompare(b.label, "it");
      }
      if (rankA === -1) {
        return 1;
      }
      if (rankB === -1) {
        return -1;
      }
      return rankA - rankB;
    });
}

function dateMs(row: RowBody): number | undefined {
  const date = rowDate(row);
  return date === undefined ? undefined : new Date(date).getTime();
}

function byPrescribedDesc(a: RowBody, b: RowBody): number {
  return compareValues(dateMs(a), dateMs(b), "desc");
}

function compareAppointment(
  a: RowBody,
  b: RowBody,
  dir: SortDir,
  context: BookingContext,
): number {
  const atA = upcomingAt(a, context);
  const atB = upcomingAt(b, context);
  if (atA === undefined && atB === undefined) {
    return byPrescribedDesc(a, b);
  }
  if (atA === undefined) {
    return 1;
  }
  if (atB === undefined) {
    return -1;
  }
  return compareValues(atA, atB, dir) || byPrescribedDesc(a, b);
}

function sortValue(
  row: RowBody,
  key: Exclude<VisitSortKey, "appuntamento">,
  context: BookingContext,
): string | number | undefined {
  if (key === "prescritta") {
    return dateMs(row);
  }
  if (key === "prestazione") {
    return rowTitle(row);
  }
  if (key === "nre") {
    return row.nre;
  }
  const state = effectiveState(
    row,
    context.bookingByNre,
    context.unbooked,
    context.now,
    context.verdictByNre,
  );
  if (key === "stato") {
    return state.label;
  }
  if (state.kind !== "booked") {
    return undefined;
  }
  const unit = context.bookingByNre.get(row.nre.toUpperCase())?.unit;
  return unit === undefined ? undefined : unitName(unit);
}

export function sortRows(
  rows: readonly RowBody[],
  key: VisitSortKey,
  dir: SortDir,
  context: BookingContext,
): RowBody[] {
  return [...rows].sort((a, b) => {
    if (key === "appuntamento") {
      return compareAppointment(a, b, dir, context);
    }
    return (
      compareValues(
        sortValue(a, key, context),
        sortValue(b, key, context),
        dir,
      ) || byPrescribedDesc(a, b)
    );
  });
}
