import { describe, expect, it } from "vitest";

import {
  formatDate,
  formatDateTime,
  formatShortDateTime,
  formatTime,
  formatUpdated,
} from "./format";

describe("formatDate", () => {
  it("formats an ISO date in Rome time", () => {
    expect(formatDate("2026-11-20T09:30:00+01:00")).toBe("ven 20 nov 2026");
  });

  it("uses the Rome day across the UTC boundary", () => {
    expect(formatDate("2026-11-19T23:30:00Z")).toBe("ven 20 nov 2026");
  });

  it("returns a dash for undefined", () => {
    expect(formatDate(undefined)).toBe("—");
  });
});

describe("formatDateTime", () => {
  it("formats date and time in Rome time", () => {
    expect(formatDateTime("2026-11-20T08:30:00Z")).toBe(
      "ven 20 nov 2026, 09:30",
    );
  });

  it("returns a dash for undefined", () => {
    expect(formatDateTime(undefined)).toBe("—");
  });
});

describe("formatTime", () => {
  it("formats the time in Rome time", () => {
    expect(formatTime("2026-07-20T07:05:00Z")).toBe("09:05");
  });

  it("returns a dash for undefined", () => {
    expect(formatTime(undefined)).toBe("—");
  });
});

describe("formatShortDateTime", () => {
  it("formats day, month and time in Rome time", () => {
    expect(formatShortDateTime("2026-11-10T08:05:00Z")).toBe("10/11 09:05");
  });
});

describe("formatUpdated", () => {
  const now = new Date("2026-11-10T12:00:00Z");

  it("shows only the time for a timestamp from today", () => {
    expect(formatUpdated("2026-11-10T08:05:00Z", now)).toBe(
      "Aggiornato alle 09:05",
    );
  });

  it("shows day, month and time for another day", () => {
    expect(formatUpdated("2026-11-09T08:05:00Z", now)).toBe(
      "Aggiornato il 09/11 09:05",
    );
  });

  it("uses the Rome day across the UTC boundary", () => {
    expect(formatUpdated("2026-11-09T23:30:00Z", now)).toBe(
      "Aggiornato alle 00:30",
    );
  });

  it("returns undefined for null and undefined", () => {
    expect(formatUpdated(null, now)).toBeUndefined();
    expect(formatUpdated(undefined, now)).toBeUndefined();
  });
});
