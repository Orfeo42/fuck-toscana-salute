import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ReportRefBody, ReportRowBody } from "../../api/client";
import { render } from "../../test/render";
import { WithSession } from "../../test/WithSession";
import { ReportDetailModal } from "./ReportDetailModal";

vi.mock("../../lib/pdf", () => ({
  loadPdf: () => ({
    promise: new Promise<never>(() => undefined),
    destroy: () => Promise.resolve(),
  }),
}));

const mocks = vi.hoisted(() => ({
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
}));

vi.mock("../../api/client", () => ({
  api: { POST: mocks.post, PUT: mocks.put, DELETE: mocks.del },
}));

const remoteRef: ReportRefBody = {
  kind: "lab",
  abstractId: "a1",
  detailId: "d1",
  repCode: "r1",
};

const localRef: ReportRefBody = {
  kind: "local",
  abstractId: "L1",
  detailId: "",
  repCode: "",
};

const remote: ReportRowBody = {
  id: "lab|a1|d1|r1",
  kind: "lab",
  kindLabel: "Laboratorio",
  title: "Laboratorio",
  ref: remoteRef,
  date: "2026-10-01T09:00:00Z",
  provider: "Ospedale Esempio",
  unread: false,
  hidden: false,
  origin: "remote",
  visits: [],
  linkedLabel: "",
};

const local: ReportRowBody = {
  id: "local|L1",
  kind: "local",
  kindLabel: "Caricato",
  title: "Ecografia addome",
  customTitle: "Ecografia addome",
  ref: localRef,
  date: "2026-10-05T09:00:00Z",
  editDate: "2026-10-05",
  provider: "Studio Prova",
  unread: false,
  hidden: false,
  origin: "local",
  visits: [{ nre: "0123A", title: "Ecografia addome" }],
  linkedLabel: "Collegato a 1 visita",
};

interface Mocked {
  row: ReportRowBody | undefined;
  rowStatus?: number;
  documents?: { position: string; label: string }[];
  blob?: Blob;
}

function mockApi({
  row,
  rowStatus = 200,
  documents = [{ position: "1", label: "Documento" }],
  blob = new Blob(["pdf"], { type: "application/pdf" }),
}: Mocked): void {
  mocks.post.mockImplementation((path: string) => {
    if (path === "/api/reports/row") {
      return Promise.resolve(
        row === undefined
          ? { data: undefined, response: { status: rowStatus } }
          : {
              data: { row, refreshing: false },
              response: { status: rowStatus },
            },
      );
    }
    if (path === "/api/reports/documents") {
      return Promise.resolve({
        data: { documents },
        response: { status: 200 },
      });
    }
    if (path === "/api/reports/visit-options") {
      return Promise.resolve({
        data: { options: [] },
        response: { status: 200 },
      });
    }
    return Promise.resolve({ data: blob, response: { status: 200 } });
  });
}

function renderModal(reportRef: ReportRefBody): {
  onClose: () => void;
  onChanged: () => void;
} {
  const onClose = vi.fn();
  const onChanged = vi.fn();
  render(
    <WithSession>
      <ReportDetailModal
        reportRef={reportRef}
        onClose={onClose}
        onChanged={onChanged}
      />
    </WithSession>,
  );
  return { onClose, onChanged };
}

describe("ReportDetailModal", () => {
  beforeEach(() => {
    mocks.post.mockReset();
    mocks.put.mockReset();
    mocks.del.mockReset();
    mocks.put.mockResolvedValue({ response: { ok: true } });
    mocks.del.mockResolvedValue({ response: { ok: true } });
    URL.createObjectURL = vi.fn(() => "blob:doc");
    URL.revokeObjectURL = vi.fn();
  });

  it("asks the row endpoint for the ref and names the drawer by the row title", async () => {
    mockApi({ row: remote });
    renderModal(remoteRef);

    expect(
      await screen.findByRole("dialog", { name: "Laboratorio" }),
    ).toBeVisible();
    expect(mocks.post).toHaveBeenCalledWith(
      "/api/reports/row",
      expect.objectContaining({ body: { ref: remoteRef } }),
    );
    expect(screen.getByText("Ospedale Esempio · gio 1 ott 2026")).toBeVisible();
    expect(screen.queryByText("Caricato")).toBeNull();
  });

  it("shows the Caricato badge and the title for a local row", async () => {
    mockApi({ row: local });
    renderModal(localRef);

    expect(
      await screen.findByRole("dialog", { name: "Ecografia addome" }),
    ).toBeVisible();
    expect(screen.getByText("Caricato")).toBeVisible();
    expect(await screen.findByRole("tab", { name: "Modifica" })).toBeVisible();
  });

  it("has no edit tab for a remote row", async () => {
    mockApi({ row: remote });
    renderModal(remoteRef);

    await screen.findByRole("button", { name: "Apri Documento" });
    expect(screen.queryByRole("tab", { name: "Modifica" })).toBeNull();
  });

  it("counts the linked visits from the row", async () => {
    mockApi({ row: local });
    renderModal(localRef);

    expect(
      await screen.findByRole("tab", { name: /Visite collegate/ }),
    ).toHaveTextContent("1");
  });

  it("says when the row is not found", async () => {
    mockApi({ row: undefined, rowStatus: 404 });
    renderModal(remoteRef);

    expect(await screen.findByText("Referto non trovato.")).toBeVisible();
  });

  it("opens a PDF in the viewer", async () => {
    mockApi({ row: remote });
    renderModal(remoteRef);

    await userEvent.click(
      await screen.findByRole("button", { name: "Apri Documento" }),
    );

    expect(
      await screen.findByRole("link", { name: "Apri in una nuova scheda" }),
    ).toHaveAttribute("href", "blob:doc");
  });

  it("opens an image as a data URL", async () => {
    mockApi({
      row: local,
      documents: [{ position: "1", label: "Radiografia" }],
      blob: new Blob(["png"], { type: "image/png" }),
    });
    renderModal(localRef);

    await userEvent.click(
      await screen.findByRole("button", { name: "Apri Radiografia" }),
    );

    const image = await screen.findByRole("img", { name: "Radiografia" });
    expect(image.getAttribute("src")).toMatch(/^data:image\/png;base64,/);
  });

  it("caps long document lists", async () => {
    mockApi({
      row: remote,
      documents: Array.from({ length: 7 }, (_, index) => ({
        position: String(index + 1),
        label: `Documento ${String(index + 1)}`,
      })),
    });
    renderModal(remoteRef);

    await userEvent.click(
      await screen.findByRole("button", { name: "Mostra altri 2" }),
    );

    expect(screen.getByText("Documento 7")).toBeVisible();
  });

  it("reloads the row and notifies after unlinking a visit", async () => {
    mockApi({ row: local });
    const { onChanged } = renderModal(localRef);
    await userEvent.click(
      await screen.findByRole("tab", { name: /Visite collegate/ }),
    );

    await userEvent.click(
      await screen.findByRole("button", { name: "Scollega 0123A" }),
    );

    await waitFor(() => {
      expect(onChanged).toHaveBeenCalledTimes(1);
    });
    const rowCalls = mocks.post.mock.calls.filter(
      (call: readonly unknown[]) => call[0] === "/api/reports/row",
    );
    expect(rowCalls).toHaveLength(2);
  });

  it("closes and notifies after saving a local report", async () => {
    mockApi({ row: local });
    const { onChanged, onClose } = renderModal(localRef);
    await userEvent.click(await screen.findByRole("tab", { name: "Modifica" }));

    await userEvent.click(
      await screen.findByRole("button", { name: "Salva modifiche" }),
    );

    await waitFor(() => {
      expect(onChanged).toHaveBeenCalledTimes(1);
    });
    expect(onClose).toHaveBeenCalled();
  });
});
