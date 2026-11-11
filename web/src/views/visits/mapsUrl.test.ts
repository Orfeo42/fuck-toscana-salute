import { describe, expect, it } from "vitest";

import type { UnitBody } from "../../api/client";
import { fullAddress, mapsDirectionsUrl, mapsSearchUrl } from "./mapsUrl";

const noPostalCode: UnitBody = {
  name: "Unita esempio",
  description: "",
  address: "Via Roma 1",
  city: "Firenze",
  province: "FI",
};

const unit: UnitBody = { ...noPostalCode, postalCode: "50100" };

describe("fullAddress", () => {
  it("joins all parts", () => {
    expect(fullAddress(unit)).toBe("Via Roma 1, 50100 Firenze FI");
  });

  it("skips a missing postal code", () => {
    expect(fullAddress(noPostalCode)).toBe("Via Roma 1, Firenze FI");
  });

  it("skips an empty province", () => {
    expect(fullAddress({ ...unit, province: "" })).toBe(
      "Via Roma 1, 50100 Firenze",
    );
  });

  it("skips an empty address without a dangling comma", () => {
    expect(fullAddress({ ...unit, address: "" })).toBe("50100 Firenze FI");
  });
});

describe("mapsDirectionsUrl", () => {
  it("uses the full address only, even when a place is set", () => {
    expect(mapsDirectionsUrl({ ...unit, place: "Ospedale Esempio" })).toBe(
      "https://www.google.com/maps/dir/?api=1&destination=Via%20Roma%201%2C%2050100%20Firenze%20FI",
    );
  });

  it("encodes special characters", () => {
    expect(mapsDirectionsUrl({ ...unit, address: "Via A&B 2/c" })).toBe(
      "https://www.google.com/maps/dir/?api=1&destination=Via%20A%26B%202%2Fc%2C%2050100%20Firenze%20FI",
    );
  });
});

describe("mapsSearchUrl", () => {
  it("uses the full address when no place is set", () => {
    expect(mapsSearchUrl(unit)).toBe(
      "https://www.google.com/maps/search/?api=1&query=Via%20Roma%201%2C%2050100%20Firenze%20FI",
    );
  });

  it("prefixes the place when set", () => {
    expect(mapsSearchUrl({ ...unit, place: "Ospedale Esempio" })).toBe(
      "https://www.google.com/maps/search/?api=1&query=Ospedale%20Esempio%2C%20Via%20Roma%201%2C%2050100%20Firenze%20FI",
    );
  });

  it("handles a missing postal code and province", () => {
    expect(mapsSearchUrl({ ...noPostalCode, province: "" })).toBe(
      "https://www.google.com/maps/search/?api=1&query=Via%20Roma%201%2C%20Firenze",
    );
  });
});
