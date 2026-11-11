import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PdfViewerModal } from "./PdfViewerModal";
import { render } from "../test/render";

const mocks = vi.hoisted(() => ({
  loadPdf: vi.fn(),
  destroy: vi.fn(),
}));

vi.mock("../lib/pdf", () => ({ loadPdf: mocks.loadPdf }));

function fakePage(): unknown {
  return {
    getViewport: () => ({ width: 100, height: 200 }),
    render: () => ({ promise: Promise.resolve(), cancel: vi.fn() }),
  };
}

describe("PdfViewerModal", () => {
  beforeEach(() => {
    mocks.loadPdf.mockReset();
    mocks.destroy.mockReset();
    mocks.destroy.mockResolvedValue(undefined);
  });

  it("shows the loader then one canvas per page", async () => {
    mocks.loadPdf.mockReturnValue({
      promise: Promise.resolve({
        numPages: 2,
        getPage: () => Promise.resolve(fakePage()),
      }),
      destroy: mocks.destroy,
    });
    render(
      <PdfViewerModal url="/x.pdf" title="Ricetta" opened onClose={vi.fn()} />,
    );

    expect(screen.getByLabelText("Caricamento PDF")).toBeInTheDocument();
    expect(await screen.findByLabelText("Pagina 1 di 2")).toBeInTheDocument();
    expect(screen.getByLabelText("Pagina 2 di 2")).toBeInTheDocument();
    expect(screen.queryByLabelText("Caricamento PDF")).not.toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "Ricetta" })).toBeInTheDocument();
  });

  it("shows an error with a new tab link when loading fails", async () => {
    mocks.loadPdf.mockReturnValue({
      promise: Promise.reject(new Error("401")),
      destroy: mocks.destroy,
    });
    render(
      <PdfViewerModal url="/x.pdf" title="Ricetta" opened onClose={vi.fn()} />,
    );

    expect(
      await screen.findByText("Impossibile aprire il PDF"),
    ).toBeInTheDocument();
    const link = screen.getByRole("link", { name: "Apri in una nuova scheda" });
    expect(link).toHaveAttribute("href", "/x.pdf");
    expect(link).toHaveAttribute("target", "_blank");
  });

  it("calls onClose and destroys the task on unmount", async () => {
    mocks.loadPdf.mockReturnValue({
      promise: new Promise<never>(() => undefined),
      destroy: mocks.destroy,
    });
    const onClose = vi.fn();
    const { rerender } = render(
      <PdfViewerModal url="/x.pdf" title="Ricetta" opened onClose={onClose} />,
    );
    await userEvent
      .setup()
      .click(await screen.findByRole("button", { name: "Chiudi" }));
    expect(onClose).toHaveBeenCalledTimes(1);

    rerender(
      <PdfViewerModal
        url="/x.pdf"
        title="Ricetta"
        opened={false}
        onClose={onClose}
      />,
    );
    await waitFor(() => {
      expect(mocks.destroy).toHaveBeenCalled();
    });
  });

  it("does not load while closed", () => {
    render(
      <PdfViewerModal
        url="/x.pdf"
        title="Ricetta"
        opened={false}
        onClose={vi.fn()}
      />,
    );
    expect(mocks.loadPdf).not.toHaveBeenCalled();
  });
});
