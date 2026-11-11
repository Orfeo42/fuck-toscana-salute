import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { compareValues, useSort } from "./useSort";

type Key = "date" | "name";

describe("useSort", () => {
  it("exposes the initial state", () => {
    const { result } = renderHook(() => useSort<Key>("name", "asc"));
    expect(result.current.key).toBe("name");
    expect(result.current.dir).toBe("asc");
  });

  it("flips the direction when toggling the active key", () => {
    const { result } = renderHook(() => useSort<Key>("name", "asc"));
    act(() => {
      result.current.toggle("name");
    });
    expect(result.current.dir).toBe("desc");
    act(() => {
      result.current.toggle("name");
    });
    expect(result.current.dir).toBe("asc");
  });

  it("switches to a new key ascending", () => {
    const { result } = renderHook(() => useSort<Key>("name", "desc"));
    act(() => {
      result.current.toggle("date");
    });
    expect(result.current.key).toBe("date");
    expect(result.current.dir).toBe("asc");
  });

  it("switches to a descFirst key descending", () => {
    const descFirst: readonly Key[] = ["date"];
    const { result } = renderHook(() => useSort<Key>("name", "asc", descFirst));
    act(() => {
      result.current.toggle("date");
    });
    expect(result.current.key).toBe("date");
    expect(result.current.dir).toBe("desc");
  });

  it("reports aria-sort values", () => {
    const { result } = renderHook(() => useSort<Key>("name", "asc"));
    expect(result.current.ariaSort("name")).toBe("ascending");
    expect(result.current.ariaSort("date")).toBe("none");
    act(() => {
      result.current.toggle("name");
    });
    expect(result.current.ariaSort("name")).toBe("descending");
  });
});

describe("compareValues", () => {
  it("compares numbers in both directions", () => {
    expect(compareValues(1, 2, "asc")).toBeLessThan(0);
    expect(compareValues(1, 2, "desc")).toBeGreaterThan(0);
    expect(compareValues(2, 2, "asc")).toBe(0);
  });

  it("compares strings with Italian collation ignoring case and accents", () => {
    expect(compareValues("à", "A", "asc")).toBe(0);
    expect(compareValues("Zeta", "alfa", "asc")).toBeGreaterThan(0);
  });

  it("compares numeric substrings by value", () => {
    expect(compareValues("2", "10", "asc")).toBeLessThan(0);
    expect(compareValues("2", "10", "desc")).toBeGreaterThan(0);
  });

  it("sorts undefined last in both directions", () => {
    expect(compareValues(undefined, "a", "asc")).toBeGreaterThan(0);
    expect(compareValues("a", undefined, "asc")).toBeLessThan(0);
    expect(compareValues(undefined, "a", "desc")).toBeGreaterThan(0);
    expect(compareValues("a", undefined, "desc")).toBeLessThan(0);
    expect(compareValues(undefined, undefined, "asc")).toBe(0);
  });
});
