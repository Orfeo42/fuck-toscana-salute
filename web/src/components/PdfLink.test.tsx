import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { PdfLink } from "./PdfLink";
import { render } from "../test/render";

vi.mock("../lib/pdf", () => ({
  loadPdf: () => ({
    promise: new Promise<never>(() => undefined),
    destroy: () => Promise.resolve(),
  }),
}));

describe("PdfLink", () => {
  it("opens the viewer dialog on click", async () => {
    render(<PdfLink documentRef="a/b c&d=1" />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    await userEvent
      .setup()
      .click(screen.getByRole("button", { name: "Apri ricetta (PDF)" }));

    expect(
      await screen.findByRole("dialog", { name: "Ricetta" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Apri in una nuova scheda" }),
    ).toHaveAttribute(
      "href",
      "/api/documents/prescription?ref=a%2Fb+c%26d%3D1",
    );
  });

  it("removes the viewer dialog from the DOM after closing", async () => {
    const user = userEvent.setup();
    render(<PdfLink documentRef="ref1" />);
    await user.click(
      screen.getByRole("button", { name: "Apri ricetta (PDF)" }),
    );
    await screen.findByRole("dialog", { name: "Ricetta" });

    await user.click(screen.getByRole("button", { name: "Chiudi" }));

    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  });

  it("is not a link anymore", () => {
    render(<PdfLink documentRef="ref1" />);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("uses a custom label", () => {
    render(<PdfLink documentRef="ref1" label="Scarica" />);
    expect(screen.getByRole("button", { name: "Scarica" })).toBeInTheDocument();
  });

  it("renders nothing for an empty ref", () => {
    render(<PdfLink documentRef="" />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("renders nothing for an undefined ref", () => {
    render(<PdfLink documentRef={undefined} />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
