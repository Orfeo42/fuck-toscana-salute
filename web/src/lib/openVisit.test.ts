import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import {
  clearRequestedVisit,
  requestOpenVisit,
  useRequestedVisit,
} from "./openVisit";

afterEach(() => {
  clearRequestedVisit();
  window.location.hash = "";
});

describe("openVisit", () => {
  it("starts with no request", () => {
    const { result } = renderHook(() => useRequestedVisit());

    expect(result.current).toBeUndefined();
  });

  it("stores the upper-cased NRE and moves to the visits page without it in the URL", () => {
    const { result } = renderHook(() => useRequestedVisit());

    act(() => {
      requestOpenVisit("0900a1234567890");
    });

    expect(result.current).toBe("0900A1234567890");
    expect(window.location.hash).toBe("#/visite");
  });

  it("clears the request", () => {
    const { result } = renderHook(() => useRequestedVisit());
    act(() => {
      requestOpenVisit("0900A1234567890");
    });

    act(() => {
      clearRequestedVisit();
    });

    expect(result.current).toBeUndefined();
  });
});
