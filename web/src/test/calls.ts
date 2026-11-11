import type { Mock } from "vitest";

export function callsTo(get: Mock, path: string): number {
  return get.mock.calls.filter((call) => call[0] === path).length;
}
