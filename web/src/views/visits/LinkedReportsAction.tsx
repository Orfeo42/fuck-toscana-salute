import { FileText } from "lucide-react";
import type { JSX } from "react";

import type { ReportRefBody, VisitRowBody } from "../../api/client";
import { IconAction } from "../../components/IconAction";

interface Props {
  row: VisitRowBody;
  onOpenReport: (ref: ReportRefBody) => void;
  onOpenDetail: () => void;
}

export function LinkedReportsAction({
  row,
  onOpenReport,
  onOpenDetail,
}: Props): JSX.Element | null {
  const [only] = row.reports;
  if (only === undefined) {
    return null;
  }
  return (
    <IconAction
      label={`Referti collegati (${String(row.reports.length)})`}
      icon={FileText}
      onClick={() => {
        if (row.reports.length === 1 && only.available) {
          onOpenReport(only.ref);
          return;
        }
        onOpenDetail();
      }}
    />
  );
}
