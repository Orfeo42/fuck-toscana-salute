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
  return (
    <th scope="col" aria-sort={sort.ariaSort(sortKey)}>
      <button
        type="button"
        onClick={() => {
          sort.toggle(sortKey);
        }}
      >
        {label}
        {active ? (
          <span aria-hidden="true">{sort.dir === "asc" ? "▲" : "▼"}</span>
        ) : null}
      </button>
    </th>
  );
}
