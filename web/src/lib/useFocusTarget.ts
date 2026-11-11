import { useEffect, useRef, type RefObject } from "react";

import { useRoute } from "./route";

export type FocusKind = "appuntamento" | "ricetta";

export interface FocusTarget<T extends HTMLElement> {
  id: string;
  ref: RefObject<T | null>;
  tabIndex: -1;
  focused: boolean;
}

export function focusId(kind: FocusKind, nre: string): string {
  return `${kind}-${nre.toUpperCase()}`;
}

export function useFocusTarget<T extends HTMLElement>(
  kind: FocusKind,
  nre: string,
): FocusTarget<T> {
  const route = useRoute();
  const ref = useRef<T | null>(null);
  const expectedTab = kind === "appuntamento" ? "appuntamenti" : "ricette";
  const focused =
    route.tab === expectedTab && route.focus === nre.toUpperCase();

  useEffect(() => {
    const el = ref.current;
    if (!focused || el === null) {
      return;
    }
    if (typeof el.scrollIntoView === "function") {
      el.scrollIntoView({ block: "center" });
    }
    el.focus();
  }, [focused]);

  return { id: focusId(kind, nre), ref, tabIndex: -1, focused };
}
