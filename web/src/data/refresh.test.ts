import { beforeEach, describe, expect, it, vi } from "vitest";

import { refreshPortals } from "./refresh";

const mocks = vi.hoisted(() => ({ post: vi.fn() }));

vi.mock("../api/client", () => ({
  api: { POST: mocks.post },
}));

describe("refreshPortals", () => {
  beforeEach(() => {
    mocks.post.mockReset();
  });

  it("posts the scope and reports success", async () => {
    mocks.post.mockResolvedValue({ response: { ok: true } });

    await expect(refreshPortals({ nres: ["0900A0000000001"] })).resolves.toBe(
      true,
    );
    expect(mocks.post).toHaveBeenCalledWith("/api/refresh", {
      body: { nres: ["0900A0000000001"] },
    });
  });

  it("reports failure on a non-2xx answer", async () => {
    mocks.post.mockResolvedValue({ response: { ok: false } });

    await expect(refreshPortals({ all: true })).resolves.toBe(false);
  });
});
