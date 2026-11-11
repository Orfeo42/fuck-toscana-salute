import type { AppointmentBody, RowBody } from "../../api/client";
import { isClosedPrescription } from "../../lib/status";

const toBookStatus = 3;
const cancelledBooking = "DISDETTO";

export type StatusFilter = "bookable" | "open" | "all";
export type KindFilter = "all" | "visit" | "drug";

export interface Criteria {
  status: StatusFilter;
  kind: KindFilter;
  search: string;
}

export type StatusCounts = Record<StatusFilter, number>;

export interface BookedState {
  kind: "booked";
  at: string | undefined;
  status: string;
}

export interface BookedElsewhereState {
  kind: "booked_elsewhere";
}

export interface OfficialState {
  kind: "official";
  status: number;
}

export type EffectiveState = BookedState | BookedElsewhereState | OfficialState;

export interface BookingContext {
  bookingByNre: ReadonlyMap<string, AppointmentBody>;
  notInCup: ReadonlySet<string>;
  now: Date;
}

export function effectiveState(
  row: RowBody,
  bookingByNre: ReadonlyMap<string, AppointmentBody>,
  notInCup: ReadonlySet<string>,
): EffectiveState {
  const key = row.nre.toUpperCase();
  const booking = bookingByNre.get(key);
  if (booking !== undefined && booking.status !== cancelledBooking) {
    return { kind: "booked", at: booking.at, status: booking.status };
  }
  if (notInCup.has(key)) {
    return { kind: "booked_elsewhere" };
  }
  return { kind: "official", status: row.fse?.status ?? toBookStatus };
}

function isBookable(row: RowBody): boolean {
  return row.cup !== undefined;
}

function isOpen(row: RowBody): boolean {
  if (row.fse === undefined) {
    return true;
  }
  return !isClosedPrescription(row.fse.status);
}

function matchesStatus(
  row: RowBody,
  status: StatusFilter,
  context: BookingContext,
): boolean {
  const state = effectiveState(row, context.bookingByNre, context.notInCup);
  if (status === "bookable") {
    return isBookable(row) && state.kind === "official";
  }
  if (status === "open") {
    if (
      state.kind === "booked" &&
      state.at !== undefined &&
      new Date(state.at).getTime() > context.now.getTime()
    ) {
      return true;
    }
    return isOpen(row);
  }
  return true;
}

function matchesKind(row: RowBody, kind: KindFilter): boolean {
  if (kind === "drug") {
    return row.fse?.kind === "drug";
  }
  if (kind === "visit") {
    return row.fse?.kind === "visit" || isBookable(row);
  }
  return true;
}

function matchesSearch(row: RowBody, search: string): boolean {
  const needle = search.trim().toLowerCase();
  if (needle === "") {
    return true;
  }
  const haystack = [
    row.nre,
    row.fse?.description ?? "",
    ...(row.cup?.items ?? []),
  ];
  return haystack.some((text) => text.toLowerCase().includes(needle));
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

export function filterRows(
  rows: readonly RowBody[],
  criteria: Criteria,
  context: BookingContext,
): RowBody[] {
  return rows.filter(
    (row) =>
      matchesStatus(row, criteria.status, context) &&
      matchesKind(row, criteria.kind) &&
      matchesSearch(row, criteria.search),
  );
}

export function countByStatus(
  rows: readonly RowBody[],
  kind: KindFilter,
  search: string,
  context: BookingContext,
): StatusCounts {
  const base = rows.filter(
    (row) => matchesKind(row, kind) && matchesSearch(row, search),
  );
  return {
    bookable: base.filter((row) => matchesStatus(row, "bookable", context))
      .length,
    open: base.filter((row) => matchesStatus(row, "open", context)).length,
    all: base.length,
  };
}

export function sortRows(rows: readonly RowBody[]): RowBody[] {
  return [...rows].sort((a, b) => {
    const dateA = rowDate(a);
    const dateB = rowDate(b);
    if (dateA === undefined && dateB === undefined) {
      return 0;
    }
    if (dateA === undefined) {
      return 1;
    }
    if (dateB === undefined) {
      return -1;
    }
    return new Date(dateB).getTime() - new Date(dateA).getTime();
  });
}
