import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ReportBody, RowBody } from "../../api/client";
import { render as plainRender } from "../../test/render";
import { WithHealth } from "../../test/WithHealth";
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

const remote: ReportBody = {
  kind: "lab",
  kindLabel: "Laboratorio",
  abstractId: "a1",
  detailId: "d1",
  repCode: "r1",
  date: "2026-10-01T09:00:00Z",
  provider: "Ospedale Esempio",
  unread: false,
  hidden: false,
  origin: "remote",
  nres: null,
};

const local: ReportBody = {
  kind: "local",
  kindLabel: "Caricato",
  abstractId: "L1",
  detailId: "",
  repCode: "",
  date: "2026-10-05T09:00:00Z",
  provider: "Studio Prova",
  title: "Ecografia addome",
  unread: false,
  hidden: false,
  origin: "local",
  nres: ["0123A"],
};

const rows: RowBody[] = [
  {
    nre: "0123A",
    overridden: false,
    reports: null,
    fse: {
      id: "p1",
      nre: "0123A",
      description: "Ecografia addome",
      kind: "visit",
      status: 3,
      documentRef: "",
    },
  },
  {
    nre: "0456B",
    overridden: false,
    reports: null,
    fse: {
      id: "p2",
      nre: "0456B",
      description: "Visita cardiologica",
      kind: "visit",
      status: 3,
      documentRef: "",
    },
  },
];

function render(report: ReportBody): {
  onClose: () => void;
  onChanged: () => void;
} {
  const onClose = vi.fn();
  const onChanged = vi.fn();
  plainRender(
    <WithHealth rows={rows}>
      <ReportDetailModal
        report={report}
        onClose={onClose}
        onChanged={onChanged}
      />
    </WithHealth>,
  );
  return { onClose, onChanged };
}

const createUrl = vi.fn(() => "blob:doc");

function topDialog(): HTMLElement {
  const dialog = screen.getAllByRole("dialog").at(-1);
  if (dialog === undefined) {
    throw new Error("missing dialog");
  }
  return dialog;
}

function mockDocuments(blob: Blob, label = "Documento"): void {
  mocks.post.mockImplementation((path: string) =>
    Promise.resolve(
      path === "/api/reports/documents"
        ? {
            data: { documents: [{ position: "1", label }] },
            response: { status: 200 },
          }
        : { data: blob, response: { status: 200 } },
    ),
  );
}

describe("ReportDetailModal", () => {
  beforeEach(() => {
    mocks.post.mockReset();
    mocks.put.mockReset();
    mocks.del.mockReset();
    mocks.put.mockResolvedValue({ response: { ok: true } });
    mocks.del.mockResolvedValue({ response: { ok: true } });
    createUrl.mockClear();
    URL.createObjectURL = createUrl;
    URL.revokeObjectURL = vi.fn();
  });

  describe("viewer selection", () => {
    it("opens a PDF in the PDF viewer", async () => {
      mockDocuments(new Blob(["pdf"], { type: "application/pdf" }));
      render(remote);

      await userEvent.click(
        await screen.findByRole("button", { name: "Apri PDF" }),
      );

      expect(
        await screen.findByRole("link", { name: "Apri in una nuova scheda" }),
      ).toHaveAttribute("href", "blob:doc");
      expect(screen.queryByRole("img", { name: "Documento" })).toBeNull();
    });

    it("opens an image in the image viewer as a data URL", async () => {
      mockDocuments(new Blob(["png"], { type: "image/png" }), "Radiografia");
      render(local);

      await userEvent.click(
        await screen.findByRole("button", { name: "Apri" }),
      );

      const image = await screen.findByRole("img", { name: "Radiografia" });
      expect(image.getAttribute("src")).toMatch(/^data:image\/png;base64,/);
      expect(createUrl).not.toHaveBeenCalled();
      expect(
        screen.queryByRole("link", { name: "Apri in una nuova scheda" }),
      ).toBeNull();
    });
  });

  describe("remote report", () => {
    it("has no edit section", async () => {
      mockDocuments(new Blob(["pdf"]));
      render(remote);
      await screen.findByRole("button", { name: "Apri PDF" });

      expect(
        screen.queryByRole("button", { name: "Elimina referto" }),
      ).toBeNull();
    });
  });

  describe("local report", () => {
    beforeEach(() => {
      mockDocuments(new Blob(["pdf"], { type: "application/pdf" }));
    });

    it("prefills the editor and saves the changes", async () => {
      const { onChanged, onClose } = render(local);

      expect(await screen.findByLabelText("Titolo")).toHaveValue(
        "Ecografia addome",
      );
      expect(screen.getByLabelText("Data")).toHaveValue("2026-10-05");
      expect(screen.getByLabelText("Struttura")).toHaveValue("Studio Prova");

      await userEvent.clear(screen.getByLabelText("Titolo"));
      await userEvent.type(screen.getByLabelText("Titolo"), "Eco controllo");
      await userEvent.click(
        screen.getByRole("button", { name: "Salva modifiche" }),
      );

      await waitFor(() => {
        expect(onChanged).toHaveBeenCalledTimes(1);
      });
      expect(mocks.put).toHaveBeenCalledWith("/api/reports/local", {
        body: {
          id: "L1",
          title: "Eco controllo",
          date: "2026-10-05",
          provider: "Studio Prova",
        },
      });
      expect(onClose).toHaveBeenCalled();
    });

    it("requires a title", async () => {
      render(local);

      await userEvent.clear(await screen.findByLabelText("Titolo"));
      await userEvent.click(
        screen.getByRole("button", { name: "Salva modifiche" }),
      );

      expect(await screen.findByText("Il titolo è obbligatorio")).toBeVisible();
      expect(mocks.put).not.toHaveBeenCalled();
    });

    it("shows an error when saving fails", async () => {
      mocks.put.mockResolvedValue({ response: { ok: false } });
      const { onChanged } = render(local);

      await userEvent.click(
        await screen.findByRole("button", { name: "Salva modifiche" }),
      );

      expect(await screen.findByText("Salvataggio non riuscito")).toBeVisible();
      expect(onChanged).not.toHaveBeenCalled();
    });

    it("asks for an in-app confirmation before deleting", async () => {
      const confirm = vi.spyOn(window, "confirm");
      const { onChanged, onClose } = render(local);

      await userEvent.click(
        await screen.findByRole("button", { name: "Elimina referto" }),
      );

      expect(await screen.findByText("Eliminare il referto?")).toBeVisible();
      expect(mocks.del).not.toHaveBeenCalled();

      await userEvent.click(
        within(topDialog()).getByRole("button", { name: "Elimina" }),
      );

      await waitFor(() => {
        expect(onChanged).toHaveBeenCalledTimes(1);
      });
      expect(mocks.del).toHaveBeenCalledWith("/api/reports/local", {
        body: { id: "L1" },
      });
      expect(onClose).toHaveBeenCalled();
      expect(confirm).not.toHaveBeenCalled();
    });

    it("does not delete when the confirmation is dismissed", async () => {
      render(local);

      await userEvent.click(
        await screen.findByRole("button", { name: "Elimina referto" }),
      );
      await userEvent.click(
        await screen.findByRole("button", { name: "Annulla" }),
      );

      expect(mocks.del).not.toHaveBeenCalled();
    });

    it("shows an error when deleting fails", async () => {
      mocks.del.mockResolvedValue({ response: { ok: false } });
      const { onChanged } = render(local);

      await userEvent.click(
        await screen.findByRole("button", { name: "Elimina referto" }),
      );
      await userEvent.click(
        await screen.findByRole("button", { name: "Elimina" }),
      );

      expect(
        await screen.findByText("Eliminazione non riuscita"),
      ).toBeVisible();
      expect(onChanged).not.toHaveBeenCalled();
    });
  });

  describe("linked visits", () => {
    beforeEach(() => {
      mockDocuments(new Blob(["pdf"]));
    });

    it("lists the linked visits resolved to their description", async () => {
      render(local);

      const section = (await screen.findByText("Visite collegate")).closest(
        "section",
      );
      expect(section).not.toBeNull();
      expect(screen.getAllByText("0123A").length).toBeGreaterThan(0);
      expect(
        screen.getByRole("button", { name: "Scollega 0123A" }),
      ).toBeVisible();
    });

    it("says when there are no linked visits", async () => {
      render(remote);

      expect(
        await screen.findByText("Nessuna visita collegata."),
      ).toBeVisible();
    });

    it("unlinks a visit", async () => {
      mocks.del.mockResolvedValue({ response: { ok: true } });
      const { onChanged } = render(local);

      await userEvent.click(
        await screen.findByRole("button", { name: "Scollega 0123A" }),
      );

      await waitFor(() => {
        expect(onChanged).toHaveBeenCalledTimes(1);
      });
      expect(mocks.del).toHaveBeenCalledWith("/api/links", {
        body: {
          nre: "0123A",
          report: {
            kind: "local",
            abstractId: "L1",
            detailId: "",
            repCode: "",
          },
        },
      });
      expect(
        await screen.findByText("Nessuna visita collegata."),
      ).toBeVisible();
    });

    it("links a visit not yet linked and offers only unlinked ones", async () => {
      const { onChanged } = render(local);

      await userEvent.click(
        await screen.findByRole("combobox", { name: "Collega a visita" }),
      );
      expect(
        screen.queryByRole("option", { name: /Ecografia addome · 0123A/ }),
      ).toBeNull();
      await userEvent.click(
        await screen.findByRole("option", {
          name: "Visita cardiologica · 0456B",
        }),
      );

      await waitFor(() => {
        expect(onChanged).toHaveBeenCalledTimes(1);
      });
      expect(mocks.put).toHaveBeenCalledWith("/api/links", {
        body: {
          nre: "0456B",
          report: {
            kind: "local",
            abstractId: "L1",
            detailId: "",
            repCode: "",
          },
        },
      });
      expect(
        await screen.findByRole("button", { name: "Scollega 0456B" }),
      ).toBeVisible();
    });

    it("shows an error when linking fails", async () => {
      mocks.put.mockResolvedValue({ response: { ok: false } });
      const { onChanged } = render(remote);

      await userEvent.click(
        await screen.findByRole("combobox", { name: "Collega a visita" }),
      );
      await userEvent.click(
        await screen.findByRole("option", {
          name: "Visita cardiologica · 0456B",
        }),
      );

      expect(await screen.findByText("Operazione non riuscita")).toBeVisible();
      expect(onChanged).not.toHaveBeenCalled();
    });
  });
});
