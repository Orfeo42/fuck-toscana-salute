import { describe, expect, it } from "vitest";

import type { AppointmentBody } from "../api/client";
import { buildIndexes } from "./indexes";

function appointment(
  id: string,
  overrides: Partial<AppointmentBody> = {},
): AppointmentBody {
  return {
    id,
    nre: "0900A0000000001",
    status: "PRENOTATO",
    at: "2026-11-12T09:30:00Z",
    unit: {
      name: "OSP ESEMPIO",
      description: "Ospedale Esempio",
      address: "Via Roma 1",
      city: "Firenze",
      province: "FI",
    },
    ...overrides,
  };
}

describe("buildIndexes", () => {
  it("returns empty indexes for undefined input", () => {
    const result = buildIndexes(undefined, undefined);
    expect(result.bookingByNre.size).toBe(0);
    expect(result.notInCup.size).toBe(0);
    expect(result.prescriptionNres.size).toBe(0);
  });

  it("handles null arrays", () => {
    const result = buildIndexes(
      { appointments: null, failed: null },
      { prescriptions: null },
    );
    expect(result.bookingByNre.size).toBe(0);
    expect(result.notInCup.size).toBe(0);
    expect(result.prescriptionNres.size).toBe(0);
  });

  it("prefers a live booking over a later cancelled one", () => {
    const live = appointment("a", { at: "2026-11-12T09:30:00Z" });
    const cancelled = appointment("b", {
      status: "DISDETTO",
      at: "2026-12-01T09:30:00Z",
    });
    const result = buildIndexes(
      { appointments: [cancelled, live], failed: [] },
      undefined,
    );
    expect(result.bookingByNre.get("0900A0000000001")?.id).toBe("a");
  });

  it("prefers the latest among live bookings", () => {
    const early = appointment("a", { at: "2026-11-12T09:30:00Z" });
    const late = appointment("b", { at: "2026-12-12T09:30:00Z" });
    const result = buildIndexes(
      { appointments: [late, early], failed: [] },
      undefined,
    );
    expect(result.bookingByNre.get("0900A0000000001")?.id).toBe("b");
  });

  it("falls back to the latest cancelled booking", () => {
    const early = appointment("a", {
      status: "DISDETTO",
      at: "2026-11-12T09:30:00Z",
    });
    const late = appointment("b", {
      status: "DISDETTO",
      at: "2026-12-12T09:30:00Z",
    });
    const result = buildIndexes(
      { appointments: [early, late], failed: [] },
      undefined,
    );
    expect(result.bookingByNre.get("0900A0000000001")?.id).toBe("b");
  });

  it("treats a booking without date as the oldest", () => {
    const undated: AppointmentBody = {
      id: "a",
      nre: "0900A0000000001",
      status: "PRENOTATO",
      unit: appointment("a").unit,
    };
    const dated = appointment("b");
    const result = buildIndexes(
      { appointments: [dated, undated], failed: [] },
      undefined,
    );
    expect(result.bookingByNre.get("0900A0000000001")?.id).toBe("b");
  });

  it("upper-cases NRE keys", () => {
    const result = buildIndexes(
      {
        appointments: [appointment("a", { nre: "0900a0000000001" })],
        failed: [{ nre: "0900a0000000002", reason: "not_in_cup" }],
      },
      { prescriptions: [{ nre: "0900a0000000003" }] },
    );
    expect(result.bookingByNre.has("0900A0000000001")).toBe(true);
    expect(result.notInCup.has("0900A0000000002")).toBe(true);
    expect(result.prescriptionNres.has("0900A0000000003")).toBe(true);
  });

  it("collects only not_in_cup failures", () => {
    const result = buildIndexes(
      {
        appointments: [],
        failed: [
          { nre: "0900A0000000002", reason: "not_in_cup" },
          { nre: "0900A0000000003", reason: "other" },
        ],
      },
      undefined,
    );
    expect([...result.notInCup]).toEqual(["0900A0000000002"]);
  });
});
