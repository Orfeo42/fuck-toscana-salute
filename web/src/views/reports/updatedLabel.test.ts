import { describe, expect, it } from "vitest";

import { updatedLabel } from "./updatedLabel";

const now = new Date("2026-11-10T12:00:00Z");

describe("updatedLabel", () => {
  it("shows only the time for today", () => {
    expect(updatedLabel("2026-11-10T08:30:00Z", now)).toBe(
      "Aggiornato alle 09:30",
    );
  });

  it("shows day, month and time for another day", () => {
    expect(updatedLabel("2026-11-08T14:05:00Z", now)).toBe(
      "Aggiornato il 08/11 15:05",
    );
  });
});
