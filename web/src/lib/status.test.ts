import { describe, expect, it } from "vitest";

import {
  appointmentStatusLabel,
  isClosedPrescription,
  prescriptionStatusLabel,
} from "./status";

describe("prescriptionStatusLabel", () => {
  it("maps a known code", () => {
    expect(prescriptionStatusLabel(3)).toBe("Da prenotare");
  });

  it("falls back for an unknown code", () => {
    expect(prescriptionStatusLabel(99)).toBe("Stato 99");
  });
});

describe("isClosedPrescription", () => {
  it("is true for done", () => {
    expect(isClosedPrescription(8)).toBe(true);
  });

  it("is false for booked", () => {
    expect(isClosedPrescription(30)).toBe(false);
  });
});

describe("appointmentStatusLabel", () => {
  it("capitalizes the upstream value", () => {
    expect(appointmentStatusLabel("PRENOTATO")).toBe("Prenotato");
  });
});
