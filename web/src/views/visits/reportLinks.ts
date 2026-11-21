import type { ReportBody, ReportRefBody, RowBody } from "../../api/client";

export function reportKey(report: ReportRefBody): string {
  return [report.kind, report.abstractId, report.detailId, report.repCode].join(
    "|",
  );
}

export function linkedReports(
  row: RowBody,
  reports: readonly ReportBody[],
): ReportBody[] {
  const keys = new Set((row.reports ?? []).map(reportKey));
  return reports.filter((report) => keys.has(reportKey(report)));
}

export function reportLabel(report: ReportBody): string {
  return report.title ?? report.kindLabel;
}
