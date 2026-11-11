import { useSyncExternalStore } from "react";

export type TabId = "appuntamenti" | "ricette";

export interface Route {
  tab: TabId;
  focus: string | undefined;
}

const hashPrefix = "#/";

function isTabId(value: string): value is TabId {
  return value === "appuntamenti" || value === "ricette";
}

function decode(value: string): string | undefined {
  try {
    return decodeURIComponent(value).toUpperCase();
  } catch {
    return undefined;
  }
}

export function parseRoute(hash: string): Route {
  const fallback: Route = { tab: "appuntamenti", focus: undefined };
  if (!hash.startsWith(hashPrefix)) {
    return fallback;
  }
  const [tab, nre, ...rest] = hash.slice(hashPrefix.length).split("/");
  if (tab === undefined || !isTabId(tab) || rest.length > 0) {
    return fallback;
  }
  if (nre === undefined || nre === "") {
    return { tab, focus: undefined };
  }
  const focus = decode(nre);
  return focus === undefined ? fallback : { tab, focus };
}

export function appointmentHref(nre: string): string {
  return `${hashPrefix}appuntamenti/${encodeURIComponent(nre)}`;
}

export function prescriptionHref(nre: string): string {
  return `${hashPrefix}ricette/${encodeURIComponent(nre)}`;
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
