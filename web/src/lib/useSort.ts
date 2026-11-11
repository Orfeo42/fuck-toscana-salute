import { useCallback, useState } from "react";

export type SortDir = "asc" | "desc";

export interface SortState<K extends string> {
  key: K;
  dir: SortDir;
  toggle: (key: K) => void;
  ariaSort: (key: K) => "ascending" | "descending" | "none";
}

export function compareValues(
  a: string | number | undefined,
  b: string | number | undefined,
  dir: SortDir,
): number {
  if (a === undefined && b === undefined) return 0;
  if (a === undefined) return 1;
  if (b === undefined) return -1;
  const sign = dir === "asc" ? 1 : -1;
  if (typeof a === "number" && typeof b === "number") return sign * (a - b);
  return (
    sign *
    String(a).localeCompare(String(b), "it", {
      sensitivity: "base",
      numeric: true,
    })
  );
}

export function useSort<K extends string>(
  initialKey: K,
  initialDir: SortDir,
  descFirst?: readonly K[],
): SortState<K> {
  const [state, setState] = useState<{ key: K; dir: SortDir }>({
    key: initialKey,
    dir: initialDir,
  });

  const toggle = useCallback(
    (key: K): void => {
      setState((prev) => {
        if (prev.key === key) {
          return { key, dir: prev.dir === "asc" ? "desc" : "asc" };
        }
        return { key, dir: descFirst?.includes(key) ? "desc" : "asc" };
      });
    },
    [descFirst],
  );

  const ariaSort = useCallback(
    (key: K): "ascending" | "descending" | "none" => {
      if (state.key !== key) return "none";
      return state.dir === "asc" ? "ascending" : "descending";
    },
    [state],
  );

  return { key: state.key, dir: state.dir, toggle, ariaSort };
}
