import { FileText } from "lucide-react";
import type { JSX } from "react";

import type { ReportBody, RowBody } from "../../api/client";
import { IconAction } from "../../components/IconAction";
import { linkedReports } from "./reportLinks";

interface Props {
  row: RowBody;
  reports: readonly ReportBody[];
  onOpenReport: (report: ReportBody) => void;
  onOpenDetail: () => void;
}

export function LinkedReportsAction({
  row,
  reports,
  onOpenReport,
  onOpenDetail,
}: Props): JSX.Element | null {
  const count = row.reports?.length ?? 0;
  if (count === 0) {
    return null;
  }
  const resolved = linkedReports(row, reports);
  const [only] = resolved;
  return (
    <IconAction
      label={`Referti collegati (${String(count)})`}
      icon={FileText}
      onClick={() => {
        if (count === 1 && only !== undefined) {
          onOpenReport(only);
          return;
        }
        onOpenDetail();
      }}
    />
  );
}
