import { act, renderHook } from "@testing-library/react";
import type { JSX, ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { WithSession } from "../test/WithSession";
import { useRowQuery } from "./useRowQuery";

const mocks = vi.hoisted(() => ({ post: vi.fn() }));

vi.mock("../api/client", () => ({ api: { POST: mocks.post } }));

function wrapper({ children }: { children: ReactNode }): JSX.Element {
  return <WithSession>{children}</WithSession>;
}

async function advance(ms: number): Promise<void> {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

describe("useRowQuery", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mocks.post.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("fetches a changed row without debounce", async () => {
    mocks.post.mockResolvedValue({
      data: { refreshing: false },
      response: { status: 200 },
    });
    const { rerender } = renderHook(
      (nre: string) => useRowQuery("/api/visits/row", { nre }),
      { wrapper, initialProps: "0900A0000000001" },
    );
    await advance(0);

    rerender("0900A0000000002");
    await advance(0);

    expect(mocks.post).toHaveBeenCalledTimes(2);
    expect(mocks.post.mock.calls[1]?.[1]).toMatchObject({
      body: { nre: "0900A0000000002" },
    });
  });

  it("polls while refreshing and then stops", async () => {
    mocks.post
      .mockResolvedValueOnce({
        data: { refreshing: true },
        response: { status: 200 },
      })
      .mockResolvedValueOnce({
        data: { refreshing: false },
        response: { status: 200 },
      });
    renderHook(
      () => useRowQuery("/api/drugs/row", { nre: "0900A0000000001" }),
      {
        wrapper,
      },
    );
    await advance(0);

    await advance(3000);
    expect(mocks.post).toHaveBeenCalledTimes(2);
    await advance(30000);
    expect(mocks.post).toHaveBeenCalledTimes(2);
  });

  it("flags a 404 as not found without an error", async () => {
    mocks.post.mockResolvedValue({
      data: undefined,
      response: { status: 404 },
    });
    const { result } = renderHook(
      () => useRowQuery("/api/visits/row", { nre: "0900A0000000001" }),
      { wrapper },
    );
    await advance(0);

    expect(result.current.notFound).toBe(true);
    expect(result.current.error).toBeUndefined();
    expect(result.current.data).toBeUndefined();
  });
});
