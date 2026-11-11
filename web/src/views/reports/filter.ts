import type { ReportBody } from "../../api/client";
import { compareValues, type SortDir } from "../../lib/useSort";

export type ReportSortKey = "date" | "kindLabel" | "provider";

export const reportSortKeys: readonly ReportSortKey[] = [
  "date",
  "kindLabel",
  "provider",
];

export function reportId(report: ReportBody): string {
  return [report.kind, report.abstractId, report.detailId, report.repCode].join(
    "|",
  );
}

export function searchReports(
  reports: readonly ReportBody[],
  query: string,
): ReportBody[] {
  const needle = query.trim().toLowerCase();
  if (needle === "") {
    return [...reports];
  }
  return reports.filter(
    (report) =>
      report.provider.toLowerCase().includes(needle) ||
      report.kindLabel.toLowerCase().includes(needle),
  );
}

export function kindLabels(reports: readonly ReportBody[]): string[] {
  return [...new Set(reports.map((report) => report.kindLabel))].sort((a, b) =>
    a.localeCompare(b, "it"),
  );
}

export function filterByKind(
  reports: readonly ReportBody[],
  kindLabel: string,
): ReportBody[] {
  if (kindLabel === "") {
    return [...reports];
  }
  return reports.filter((report) => report.kindLabel === kindLabel);
}

export function sortReports(
  reports: readonly ReportBody[],
  key: ReportSortKey,
  dir: SortDir,
): ReportBody[] {
  return [...reports].sort((a, b) => compareValues(a[key], b[key], dir));
}
