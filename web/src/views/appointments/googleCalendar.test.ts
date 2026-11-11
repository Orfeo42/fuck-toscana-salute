import { describe, expect, it } from "vitest";

import type { AppointmentBody } from "../../api/client";
import { googleCalendarUrl } from "./googleCalendar";

function appointment(
  overrides: Partial<AppointmentBody> = {},
): AppointmentBody {
  return {
    id: "a1",
    nre: "0900A0000000001",
    status: "PRENOTATO",
    at: "2026-11-12T09:30:00+01:00",
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

describe("googleCalendarUrl", () => {
  it("builds the exact URL converting a +01:00 time to UTC", () => {
    const url = googleCalendarUrl(
      appointment({
        fse: {
          description: "Visita cardiologica",
          documentRef: "doc1",
          date: "2026-11-01T00:00:00Z",
          status: 1,
          kind: "x",
          id: "f1",
          nre: "0900A0000000001",
        },
      }),
    );

    expect(url).toBe(
      "https://calendar.google.com/calendar/render?action=TEMPLATE&text=Visita+cardiologica&dates=20261112T083000Z%2F20261112T093000Z&location=Ospedale+Esempio%2C+Via+Roma+1%2C+Firenze%2C+FI&details=Prenotazione+CUP+Regione+Toscana",
    );
  });

  it("converts a +02:00 time to UTC", () => {
    const url = googleCalendarUrl(
      appointment({ at: "2026-07-12T09:30:00+02:00" }),
    );

    expect(new URL(url ?? "").searchParams.get("dates")).toBe(
      "20260712T073000Z/20260712T083000Z",
    );
  });

  it("falls back to the generic title", () => {
    const url = googleCalendarUrl(appointment());

    expect(new URL(url ?? "").searchParams.get("text")).toBe("Visita / esame");
  });

  it("returns undefined when at is missing", () => {
    const { id, nre, status, unit } = appointment();

    expect(googleCalendarUrl({ id, nre, status, unit })).toBeUndefined();
  });

  it("does not leak the NRE", () => {
    const url = googleCalendarUrl(appointment());

    expect(url).not.toContain("0900A0000000001");
  });
});
