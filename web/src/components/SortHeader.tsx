import { Group, Table, UnstyledButton } from "@mantine/core";
import { ChevronDown, ChevronUp } from "lucide-react";
import type { JSX } from "react";

import type { SortState } from "../lib/useSort";

interface Props<K extends string> {
  label: string;
  sortKey: K;
  sort: SortState<K>;
}

export function SortHeader<K extends string>({
  label,
  sortKey,
  sort,
}: Props<K>): JSX.Element {
  const active = sort.key === sortKey;
  const Chevron = sort.dir === "asc" ? ChevronUp : ChevronDown;
  return (
    <Table.Th scope="col" aria-sort={sort.ariaSort(sortKey)}>
      <UnstyledButton
        w="100%"
        onClick={() => {
          sort.toggle(sortKey);
        }}
      >
        <Group gap={4} wrap="nowrap">
          {label}
          {active ? <Chevron aria-hidden="true" size={14} /> : null}
        </Group>
      </UnstyledButton>
    </Table.Th>
  );
}
