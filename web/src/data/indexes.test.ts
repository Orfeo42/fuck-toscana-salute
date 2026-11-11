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

const noUpdates = { lists: null, appointments: {}, verdicts: {} };

describe("buildIndexes", () => {
  it("returns empty indexes for undefined input", () => {
    const result = buildIndexes(undefined, undefined);
    expect(result.bookingByNre.size).toBe(0);
    expect(result.unbooked.size).toBe(0);
    expect(result.prescriptionNres.size).toBe(0);
  });

  it("handles null arrays", () => {
    const result = buildIndexes(
      {
        appointments: null,
        failed: null,
        unbooked: null,
        verdicts: null,
        updated: noUpdates,
      },
      { prescriptions: null, updated: null },
    );
    expect(result.bookingByNre.size).toBe(0);
    expect(result.unbooked.size).toBe(0);
    expect(result.prescriptionNres.size).toBe(0);
  });

  it("prefers a live booking over a later cancelled one", () => {
    const live = appointment("a", { at: "2026-11-12T09:30:00Z" });
    const cancelled = appointment("b", {
      status: "DISDETTO",
      at: "2026-12-01T09:30:00Z",
    });
    const result = buildIndexes(
      {
        appointments: [cancelled, live],
        failed: [],
        unbooked: [],
        verdicts: [],
        updated: noUpdates,
      },
      undefined,
    );
    expect(result.bookingByNre.get("0900A0000000001")?.id).toBe("a");
  });

  it("prefers the latest among live bookings", () => {
    const early = appointment("a", { at: "2026-11-12T09:30:00Z" });
    const late = appointment("b", { at: "2026-12-12T09:30:00Z" });
    const result = buildIndexes(
      {
        appointments: [late, early],
        failed: [],
        unbooked: [],
        verdicts: [],
        updated: noUpdates,
      },
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
      {
        appointments: [early, late],
        failed: [],
        unbooked: [],
        verdicts: [],
        updated: noUpdates,
      },
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
      {
        appointments: [dated, undated],
        failed: [],
        unbooked: [],
        verdicts: [],
        updated: noUpdates,
      },
      undefined,
    );
    expect(result.bookingByNre.get("0900A0000000001")?.id).toBe("b");
  });

  it("upper-cases NRE keys", () => {
    const result = buildIndexes(
      {
        appointments: [appointment("a", { nre: "0900a0000000001" })],
        failed: [],
        unbooked: ["0900a0000000002"],
        verdicts: [],
        updated: noUpdates,
      },
      { prescriptions: [{ nre: "0900a0000000003" }], updated: null },
    );
    expect(result.bookingByNre.has("0900A0000000001")).toBe(true);
    expect(result.unbooked.has("0900A0000000002")).toBe(true);
    expect(result.prescriptionNres.has("0900A0000000003")).toBe(true);
  });

  it("builds the unbooked set from the unbooked list", () => {
    const result = buildIndexes(
      {
        appointments: [],
        failed: [],
        unbooked: ["0900A0000000002"],
        verdicts: [],
        updated: noUpdates,
      },
      undefined,
    );
    expect([...result.unbooked]).toEqual(["0900A0000000002"]);
  });

  it("builds verdicts with their verification time, upper-casing NREs", () => {
    const result = buildIndexes(
      {
        appointments: [],
        failed: [],
        unbooked: [],
        verdicts: [{ nre: "0900a0000000003", reason: "handled", message: "m" }],
        updated: {
          lists: null,
          appointments: { "0900a0000000003": "2026-11-10T08:00:00Z" },
          verdicts: { "0900A0000000003": "2026-11-10T08:30:00Z" },
        },
      },
      undefined,
    );
    expect(result.verdictByNre.get("0900A0000000003")).toEqual({
      reason: "handled",
      message: "m",
      verifiedAt: "2026-11-10T08:30:00Z",
    });
    expect(result.appointmentCheckedAtByNre.get("0900A0000000003")).toBe(
      "2026-11-10T08:00:00Z",
    );
  });

  it("leaves verifiedAt undefined when no time is known", () => {
    const result = buildIndexes(
      {
        appointments: [],
        failed: [],
        unbooked: [],
        verdicts: [{ nre: "0900A0000000003", reason: "other", message: "m" }],
        updated: noUpdates,
      },
      undefined,
    );
    expect(
      result.verdictByNre.get("0900A0000000003")?.verifiedAt,
    ).toBeUndefined();
  });
});
