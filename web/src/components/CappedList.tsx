import { Button, Stack } from "@mantine/core";
import { useState } from "react";
import type { JSX, ReactNode } from "react";

interface Props<T> {
  items: readonly T[];
  getKey: (item: T) => string;
  renderItem: (item: T) => ReactNode;
  limit?: number;
}

const defaultLimit = 5;

export function CappedList<T>({
  items,
  getKey,
  renderItem,
  limit = defaultLimit,
}: Props<T>): JSX.Element {
  const [expanded, setExpanded] = useState(false);
  const hidden = items.length - limit;
  const shown = expanded || hidden <= 0 ? items : items.slice(0, limit);

  return (
    <Stack gap="xs">
      {shown.map((item) => (
        <div key={getKey(item)}>{renderItem(item)}</div>
      ))}
      {hidden > 0 ? (
        <Button
          variant="subtle"
          size="compact-sm"
          w="fit-content"
          onClick={() => {
            setExpanded(!expanded);
          }}
        >
          {expanded ? "Mostra meno" : `Mostra altri ${String(hidden)}`}
        </Button>
      ) : null}
    </Stack>
  );
}
