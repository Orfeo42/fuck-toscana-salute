import { useSyncExternalStore } from "react";

export type TabId = "visite" | "farmaci";

export interface Route {
  tab: TabId;
  focus: string | undefined;
}

const hashPrefix = "#/";

const legacyTabs: Readonly<Record<string, TabId>> = {
  appuntamenti: "visite",
  ricette: "visite",
};

function toTabId(value: string): TabId | undefined {
  if (value === "visite" || value === "farmaci") {
    return value;
  }
  return legacyTabs[value];
}

function decode(value: string): string | undefined {
  try {
    return decodeURIComponent(value).toUpperCase();
  } catch {
    return undefined;
  }
}

export function parseRoute(hash: string): Route {
  const fallback: Route = { tab: "visite", focus: undefined };
  if (!hash.startsWith(hashPrefix)) {
    return fallback;
  }
  const [segment, nre, ...rest] = hash.slice(hashPrefix.length).split("/");
  const tab = segment === undefined ? undefined : toTabId(segment);
  if (tab === undefined || rest.length > 0) {
    return fallback;
  }
  if (nre === undefined || nre === "") {
    return { tab, focus: undefined };
  }
  const focus = decode(nre);
  return focus === undefined ? fallback : { tab, focus };
}

export function rowHref(tab: TabId, nre: string): string {
  return `${hashPrefix}${tab}/${encodeURIComponent(nre)}`;
}

function subscribe(onChange: () => void): () => void {
  window.addEventListener("hashchange", onChange);
  return () => {
    window.removeEventListener("hashchange", onChange);
  };
}

function currentHash(): string {
  return window.location.hash;
}

export function useRoute(): Route {
  const hash = useSyncExternalStore(subscribe, currentHash);
  return parseRoute(hash);
}
