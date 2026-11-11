import type { RowBody } from "../../api/client";
import { formatDate } from "../../lib/format";
import type { EffectiveState } from "./filter";

export function statusDetails(row: RowBody, state: EffectiveState): string[] {
  const details: string[] = [];
  if (state.kind === "official" && state.note !== undefined) {
    details.push(state.note);
  }
  const expiresOn = row.fse?.expiresOn;
  const estimatedSuffix = row.fse?.expiryEstimated === true ? " (stimata)" : "";
  if (expiresOn !== undefined && state.kind === "expired") {
    details.push(`scaduta il ${formatDate(expiresOn)}${estimatedSuffix}`);
  }
  if (
    expiresOn !== undefined &&
    state.kind === "official" &&
    state.status === 3
  ) {
    details.push(`scade il ${formatDate(expiresOn)}${estimatedSuffix}`);
  }
  return details;
}
