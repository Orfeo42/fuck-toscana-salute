import { beforeEach, describe, expect, it, vi } from "vitest";

import { startRefresh } from "./refresh";

const mocks = vi.hoisted(() => ({ post: vi.fn() }));

vi.mock("../api/client", () => ({
  api: { POST: mocks.post },
}));

describe("startRefresh", () => {
  beforeEach(() => {
    mocks.post.mockReset();
  });

  it("posts the scope to the start endpoint and reports success", async () => {
    mocks.post.mockResolvedValue({ response: { ok: true } });

    await expect(startRefresh({ lists: true })).resolves.toBe(true);
    expect(mocks.post).toHaveBeenCalledWith("/api/refresh/start", {
      body: { lists: true },
    });
  });

  it("reports failure on a non-2xx answer", async () => {
    mocks.post.mockResolvedValue({ response: { ok: false } });

    await expect(startRefresh({ nres: ["0900A0000000001"] })).resolves.toBe(
      false,
    );
  });
});
