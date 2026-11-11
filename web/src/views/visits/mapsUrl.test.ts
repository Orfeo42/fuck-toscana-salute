import { describe, expect, it } from "vitest";

import { mapsDirectionsUrl, mapsSearchUrl } from "./mapsUrl";

describe("mapsDirectionsUrl", () => {
  it("builds a Google Maps directions url with the encoded destination", () => {
    expect(mapsDirectionsUrl("Via Roma 1", "Firenze")).toBe(
      "https://www.google.com/maps/dir/?api=1&destination=Via%20Roma%201%2C%20Firenze",
    );
  });

  it("encodes special characters", () => {
    expect(mapsDirectionsUrl("Via A&B 2/c", "San Gimignano")).toBe(
      "https://www.google.com/maps/dir/?api=1&destination=Via%20A%26B%202%2Fc%2C%20San%20Gimignano",
    );
  });
});

describe("mapsSearchUrl", () => {
  it("builds a Google Maps search url with the encoded query", () => {
    expect(mapsSearchUrl("Via Roma 1", "Firenze")).toBe(
      "https://www.google.com/maps/search/?api=1&query=Via%20Roma%201%2C%20Firenze",
    );
  });

  it("encodes special characters", () => {
    expect(mapsSearchUrl("Via A&B 2/c", "San Gimignano")).toBe(
      "https://www.google.com/maps/search/?api=1&query=Via%20A%26B%202%2Fc%2C%20San%20Gimignano",
    );
  });
});
