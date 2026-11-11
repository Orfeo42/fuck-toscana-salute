import type { AppointmentBody, RowBody, UnitBody } from "../../api/client";
import type { StatusTone } from "../../components/StatusBadge";
import { prescriptionStatusLabel } from "../../lib/status";
import { compareValues, type SortDir } from "../../lib/useSort";
import { romeDay } from "./relativeDay";

const toBookStatus = 3;
const cancelledBooking = "DISDETTO";
const bookedLabel = "Prenotata";
const bookedElsewhereLabel = "Prenotata fuori dal CUP regionale";
const expiredLabel = "Scaduta";

const officialTones: Readonly<Record<number, StatusTone>> = {
  3: "tobook",
  4: "closed",
  5: "pending",
  7: "pending",
  8: "done",
  9: "done",
  30: "booked",
};

const closedStatuses: ReadonlySet<number> = new Set([4, 8, 9]);

export type StatusFilter = "upcoming" | "bookable" | "open" | "all";

export type VisitSortKey =
  "prescritta" | "prestazione" | "stato" | "appuntamento" | "struttura" | "nre";

export interface Criteria {
  status: StatusFilter;
  search: string;
  showExpired: boolean;
}

export type StatusCounts = Record<StatusFilter, number>;

export interface BookedState {
  kind: "booked";
  at: string | undefined;
  status: string;
  label: string;
  tone: StatusTone;
}

export interface BookedElsewhereState {
  kind: "booked_elsewhere";
  label: string;
  tone: StatusTone;
}

export interface OfficialState {
  kind: "official";
  status: number;
  label: string;
  tone: StatusTone;
}

export interface ExpiredState {
  kind: "expired";
  label: string;
  tone: StatusTone;
  expiresOn: string;
}

export type EffectiveState =
  BookedState | BookedElsewhereState | ExpiredState | OfficialState;

export interface BookingContext {
  bookingByNre: ReadonlyMap<string, AppointmentBody>;
  notInCup: ReadonlySet<string>;
  now: Date;
}

export function effectiveState(
  row: RowBody,
  bookingByNre: ReadonlyMap<string, AppointmentBody>,
  notInCup: ReadonlySet<string>,
  now: Date,
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
  if (notInCup.has(key)) {
    return {
      kind: "booked_elsewhere",
      label: bookedElsewhereLabel,
      tone: "pending",
    };
  }
  const status = row.fse?.status ?? toBookStatus;
  const expiresOn = row.fse?.expiresOn;
  if (
    status === toBookStatus &&
    expiresOn !== undefined &&
    romeDay(new Date(expiresOn)) < romeDay(now)
  ) {
    return {
      kind: "expired",
      label: expiredLabel,
      tone: "closed",
      expiresOn,
    };
  }
  return {
    kind: "official",
    status,
    label: prescriptionStatusLabel(status),
    tone: officialTones[status] ?? "closed",
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
    context.notInCup,
    context.now,
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

function isFutureBooking(row: RowBody, context: BookingContext): boolean {
  const state = effectiveState(
    row,
    context.bookingByNre,
    context.notInCup,
    context.now,
  );
  return (
    state.kind === "booked" &&
    state.at !== undefined &&
    new Date(state.at).getTime() > context.now.getTime()
  );
}

function isOpen(row: RowBody, context: BookingContext): boolean {
  const state = effectiveState(
    row,
    context.bookingByNre,
    context.notInCup,
    context.now,
  );
  if (state.kind === "expired") {
    return false;
  }
  if (row.fse === undefined) {
    return true;
  }
  return !closedStatuses.has(row.fse.status);
}

function isExpired(row: RowBody, context: BookingContext): boolean {
  return (
    effectiveState(row, context.bookingByNre, context.notInCup, context.now)
      .kind === "expired"
  );
}

function matchesStatus(
  row: RowBody,
  status: StatusFilter,
  context: BookingContext,
): boolean {
  if (status === "upcoming") {
    return upcomingAt(row, context) !== undefined;
  }
  if (status === "bookable") {
    const state = effectiveState(
      row,
      context.bookingByNre,
      context.notInCup,
      context.now,
    );
    return state.kind === "official" && state.status === toBookStatus;
  }
  if (status === "open") {
    return isFutureBooking(row, context) || isOpen(row, context);
  }
  return true;
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

export function filterRows(
  rows: readonly RowBody[],
  criteria: Criteria,
  context: BookingContext,
): RowBody[] {
  return rows.filter(
    (row) =>
      (criteria.showExpired || !isExpired(row, context)) &&
      matchesStatus(row, criteria.status, context) &&
      matchesSearch(row, criteria.search, context),
  );
}

export function countByStatus(
  rows: readonly RowBody[],
  search: string,
  showExpired: boolean,
  context: BookingContext,
): StatusCounts {
  const base = rows.filter(
    (row) =>
      (showExpired || !isExpired(row, context)) &&
      matchesSearch(row, search, context),
  );
  return {
    upcoming: base.filter((row) => matchesStatus(row, "upcoming", context))
      .length,
    bookable: base.filter((row) => matchesStatus(row, "bookable", context))
      .length,
    open: base.filter((row) => matchesStatus(row, "open", context)).length,
    all: base.length,
  };
}

export function countExpired(
  rows: readonly RowBody[],
  search: string,
  context: BookingContext,
): number {
  return rows.filter(
    (row) => isExpired(row, context) && matchesSearch(row, search, context),
  ).length;
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
    context.notInCup,
    context.now,
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
