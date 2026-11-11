import { renderHook, act } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import { parseRoute, rowHref, useRoute } from "./route";

describe("parseRoute", () => {
  it("parses the visite tab", () => {
    expect(parseRoute("#/visite")).toEqual({
      tab: "visite",
      focus: undefined,
    });
  });

  it("parses the farmaci tab", () => {
    expect(parseRoute("#/farmaci")).toEqual({
      tab: "farmaci",
      focus: undefined,
    });
  });

  it("parses an NRE on each tab", () => {
    expect(parseRoute("#/visite/0900A0000000001")).toEqual({
      tab: "visite",
      focus: "0900A0000000001",
    });
    expect(parseRoute("#/farmaci/0900A0000000001")).toEqual({
      tab: "farmaci",
      focus: "0900A0000000001",
    });
  });

  it("upper-cases the NRE", () => {
    expect(parseRoute("#/farmaci/0900a0000000001").focus).toBe(
      "0900A0000000001",
    );
  });

  it("decodes the NRE", () => {
    expect(parseRoute("#/farmaci/ab%20c").focus).toBe("AB C");
  });

  it("ignores a trailing slash", () => {
    expect(parseRoute("#/farmaci/")).toEqual({
      tab: "farmaci",
      focus: undefined,
    });
  });

  it("falls back for unknown input", () => {
    const fallback = { tab: "visite", focus: undefined };
    expect(parseRoute("")).toEqual(fallback);
    expect(parseRoute("#")).toEqual(fallback);
    expect(parseRoute("#/altro")).toEqual(fallback);
    expect(parseRoute("#/altro/0900A0000000001")).toEqual(fallback);
    expect(parseRoute("#/farmaci/a/b")).toEqual(fallback);
    expect(parseRoute("#/farmaci/%E0%A4%A")).toEqual(fallback);
  });
});

describe("href builders", () => {
  it("builds an encoded row href", () => {
    expect(rowHref("visite", "a b/c")).toBe("#/visite/a%20b%2Fc");
  });

  it("maps legacy tabs to visite", () => {
    expect(parseRoute("#/appuntamenti/0900A0000000001")).toEqual({
      tab: "visite",
      focus: "0900A0000000001",
    });
    expect(parseRoute("#/ricette")).toEqual({
      tab: "visite",
      focus: undefined,
    });
  });
});

describe("useRoute", () => {
  beforeEach(() => {
    window.location.hash = "";
  });

  it("follows hashchange", () => {
    const { result } = renderHook(() => useRoute());
    expect(result.current.tab).toBe("visite");

    act(() => {
      window.location.hash = "#/farmaci/0900A0000000001";
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });

    expect(result.current).toEqual({
      tab: "farmaci",
      focus: "0900A0000000001",
    });
  });
});
