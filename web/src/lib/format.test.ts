import { describe, expect, it } from "vitest";

import { formatDate, formatDateTime, formatTime } from "./format";

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
