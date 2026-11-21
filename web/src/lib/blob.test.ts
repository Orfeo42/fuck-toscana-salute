import { describe, expect, it } from "vitest";

import { blobToDataUrl } from "./blob";

describe("blobToDataUrl", () => {
  it("encodes the blob as a data URL with its type", async () => {
    const url = await blobToDataUrl(new Blob(["abc"], { type: "image/png" }));

    expect(url).toBe("data:image/png;base64,YWJj");
  });
});
