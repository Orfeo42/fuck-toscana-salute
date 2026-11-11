import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PdfLink } from "./PdfLink";

describe("PdfLink", () => {
  it("builds the href with an encoded ref", () => {
    render(<PdfLink documentRef="a/b c&d=1" />);
    const link = screen.getByRole("link", { name: "Apri ricetta (PDF)" });
    expect(link).toHaveAttribute(
      "href",
      "/api/documents/prescription?ref=a%2Fb+c%26d%3D1",
    );
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("uses a custom label", () => {
    render(<PdfLink documentRef="ref1" label="Scarica" />);
    expect(screen.getByRole("link", { name: "Scarica" })).toBeInTheDocument();
  });

  it("renders nothing for an empty ref", () => {
    render(<PdfLink documentRef="" />);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("renders nothing for an undefined ref", () => {
    render(<PdfLink documentRef={undefined} />);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
