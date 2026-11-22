import {
  screen,
  waitFor,
  within,
  type RenderResult,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ReportBody, ReportsBody } from "../api/client";
import { callsTo } from "../test/calls";
import { render as plainRender } from "../test/render";
import { WithHealth } from "../test/WithHealth";
import { ReportsView } from "./ReportsView";

vi.mock("../lib/pdf", () => ({
  loadPdf: () => ({
    promise: new Promise<never>(() => undefined),
    destroy: () => Promise.resolve(),
  }),
}));

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  refresh: vi.fn(),
}));

vi.mock("../api/client", () => ({
  api: { GET: mocks.get, POST: mocks.post },
}));

vi.mock("./reports/UploadReportDialog", () => ({
  UploadReportDialog: ({
    opened,
    onClose,
    onUploaded,
  }: {
    opened: boolean;
    onClose: () => void;
    onUploaded: (report: ReportBody) => void;
  }) =>
    opened ? (
      <div role="dialog" aria-label="Carica referto">
        <button
          type="button"
          onClick={() => {
            onUploaded(lab);
          }}
        >
          Simula caricamento
        </button>
        <button type="button" onClick={onClose}>
          Chiudi finestra
        </button>
      </div>
    ) : null,
}));

function render(ui: ReactNode): RenderResult {
  return plainRender(<WithHealth rows={[]}>{ui}</WithHealth>);
}

vi.mock("../data/refresh", () => ({ refreshPortals: mocks.refresh }));

const now = new Date("2026-11-10T12:00:00Z");

const lab: ReportBody = {
  kind: "lab",
  kindLabel: "Laboratorio",
  abstractId: "a1",
  detailId: "d1",
  repCode: "r1",
  date: "2026-10-01T09:00:00Z",
  provider: "Ospedale Esempio",
  unread: true,
  hidden: false,
  origin: "remote",
  nres: null,
};

const radio: ReportBody = {
  kind: "rad",
  kindLabel: "Radiologia",
  abstractId: "a2",
  detailId: "d2",
  repCode: "r2",
  date: "2026-11-01T09:00:00Z",
  provider: "Clinica Prova",
  unread: false,
  hidden: true,
  origin: "remote",
  nres: null,
};

const older: ReportBody = {
  kind: "lab",
  kindLabel: "Laboratorio",
  abstractId: "a3",
  detailId: "d3",
  repCode: "r3",
  date: "2026-09-01T09:00:00Z",
  provider: "Centro Campione",
  unread: false,
  hidden: false,
  origin: "remote",
  nres: null,
};

const uploaded: ReportBody = {
  kind: "local",
  kindLabel: "Caricato",
  abstractId: "L1",
  detailId: "",
  repCode: "",
  date: "2026-11-05T09:00:00Z",
  provider: "Studio Prova",
  title: "Ecografia addome",
  unread: false,
  hidden: false,
  origin: "local",
  nres: ["0123A", "0456B"],
};

function body(overrides: Partial<ReportsBody> = {}): ReportsBody {
  return {
    reports: [lab, radio, older],
    failures: [],
    updated: "2026-11-10T08:30:00Z",
    ...overrides,
  };
}

function mockList(data: ReportsBody): void {
  mocks.get.mockResolvedValue({ data, response: { status: 200 } });
}

function bodyRows(): HTMLElement[] {
  return screen.getAllByRole("row").slice(1);
}

function bodyRow(index: number): HTMLElement {
  const row = bodyRows()[index];
  if (row === undefined) {
    throw new Error(`missing row ${String(index)}`);
  }
  return row;
}

function topDialog(): HTMLElement {
  const dialog = screen.getAllByRole("dialog").at(-1);
  if (dialog === undefined) {
    throw new Error("missing dialog");
  }
  return dialog;
}

describe("ReportsView", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    mocks.post.mockReset();
    mocks.refresh.mockReset();
  });

  it("renders the rows sorted by date descending", async () => {
    mockList(body());
    render(<ReportsView now={now} />);

    await screen.findByText("Clinica Prova");

    const rows = bodyRows();
    expect(rows).toHaveLength(3);
    expect(rows[0]).toHaveTextContent("Clinica Prova");
    expect(rows[1]).toHaveTextContent("Ospedale Esempio");
    expect(rows[2]).toHaveTextContent("Centro Campione");
  });

  it("shows the unread and hidden badges", async () => {
    mockList(body());
    render(<ReportsView now={now} />);

    await screen.findByText("Clinica Prova");

    expect(within(bodyRow(1)).getByText("Nuovo")).toBeVisible();
    expect(within(bodyRow(0)).getByText("Oscurato")).toBeVisible();
    expect(screen.getAllByText("Nuovo")).toHaveLength(1);
  });

  it("filters by search text", async () => {
    mockList(body());
    render(<ReportsView now={now} />);
    await screen.findByText("Clinica Prova");

    await userEvent.type(
      screen.getByRole("searchbox", { name: "Cerca" }),
      "campione",
    );

    expect(bodyRows()).toHaveLength(1);
    expect(screen.getByText("Centro Campione")).toBeVisible();
    expect(screen.getByText("1 referti")).toBeVisible();
  });

  it("filters by type", async () => {
    mockList(body());
    render(<ReportsView now={now} />);
    await screen.findByText("Clinica Prova");

    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Tipo" }),
      "Radiologia",
    );

    expect(bodyRows()).toHaveLength(1);
    expect(screen.getByText("Clinica Prova")).toBeVisible();
  });

  it("shows when the data was updated", async () => {
    mockList(body());
    render(<ReportsView now={now} />);

    expect(await screen.findByText("Aggiornato alle 09:30")).toBeVisible();
  });

  it("lists the failed sections", async () => {
    mockList(
      body({
        failures: [
          { kind: "rsa", kindLabel: "RSA", message: "errore" },
          { kind: "rap", kindLabel: "RAP", message: "errore" },
        ],
      }),
    );
    render(<ReportsView now={now} />);

    expect(
      await screen.findByText(
        "Alcune sezioni non sono state caricate: RSA, RAP",
      ),
    ).toBeVisible();
  });

  it("marks uploaded reports with an icon and shows their title", async () => {
    mockList(body({ reports: [lab, uploaded] }));
    render(<ReportsView now={now} />);

    await screen.findByText("Ecografia addome");

    const row = bodyRow(0);
    expect(
      within(row).getByRole("img", { name: "Caricato da te" }),
    ).toBeVisible();
    expect(within(row).queryByText("Caricato")).toBeNull();
    expect(
      within(bodyRow(1)).queryByRole("img", { name: "Caricato da te" }),
    ).toBeNull();
  });

  it("shows how many visits a report is linked to", async () => {
    mockList(body({ reports: [lab, uploaded] }));
    render(<ReportsView now={now} />);
    await screen.findByText("Ecografia addome");

    expect(
      within(bodyRow(0)).getByRole("img", { name: "Collegato a 2 visite" }),
    ).toBeVisible();
    expect(
      within(bodyRow(1)).queryByRole("img", { name: /Collegato/ }),
    ).toBeNull();
  });

  it("uses the singular for a single linked visit", async () => {
    mockList(body({ reports: [{ ...uploaded, nres: ["0123A"] }] }));
    render(<ReportsView now={now} />);

    expect(
      await screen.findByRole("img", { name: "Collegato a 1 visita" }),
    ).toBeVisible();
  });

  it("finds an uploaded report by its title", async () => {
    mockList(body({ reports: [lab, uploaded] }));
    render(<ReportsView now={now} />);
    await screen.findByText("Ecografia addome");

    await userEvent.type(
      screen.getByRole("searchbox", { name: "Cerca" }),
      "addome",
    );

    expect(bodyRows()).toHaveLength(1);
  });

  it("hints to log in to the FSE and keeps the local reports", async () => {
    mockList(
      body({
        reports: [uploaded],
        failures: [
          { kind: "fse", kindLabel: "Fascicolo Sanitario", message: "login" },
        ],
      }),
    );
    render(<ReportsView now={now} />);

    expect(
      await screen.findByText(
        "Accedi al Fascicolo Sanitario per vedere anche i referti del portale regionale.",
      ),
    ).toBeVisible();
    expect(screen.getByText("Ecografia addome")).toBeVisible();
    expect(screen.queryByText(/Alcune sezioni/)).toBeNull();
  });

  it("opens the upload dialog and reloads the list after an upload", async () => {
    mockList(body());
    render(<ReportsView now={now} />);
    await screen.findByText("Clinica Prova");

    await userEvent.click(
      screen.getByRole("button", { name: "Carica referto" }),
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Simula caricamento" }),
    );

    await waitFor(() => {
      expect(mocks.get).toHaveBeenCalledTimes(2);
    });
    expect(screen.getByText("Clinica Prova")).toBeVisible();
  });

  it("mounts the upload dialog only while open", async () => {
    mockList(body());
    render(<ReportsView now={now} />);
    await screen.findByText("Clinica Prova");
    expect(screen.queryByRole("dialog", { name: "Carica referto" })).toBeNull();

    await userEvent.click(
      screen.getByRole("button", { name: "Carica referto" }),
    );
    expect(
      screen.getByRole("dialog", { name: "Carica referto" }),
    ).toBeVisible();

    await userEvent.click(
      screen.getByRole("button", { name: "Chiudi finestra" }),
    );
    expect(screen.queryByRole("dialog", { name: "Carica referto" })).toBeNull();
  });

  it("shows the empty state", async () => {
    mockList(body({ reports: [], updated: null }));
    render(<ReportsView now={now} />);

    expect(await screen.findByText("Nessun referto disponibile")).toBeVisible();
  });

  it("asks to log in on 401", async () => {
    mocks.get.mockResolvedValue({
      data: undefined,
      response: { status: 401 },
    });
    render(<ReportsView now={now} />);

    expect(
      await screen.findByText("Accedi per vedere i referti."),
    ).toBeVisible();
  });

  it("shows an error when the portal is unavailable", async () => {
    mocks.get.mockRejectedValue(new Error("down"));
    render(<ReportsView now={now} />);

    expect(
      await screen.findByText(
        "Il portale regionale non risponde. Riprova tra poco.",
      ),
    ).toBeVisible();
  });

  it("refreshes everything and reloads the list", async () => {
    mockList(body());
    mocks.refresh.mockResolvedValue(true);
    render(<ReportsView now={now} />);
    await screen.findByText("Clinica Prova");

    await userEvent.click(
      screen.getByRole("button", { name: "Aggiorna tutto" }),
    );

    await waitFor(() => {
      expect(mocks.get).toHaveBeenCalledTimes(2);
    });
    expect(mocks.refresh).toHaveBeenCalledWith({ all: true });
  });

  it("notifies when the refresh fails", async () => {
    mockList(body());
    mocks.refresh.mockResolvedValue(false);
    render(<ReportsView now={now} />);
    await screen.findByText("Clinica Prova");

    await userEvent.click(
      screen.getByRole("button", { name: "Aggiorna tutto" }),
    );

    expect(await screen.findByText("Aggiornamento non riuscito")).toBeVisible();
  });

  describe("detail", () => {
    const createUrl = vi.fn(() => "blob:report");
    const revokeUrl = vi.fn();

    beforeEach(() => {
      createUrl.mockClear();
      revokeUrl.mockClear();
      URL.createObjectURL = createUrl;
      URL.revokeObjectURL = revokeUrl;
      mockList(body());
    });

    it("loads the documents with the exact request and opens the PDF", async () => {
      mocks.post.mockImplementation((path: string) => {
        if (path === "/api/reports/documents") {
          return Promise.resolve({
            data: {
              documents: [
                {
                  position: "2",
                  label: "Esame del sangue",
                  acceptedAt: "2026-10-01T10:00:00Z",
                },
              ],
            },
            response: { status: 200 },
          });
        }
        return Promise.resolve({
          data: new Blob(["pdf"]),
          response: { status: 200 },
        });
      });
      render(<ReportsView now={now} />);
      await screen.findByText("Ospedale Esempio");

      await userEvent.click(screen.getByText("Ospedale Esempio"));

      const dialog = await screen.findByRole("dialog");
      expect(within(dialog).getByText("Esame del sangue")).toBeVisible();
      expect(mocks.post.mock.calls[0]?.[0]).toBe("/api/reports/documents");
      expect(mocks.post.mock.calls[0]?.[1]).toMatchObject({
        body: {
          kind: "lab",
          abstractId: "a1",
          detailId: "d1",
          repCode: "r1",
        },
      });

      await userEvent.click(
        within(dialog).getByRole("button", { name: "Apri Esame del sangue" }),
      );

      await waitFor(() => {
        expect(createUrl).toHaveBeenCalledTimes(1);
      });
      expect(mocks.post).toHaveBeenCalledWith("/api/reports/pdf", {
        body: {
          kind: "lab",
          abstractId: "a1",
          detailId: "d1",
          repCode: "r1",
          position: "2",
        },
        parseAs: "blob",
      });
      expect(
        await screen.findByRole("link", { name: "Apri in una nuova scheda" }),
      ).toHaveAttribute("href", "blob:report");

      await userEvent.click(
        within(topDialog()).getByRole("button", { name: "Chiudi" }),
      );

      await waitFor(() => {
        expect(revokeUrl).toHaveBeenCalledWith("blob:report");
      });
    });

    it("opens the detail with Enter", async () => {
      mocks.post.mockResolvedValue({
        data: { documents: [] },
        response: { status: 200 },
      });
      render(<ReportsView now={now} />);
      await screen.findByText("Ospedale Esempio");

      bodyRow(1).focus();
      await userEvent.keyboard("{Enter}");

      expect(
        await screen.findByText("Nessun documento disponibile."),
      ).toBeVisible();
    });

    it("shows the 401 message when documents need a login", async () => {
      mocks.post.mockResolvedValue({
        data: undefined,
        response: { status: 401 },
      });
      render(<ReportsView now={now} />);
      await screen.findByText("Ospedale Esempio");

      await userEvent.click(screen.getByText("Ospedale Esempio"));

      const dialog = await screen.findByRole("dialog");
      expect(
        await within(dialog).findByText("Accedi per vedere i referti."),
      ).toBeVisible();
    });

    it("shows an inline error when the PDF fails", async () => {
      mocks.post.mockImplementation((path: string) => {
        if (path === "/api/reports/documents") {
          return Promise.resolve({
            data: { documents: [{ position: "", label: "Referto" }] },
            response: { status: 200 },
          });
        }
        return Promise.resolve({ data: undefined, response: { status: 500 } });
      });
      render(<ReportsView now={now} />);
      await screen.findByText("Ospedale Esempio");

      await userEvent.click(screen.getByText("Ospedale Esempio"));
      await userEvent.click(
        await screen.findByRole("button", { name: "Apri Referto" }),
      );

      expect(
        await screen.findByText("Impossibile aprire il PDF."),
      ).toBeVisible();
      expect(createUrl).not.toHaveBeenCalled();
    });

    it("never calls anything beyond the two report endpoints", async () => {
      mocks.post.mockResolvedValue({
        data: { documents: [] },
        response: { status: 200 },
      });
      render(<ReportsView now={now} />);
      await screen.findByText("Ospedale Esempio");

      await userEvent.click(screen.getByText("Ospedale Esempio"));
      await screen.findByText("Nessun documento disponibile.");

      expect(callsTo(mocks.post, "/api/reports/documents")).toBe(1);
      expect(callsTo(mocks.post, "/api/reports/pdf")).toBe(0);
      expect(mocks.get).toHaveBeenCalledTimes(1);
    });
  });
});
