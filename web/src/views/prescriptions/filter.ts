import type { RowBody } from "../../api/client";
import { isClosedPrescription } from "../../lib/status";

export type StatusFilter = "bookable" | "open" | "all";
export type KindFilter = "all" | "visit" | "drug";

export interface Criteria {
  status: StatusFilter;
  kind: KindFilter;
  search: string;
}

export type StatusCounts = Record<StatusFilter, number>;

function isBookable(row: RowBody): boolean {
  return row.cup !== undefined;
}

function isOpen(row: RowBody): boolean {
  if (row.fse === undefined) {
    return true;
  }
  return !isClosedPrescription(row.fse.status);
}

function matchesStatus(row: RowBody, status: StatusFilter): boolean {
  if (status === "bookable") {
    return isBookable(row);
  }
  if (status === "open") {
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
): RowBody[] {
  return rows.filter(
    (row) =>
      matchesStatus(row, criteria.status) &&
      matchesKind(row, criteria.kind) &&
      matchesSearch(row, criteria.search),
  );
}

export function countByStatus(
  rows: readonly RowBody[],
  kind: KindFilter,
  search: string,
): StatusCounts {
  const base = rows.filter(
    (row) => matchesKind(row, kind) && matchesSearch(row, search),
  );
  return {
    bookable: base.filter((row) => matchesStatus(row, "bookable")).length,
    open: base.filter((row) => matchesStatus(row, "open")).length,
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
