import {
  render as rtlRender,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { DrugRowBody } from "../../api/client";
import { TestProviders } from "../../test/TestProviders";
import { WithSession } from "../../test/WithSession";
import { DrugDetailModal } from "./DrugDetailModal";

const mocks = vi.hoisted(() => ({
  post: vi.fn(),
  startRefresh: vi.fn(),
}));

vi.mock("../../data/refresh", () => ({ startRefresh: mocks.startRefresh }));

vi.mock("../../lib/pdf", () => ({
  loadPdf: () => ({
    promise: new Promise<never>(() => undefined),
    destroy: () => Promise.resolve(),
  }),
}));

vi.mock("../../api/client", () => ({
  api: { POST: mocks.post },
  prescriptionPdfUrl: (ref: string) => `/api/documents/prescription?ref=${ref}`,
}));

const row: DrugRowBody = {
  nre: "0900A0000000001",
  title: "Farmaco Esempio",
  date: "2026-11-02T09:00:00Z",
  documentRef: "ref-1",
  status: { key: "taken_in_charge", label: "Da ritirare", tone: "tobook" },
};

function renderModal(onClose = vi.fn()): void {
  rtlRender(
    <WithSession>
      <DrugDetailModal nre={row.nre} onClose={onClose} />
    </WithSession>,
    { wrapper: TestProviders },
  );
}

describe("DrugDetailModal", () => {
  beforeEach(() => {
    mocks.post.mockReset();
    mocks.startRefresh.mockReset();
    mocks.startRefresh.mockResolvedValue(true);
    mocks.post.mockResolvedValue({
      data: { row, refreshing: false },
      response: { status: 200 },
    });
  });

  it("renders the row from the response", async () => {
    renderModal();

    const dialog = await screen.findByRole("dialog", {
      name: "Farmaco Esempio",
    });
    expect(within(dialog).getByText("Da ritirare")).toBeVisible();
    expect(
      within(dialog).getByRole("button", { name: "Copia NRE 0900A0000000001" }),
    ).toBeVisible();
    expect(
      within(dialog).getByRole("button", { name: "Apri ricetta (PDF)" }),
    ).toBeVisible();
  });

  it("shows not found on 404", async () => {
    mocks.post.mockResolvedValue({
      data: undefined,
      error: { detail: "x" },
      response: { status: 404 },
    });
    renderModal();

    expect(
      await screen.findByRole("dialog", { name: "Farmaco non trovato" }),
    ).toBeInTheDocument();
  });

  it("shows the portal message on a server error", async () => {
    mocks.post.mockResolvedValue({
      data: undefined,
      error: { detail: "x" },
      response: { status: 502 },
    });
    renderModal();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Il portale regionale non risponde. Riprova tra poco.",
    );
  });

  it("starts a lists refresh then re-queries the row", async () => {
    const user = userEvent.setup();
    renderModal();
    const dialog = await screen.findByRole("dialog", {
      name: "Farmaco Esempio",
    });

    await user.click(within(dialog).getByRole("button", { name: "Aggiorna" }));

    expect(mocks.startRefresh).toHaveBeenCalledWith({ lists: true });
    await waitFor(() => {
      expect(mocks.post).toHaveBeenCalledTimes(2);
    });
  });

  it("shows a notification when the refresh fails", async () => {
    const user = userEvent.setup();
    mocks.startRefresh.mockResolvedValue(false);
    renderModal();
    const dialog = await screen.findByRole("dialog", {
      name: "Farmaco Esempio",
    });

    await user.click(within(dialog).getByRole("button", { name: "Aggiorna" }));

    expect(await screen.findByText("Aggiornamento non riuscito")).toBeVisible();
    expect(mocks.post).toHaveBeenCalledTimes(1);
  });
});
