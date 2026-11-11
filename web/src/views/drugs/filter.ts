import type { RowBody } from "../../api/client";
import type { StatusTone } from "../../components/StatusBadge";
import { compareValues, type SortDir } from "../../lib/useSort";
import { prescriptionStatusLabel } from "../../lib/status";

const pickupStatus = 3;

export type DrugFilter = "pickup" | "all";
export type DrugSortKey = "data" | "farmaco" | "stato" | "nre";

export interface DrugRowData {
  nre: string;
  fse: NonNullable<RowBody["fse"]>;
}

export type DrugCounts = Record<DrugFilter, number>;

export function drugStatusLabel(status: number): string {
  if (status === pickupStatus) return "Da ritirare";
  return prescriptionStatusLabel(status);
}

export function drugStatusTone(status: number): StatusTone {
  switch (status) {
    case 3:
      return "tobook";
    case 8:
    case 9:
      return "done";
    case 7:
      return "pending";
    default:
      return "closed";
  }
}

export function drugRows(rows: readonly RowBody[]): DrugRowData[] {
  const drugs: DrugRowData[] = [];
  for (const row of rows) {
    if (row.fse?.kind === "drug") {
      drugs.push({ nre: row.nre, fse: row.fse });
    }
  }
  return drugs;
}

export function matchesSearch(row: DrugRowData, search: string): boolean {
  const needle = search.trim().toLowerCase();
  if (needle === "") return true;
  return (
    row.fse.description.toLowerCase().includes(needle) ||
    row.nre.toLowerCase().includes(needle)
  );
}

export function filterDrugs(
  rows: readonly DrugRowData[],
  filter: DrugFilter,
  search: string,
): DrugRowData[] {
  return rows.filter(
    (row) =>
      matchesSearch(row, search) &&
      (filter === "all" || row.fse.status === pickupStatus),
  );
}

export function countDrugs(
  rows: readonly DrugRowData[],
  search: string,
): DrugCounts {
  return {
    pickup: filterDrugs(rows, "pickup", search).length,
    all: filterDrugs(rows, "all", search).length,
  };
}

function sortValue(
  row: DrugRowData,
  key: DrugSortKey,
): string | number | undefined {
  switch (key) {
    case "data":
      return row.fse.date === undefined
        ? undefined
        : new Date(row.fse.date).getTime();
    case "farmaco":
      return row.fse.description;
    case "stato":
      return drugStatusLabel(row.fse.status);
    case "nre":
      return row.nre;
  }
}

export function sortDrugs(
  rows: readonly DrugRowData[],
  key: DrugSortKey,
  dir: SortDir,
): DrugRowData[] {
  return [...rows].sort(
    (a, b) =>
      compareValues(sortValue(a, key), sortValue(b, key), dir) ||
      compareValues(a.nre, b.nre, "asc"),
  );
}
