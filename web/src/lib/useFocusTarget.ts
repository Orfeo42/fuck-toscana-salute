import { useEffect } from "react";

import { useRoute, type TabId } from "./route";

export function focusId(tab: TabId, nre: string): string {
  return `${tab}-${nre.toUpperCase()}`;
}

export function useFocusTarget(tab: TabId, ready: boolean): string | undefined {
  const route = useRoute();
  const focus = route.tab === tab ? route.focus : undefined;

  useEffect(() => {
    if (focus === undefined || !ready) {
      return;
    }
    const el = document.getElementById(focusId(tab, focus));
    if (el === null) {
      return;
    }
    if (typeof el.scrollIntoView === "function") {
      el.scrollIntoView({ block: "center" });
    }
    el.focus();
  }, [tab, focus, ready]);

  return focus;
}
