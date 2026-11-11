import { useEffect, useRef, type RefObject } from "react";

import { useRoute, type TabId } from "./route";

export interface FocusTarget<T extends HTMLElement> {
  id: string;
  ref: RefObject<T | null>;
  tabIndex: -1;
  focused: boolean;
}

export function focusId(tab: TabId, nre: string): string {
  return `${tab}-${nre.toUpperCase()}`;
}

export function useFocusTarget<T extends HTMLElement>(
  tab: TabId,
  nre: string,
): FocusTarget<T> {
  const route = useRoute();
  const ref = useRef<T | null>(null);
  const focused = route.tab === tab && route.focus === nre.toUpperCase();

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

  return { id: focusId(tab, nre), ref, tabIndex: -1, focused };
}
