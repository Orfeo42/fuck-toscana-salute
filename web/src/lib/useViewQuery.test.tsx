import { act, renderHook } from "@testing-library/react";
import type { JSX, ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { PortalBody, VisitsQueryRequest } from "../api/client";
import { WithSession } from "../test/WithSession";
import { useViewQuery } from "./useViewQuery";

const mocks = vi.hoisted(() => ({ post: vi.fn() }));

vi.mock("../api/client", () => ({ api: { POST: mocks.post } }));

const base: VisitsQueryRequest = {
  dir: "desc",
  page: 1,
  pageSize: 50,
  sort: "prescribed",
};

let cupState: PortalBody["state"] = "active";

function wrapper({ children }: { children: ReactNode }): JSX.Element {
  return <WithSession cupState={cupState}>{children}</WithSession>;
}

function answer(refreshing: boolean, status = 200): void {
  mocks.post.mockResolvedValueOnce({
    data: status === 200 ? { refreshing } : undefined,
    response: { status },
  });
}

async function advance(ms: number): Promise<void> {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

describe("useViewQuery", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mocks.post.mockReset();
    cupState = "active";
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it("posts the request right away and exposes the data", async () => {
    answer(false);
    const { result } = renderHook(
      () => useViewQuery("/api/visits/query", base),
      { wrapper },
    );

    expect(result.current.loading).toBe(true);
    await advance(0);

    expect(mocks.post).toHaveBeenCalledTimes(1);
    expect(mocks.post.mock.calls[0]?.[0]).toBe("/api/visits/query");
    expect(mocks.post.mock.calls[0]?.[1]).toMatchObject({ body: base });
    expect(result.current.loading).toBe(false);
    expect(result.current.data?.refreshing).toBe(false);
  });

  it("debounces request changes by 250 ms", async () => {
    answer(false);
    answer(false);
    const { result, rerender } = renderHook(
      (request: VisitsQueryRequest) =>
        useViewQuery("/api/visits/query", request),
      { wrapper, initialProps: base },
    );
    await advance(0);

    rerender({ ...base, search: "e" });
    await advance(100);
    rerender({ ...base, search: "ec" });
    await advance(249);
    expect(mocks.post).toHaveBeenCalledTimes(1);

    await advance(1);
    expect(mocks.post).toHaveBeenCalledTimes(2);
    expect(mocks.post.mock.calls[1]?.[1]).toMatchObject({
      body: { ...base, search: "ec" },
    });
    expect(result.current.loading).toBe(false);
  });

  it("aborts the previous request when the request changes", async () => {
    answer(false);
    answer(false);
    const { rerender } = renderHook(
      (request: VisitsQueryRequest) =>
        useViewQuery("/api/visits/query", request),
      { wrapper, initialProps: base },
    );
    await advance(0);
    const first = mocks.post.mock.calls[0]?.[1] as { signal: AbortSignal };

    rerender({ ...base, search: "x" });
    await advance(250);

    expect(first.signal.aborted).toBe(true);
  });

  it("aborts on unmount", async () => {
    answer(false);
    const { unmount } = renderHook(
      () => useViewQuery("/api/visits/query", base),
      { wrapper },
    );
    await advance(0);
    const first = mocks.post.mock.calls[0]?.[1] as { signal: AbortSignal };

    unmount();

    expect(first.signal.aborted).toBe(true);
  });

  it("polls every 3 s while refreshing and stops when it is over", async () => {
    answer(true);
    answer(true);
    answer(false);
    renderHook(() => useViewQuery("/api/visits/query", base), { wrapper });
    await advance(0);
    expect(mocks.post).toHaveBeenCalledTimes(1);

    await advance(2999);
    expect(mocks.post).toHaveBeenCalledTimes(1);
    await advance(1);
    expect(mocks.post).toHaveBeenCalledTimes(2);
    await advance(3000);
    expect(mocks.post).toHaveBeenCalledTimes(3);

    await advance(30000);
    expect(mocks.post).toHaveBeenCalledTimes(3);
  });

  it("does not poll while the tab is hidden", async () => {
    answer(true);
    answer(false);
    const visibility = vi.spyOn(document, "visibilityState", "get");
    visibility.mockReturnValue("hidden");
    renderHook(() => useViewQuery("/api/visits/query", base), { wrapper });
    await advance(0);

    await advance(9000);
    expect(mocks.post).toHaveBeenCalledTimes(1);

    visibility.mockReturnValue("visible");
    await advance(3000);
    expect(mocks.post).toHaveBeenCalledTimes(2);
  });

  it("keeps the previous data while reloading", async () => {
    answer(false);
    mocks.post.mockReturnValueOnce(new Promise(() => undefined));
    const { result } = renderHook(
      () => useViewQuery("/api/visits/query", base),
      { wrapper },
    );
    await advance(0);

    act(() => {
      result.current.reload();
    });
    await advance(0);

    expect(mocks.post).toHaveBeenCalledTimes(2);
    expect(result.current.loading).toBe(true);
    expect(result.current.data?.refreshing).toBe(false);
  });

  it("refetches when a portal state changes", async () => {
    answer(false);
    answer(false);
    const { rerender } = renderHook(
      () => useViewQuery("/api/visits/query", base),
      { wrapper },
    );
    await advance(0);

    cupState = "expired";
    rerender();
    await advance(0);

    expect(mocks.post).toHaveBeenCalledTimes(2);
  });

  it("maps statuses to errors", async () => {
    answer(false, 401);
    const { result } = renderHook(
      () => useViewQuery("/api/visits/query", base),
      { wrapper },
    );
    await advance(0);

    expect(result.current.error).toBe("unauthorized");
    expect(result.current.data).toBeUndefined();
  });

  it("reports an unreachable server as unavailable", async () => {
    mocks.post.mockRejectedValueOnce(new Error("network"));
    const { result } = renderHook(
      () => useViewQuery("/api/visits/query", base),
      { wrapper },
    );
    await advance(0);

    expect(result.current.error).toBe("unavailable");
  });
});
