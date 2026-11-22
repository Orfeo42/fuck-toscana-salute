import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { useSort } from "./useSort";

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
