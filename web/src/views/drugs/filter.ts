import type { RowBody } from "../../api/client";
import type { StatusTone } from "../../components/StatusBadge";
import { compareValues, type SortDir } from "../../lib/useSort";
import { prescriptionStatusLabel } from "../../lib/status";

const pickupStatus = 3;

export const drugSortKeys = ["data", "farmaco", "stato", "nre"] as const;

export type DrugSortKey = (typeof drugSortKeys)[number];

export interface DrugRowData {
  nre: string;
  fse: NonNullable<RowBody["fse"]>;
}

export interface DrugLabelCount {
  label: string;
  count: number;
}

export const defaultExcludedDrugLabels: readonly string[] = [
  drugStatusLabel(8),
  drugStatusLabel(9),
  drugStatusLabel(4),
];

const labelOrder: readonly string[] = [
  drugStatusLabel(3),
  drugStatusLabel(5),
  drugStatusLabel(6),
  drugStatusLabel(7),
  drugStatusLabel(8),
  drugStatusLabel(9),
  drugStatusLabel(4),
];

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

export function searchDrugs(
  rows: readonly DrugRowData[],
  search: string,
): DrugRowData[] {
  return rows.filter((row) => matchesSearch(row, search));
}

export function excludeDrugLabels(
  rows: readonly DrugRowData[],
  excluded: readonly string[],
): DrugRowData[] {
  return rows.filter(
    (row) => !excluded.includes(drugStatusLabel(row.fse.status)),
  );
}

export function drugLabelCounts(
  rows: readonly DrugRowData[],
): DrugLabelCount[] {
  const counts = new Map<string, number>();
  for (const row of rows) {
    const label = drugStatusLabel(row.fse.status);
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
