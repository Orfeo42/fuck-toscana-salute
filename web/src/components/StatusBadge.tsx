import type { JSX, ReactNode } from "react";

export type StatusTone =
  "booked" | "done" | "tobook" | "pending" | "closed" | "error";

interface Props {
  tone: StatusTone;
  children: ReactNode;
}

export function StatusBadge({ tone, children }: Props): JSX.Element {
  return <span className={"status-badge status-" + tone}>{children}</span>;
}
