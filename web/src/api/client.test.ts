import { describe, expect, it } from "vitest";

import { bookingPdfUrl } from "./client";

describe("bookingPdfUrl", () => {
  it("builds the booking document url", () => {
    expect(bookingPdfUrl("ABC123456789")).toBe(
      "/api/documents/booking?id=ABC123456789",
    );
  });

  it("encodes the booking id", () => {
    expect(bookingPdfUrl("a b&c")).toBe("/api/documents/booking?id=a%20b%26c");
  });
});
