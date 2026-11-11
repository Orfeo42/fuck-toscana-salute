import { Badge } from "@mantine/core";
import type { JSX, ReactNode } from "react";

export type StatusTone =
  "booked" | "done" | "tobook" | "pending" | "closed" | "error";

const colors: Readonly<Record<StatusTone, string>> = {
  booked: "blue",
  done: "teal",
  tobook: "orange",
  pending: "yellow",
  closed: "gray",
  error: "red",
};

interface Props {
  tone: StatusTone;
  children: ReactNode;
}

export function StatusBadge({ tone, children }: Props): JSX.Element {
  return (
    <Badge variant="light" radius="sm" tt="none" color={colors[tone]}>
      {children}
    </Badge>
  );
}
