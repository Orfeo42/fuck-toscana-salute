import { describe, expect, it } from "vitest";

import { driveFolderId } from "./driveFolder";

describe("driveFolderId", () => {
  it("returns a plain id unchanged", () => {
    expect(driveFolderId("1AbC_d-9")).toBe("1AbC_d-9");
  });

  it("extracts the id from a folder URL with a query", () => {
    expect(
      driveFolderId(
        "https://drive.google.com/drive/folders/1AbC_d-9?usp=sharing",
      ),
    ).toBe("1AbC_d-9");
  });

  it("extracts the id from a user-scoped folder URL with a trailing slash", () => {
    expect(
      driveFolderId("https://drive.google.com/drive/u/0/folders/XyZ123/"),
    ).toBe("XyZ123");
  });

  it("extracts the id from an open?id= URL", () => {
    expect(driveFolderId("https://drive.google.com/open?id=QwE456")).toBe(
      "QwE456",
    );
  });

  it("trims whitespace", () => {
    expect(driveFolderId("  1AbC  ")).toBe("1AbC");
  });

  it("returns an empty string for empty input", () => {
    expect(driveFolderId("")).toBe("");
  });
});
