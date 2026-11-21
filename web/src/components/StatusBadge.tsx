import { Badge, Tooltip } from "@mantine/core";
import { UserPen } from "lucide-react";
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
  manual?: boolean;
  children: ReactNode;
}

const manualLabel = "Stato impostato a mano";

export function StatusBadge({
  tone,
  manual = false,
  children,
}: Props): JSX.Element {
  const manualIcon = (
    <Tooltip label={manualLabel} withArrow openDelay={150}>
      <UserPen size={12} role="img" aria-label={manualLabel} />
    </Tooltip>
  );
  return (
    <Badge
      variant="light"
      radius="sm"
      tt="none"
      color={colors[tone]}
      leftSection={manual ? manualIcon : undefined}
    >
      {children}
    </Badge>
  );
}
