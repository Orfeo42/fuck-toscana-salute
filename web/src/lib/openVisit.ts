import { useSyncExternalStore } from "react";

const visitsHash = "#/visite";

let requested: string | undefined;
const listeners = new Set<() => void>();

function notify(): void {
  for (const listener of listeners) {
    listener();
  }
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function snapshot(): string | undefined {
  return requested;
}

export function requestOpenVisit(nre: string): void {
  requested = nre.toUpperCase();
  notify();
  window.location.hash = visitsHash;
}

export function clearRequestedVisit(): void {
  if (requested === undefined) {
    return;
  }
  requested = undefined;
  notify();
}

export function useRequestedVisit(): string | undefined {
  return useSyncExternalStore(subscribe, snapshot);
}
