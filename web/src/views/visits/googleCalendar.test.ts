import { describe, expect, it } from "vitest";

import type { AppointmentBody } from "../../api/client";
import { calendarEventFields } from "./googleCalendar";

function appointment(
  overrides: Partial<AppointmentBody> = {},
): AppointmentBody {
  return {
    id: "APP1",
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

describe("calendarEventFields", () => {
  it("builds title, location and details", () => {
    expect(calendarEventFields(appointment(), "Visita cardiologica")).toEqual({
      title: "Visita cardiologica",
      location: "Ospedale Esempio, Via Roma 1, Firenze, FI",
      details: "Prenotazione CUP Regione Toscana",
    });
  });

  it("falls back to the generic title", () => {
    expect(calendarEventFields(appointment(), "").title).toBe("Visita / esame");
  });

  it("uses the unit name when the description is empty", () => {
    const fields = calendarEventFields(
      appointment({
        unit: {
          name: "OSP ESEMPIO",
          description: "",
          address: "",
          city: "",
          province: "",
        },
      }),
      "x",
    );

    expect(fields.location).toBe("OSP ESEMPIO");
  });

  it("does not leak the NRE", () => {
    const fields = calendarEventFields(appointment(), "Visita");

    expect(JSON.stringify(fields)).not.toContain("0900A0000000001");
  });
});
