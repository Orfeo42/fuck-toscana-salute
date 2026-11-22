import { useCallback, useState } from "react";

export type SortDir = "asc" | "desc";

export interface SortState<K extends string> {
  key: K;
  dir: SortDir;
  toggle: (key: K) => void;
  ariaSort: (key: K) => "ascending" | "descending" | "none";
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
