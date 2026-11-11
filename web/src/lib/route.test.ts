import { renderHook, act } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import {
  appointmentHref,
  parseRoute,
  prescriptionHref,
  useRoute,
} from "./route";

describe("parseRoute", () => {
  it("parses the appuntamenti tab", () => {
    expect(parseRoute("#/appuntamenti")).toEqual({
      tab: "appuntamenti",
      focus: undefined,
    });
  });

  it("parses the ricette tab", () => {
    expect(parseRoute("#/ricette")).toEqual({
      tab: "ricette",
      focus: undefined,
    });
  });

  it("parses an NRE on each tab", () => {
    expect(parseRoute("#/appuntamenti/0900A0000000001")).toEqual({
      tab: "appuntamenti",
      focus: "0900A0000000001",
    });
    expect(parseRoute("#/ricette/0900A0000000001")).toEqual({
      tab: "ricette",
      focus: "0900A0000000001",
    });
  });

  it("upper-cases the NRE", () => {
    expect(parseRoute("#/ricette/0900a0000000001").focus).toBe(
      "0900A0000000001",
    );
  });

  it("decodes the NRE", () => {
    expect(parseRoute("#/ricette/ab%20c").focus).toBe("AB C");
  });

  it("ignores a trailing slash", () => {
    expect(parseRoute("#/ricette/")).toEqual({
      tab: "ricette",
      focus: undefined,
    });
  });

  it("falls back for unknown input", () => {
    const fallback = { tab: "appuntamenti", focus: undefined };
    expect(parseRoute("")).toEqual(fallback);
    expect(parseRoute("#")).toEqual(fallback);
    expect(parseRoute("#/altro")).toEqual(fallback);
    expect(parseRoute("#/altro/0900A0000000001")).toEqual(fallback);
    expect(parseRoute("#/ricette/a/b")).toEqual(fallback);
    expect(parseRoute("#/ricette/%E0%A4%A")).toEqual(fallback);
  });
});

describe("href builders", () => {
  it("builds an appointment href", () => {
    expect(appointmentHref("0900A0000000001")).toBe(
      "#/appuntamenti/0900A0000000001",
    );
  });

  it("builds a prescription href", () => {
    expect(prescriptionHref("0900A0000000001")).toBe(
      "#/ricette/0900A0000000001",
    );
  });

  it("encodes the NRE", () => {
    expect(prescriptionHref("a b/c")).toBe("#/ricette/a%20b%2Fc");
  });
});

describe("useRoute", () => {
  beforeEach(() => {
    window.location.hash = "";
  });

  it("follows hashchange", () => {
    const { result } = renderHook(() => useRoute());
    expect(result.current.tab).toBe("appuntamenti");

    act(() => {
      window.location.hash = "#/ricette/0900A0000000001";
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });

    expect(result.current).toEqual({
      tab: "ricette",
      focus: "0900A0000000001",
    });
  });
});
