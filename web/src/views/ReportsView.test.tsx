import {
  act,
  render as rtlRender,
  screen,
  waitFor,
  within,
  type RenderResult,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { ReportRowBody, ReportsQueryBody } from "../api/client";
import { TestProviders } from "../test/TestProviders";
import { WithSession } from "../test/WithSession";
import { ReportsView } from "./ReportsView";

vi.mock("../lib/pdf", () => ({
  loadPdf: () => ({
    promise: new Promise<never>(() => undefined),
    destroy: () => Promise.resolve(),
  }),
}));

const mocks = vi.hoisted(() => ({
  post: vi.fn(),
  startRefresh: vi.fn(),
}));

vi.mock("../api/client", () => ({
  api: { POST: mocks.post },
}));

vi.mock("../data/refresh", () => ({ startRefresh: mocks.startRefresh }));

vi.mock("./reports/UploadReportDialog", () => ({
  UploadReportDialog: ({
    onClose,
    onUploaded,
  }: {
    onClose: () => void;
    onUploaded: () => void;
  }) => (
    <div role="dialog" aria-label="Carica referto">
      <button type="button" onClick={onUploaded}>
        Simula caricamento
      </button>
      <button type="button" onClick={onClose}>
        Chiudi finestra
      </button>
    </div>
  ),
}));

function render(ui: ReactNode): RenderResult {
  return rtlRender(<WithSession>{ui}</WithSession>, {
    wrapper: TestProviders,
  });
}

function row(overrides: Partial<ReportRowBody>): ReportRowBody {
  return {
    id: "lab|a1|d1|r1",
    kind: "lab",
    kindLabel: "Laboratorio",
    title: "Laboratorio",
    ref: { kind: "lab", abstractId: "a1", detailId: "d1", repCode: "r1" },
    date: "2026-10-01T09:00:00Z",
    provider: "Ospedale Esempio",
    unread: false,
    hidden: false,
    origin: "remote",
    visits: [],
    linkedLabel: "",
    ...overrides,
  };
}

const lab = row({ unread: true });
const radio = row({
  id: "ris|a2|d2|r2",
  kind: "ris",
  kindLabel: "Radiologia",
  title: "Radiologia",
  ref: { kind: "ris", abstractId: "a2", detailId: "d2", repCode: "r2" },
  date: "2026-11-01T09:00:00Z",
  provider: "Clinica Prova",
  hidden: true,
});
const uploaded = row({
  id: "local|L1",
  kind: "local",
  kindLabel: "Caricato",
  title: "Ecografia addome",
  ref: { kind: "local", abstractId: "L1", detailId: "", repCode: "" },
  provider: "Studio Prova",
  origin: "local",
  visits: [
    { nre: "0123A", title: "Visita uno" },
    { nre: "0456B", title: "Visita due" },
  ],
  linkedLabel: "Collegato a 2 visite",
});

function body(overrides: Partial<ReportsQueryBody> = {}): ReportsQueryBody {
  return {
    rows: [radio, lab],
    total: 2,
    page: 1,
    pageSize: 50,
    kinds: [
      { key: "lab", label: "Laboratorio", count: 1 },
      { key: "ris", label: "Radiologia", count: 1 },
    ],
    failures: [],
    fseUnavailable: false,
    refreshing: false,
    updated: { at: "2026-11-10T08:30:00Z", label: "Aggiornato alle 09:30" },
    issue: "none",
    ...overrides,
  };
}

function mockQuery(...responses: ReportsQueryBody[]): void {
  let index = 0;
  mocks.post.mockImplementation((path: string) => {
    if (path !== "/api/reports/query") {
      return Promise.resolve({
        data: undefined,
        response: { status: 404 },
      });
    }
    const data = responses[Math.min(index, responses.length - 1)];
    index += 1;
    return Promise.resolve({ data, response: { status: 200 } });
  });
}

function queryRequests(): unknown[] {
  return mocks.post.mock.calls
    .filter((call) => call[0] === "/api/reports/query")
    .map((call) => (call[1] as { body: unknown }).body);
}

function bodyRows(): HTMLElement[] {
  return screen.getAllByRole("row").slice(1);
}

function bodyRow(index: number): HTMLElement {
  const found = bodyRows()[index];
  if (found === undefined) {
    throw new Error(`missing row ${String(index)}`);
  }
  return found;
}

describe("ReportsView", () => {
  beforeEach(() => {
    mocks.post.mockReset();
    mocks.startRefresh.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders the rows as returned and requests the defaults", async () => {
    mockQuery(body());
    render(<ReportsView />);

    await screen.findByText("Clinica Prova");

    expect(bodyRows()).toHaveLength(2);
    expect(bodyRow(0)).toHaveTextContent("Clinica Prova");
    expect(bodyRow(1)).toHaveTextContent("Ospedale Esempio");
    expect(screen.getByText("2 referti")).toBeVisible();
    expect(screen.getByText("Aggiornato alle 09:30")).toBeVisible();
    expect(queryRequests()[0]).toEqual({
      search: "",
      sort: "date",
      dir: "desc",
      page: 1,
      pageSize: 50,
    });
  });

  it("shows the unread, hidden and linked markers", async () => {
    mockQuery(body({ rows: [radio, lab, uploaded], total: 3 }));
    render(<ReportsView />);
    await screen.findByText("Clinica Prova");

    expect(within(bodyRow(0)).getByText("Oscurato")).toBeVisible();
    expect(within(bodyRow(1)).getByText("Nuovo")).toBeVisible();
    expect(
      within(bodyRow(2)).getByRole("img", {
        name: "Collegato a 2 visite",
      }),
    ).toBeVisible();
    expect(
      within(bodyRow(2)).getByRole("img", {
        name: "Caricato da te",
      }),
    ).toBeVisible();
    expect(screen.getByText("Ecografia addome")).toBeVisible();
  });

  it("sends the search text in the request", async () => {
    mockQuery(body());
    render(<ReportsView />);
    await screen.findByText("Clinica Prova");

    await userEvent.type(
      screen.getByRole("searchbox", { name: "Cerca" }),
      "campione",
    );

    await waitFor(() => {
      expect(queryRequests().at(-1)).toMatchObject({
        search: "campione",
        page: 1,
      });
    });
  });

  it("lists the kinds with their counts and sends the selected kind", async () => {
    mockQuery(body());
    render(<ReportsView />);
    await screen.findByText("Clinica Prova");

    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Tipo" }),
      "Radiologia (1)",
    );

    await waitFor(() => {
      expect(queryRequests().at(-1)).toMatchObject({ kind: "ris", page: 1 });
    });
  });

  it("sends the sort column and direction", async () => {
    mockQuery(body());
    render(<ReportsView />);
    await screen.findByText("Clinica Prova");

    await userEvent.click(screen.getByRole("button", { name: /Struttura/ }));

    await waitFor(() => {
      expect(queryRequests().at(-1)).toMatchObject({
        sort: "provider",
        dir: "asc",
        page: 1,
      });
    });

    await userEvent.click(screen.getByRole("button", { name: /^Tipo/ }));

    await waitFor(() => {
      expect(queryRequests().at(-1)).toMatchObject({ sort: "kind" });
    });
  });

  it("changes the page size and goes back to the first page", async () => {
    mockQuery(body({ total: 120 }));
    render(<ReportsView />);
    await screen.findByText("Clinica Prova");

    await userEvent.click(await screen.findByRole("button", { name: "2" }));
    await waitFor(() => {
      expect(queryRequests().at(-1)).toMatchObject({ page: 2, pageSize: 50 });
    });

    await userEvent.click(await screen.findByRole("button", { name: "50" }));
    await userEvent.click(await screen.findByRole("menuitem", { name: "25" }));

    await waitFor(() => {
      expect(queryRequests().at(-1)).toMatchObject({ page: 1, pageSize: 25 });
    });
  });

  it("starts a refresh and queries again", async () => {
    mockQuery(body());
    mocks.startRefresh.mockResolvedValue(true);
    render(<ReportsView />);
    await screen.findByText("Clinica Prova");
    const before = queryRequests().length;

    await userEvent.click(
      screen.getByRole("button", { name: "Aggiorna tutto" }),
    );

    expect(mocks.startRefresh).toHaveBeenCalledWith({ reports: true });
    await waitFor(() => {
      expect(queryRequests().length).toBe(before + 1);
    });
  });

  it("notifies when the refresh cannot start", async () => {
    mockQuery(body());
    mocks.startRefresh.mockResolvedValue(false);
    render(<ReportsView />);
    await screen.findByText("Clinica Prova");

    await userEvent.click(
      screen.getByRole("button", { name: "Aggiorna tutto" }),
    );

    expect(await screen.findByText("Aggiornamento non riuscito")).toBeVisible();
  });

  it("polls while the backend is refreshing", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mockQuery(body({ refreshing: true }), body({ refreshing: false }));
    render(<ReportsView />);
    await screen.findByText("Clinica Prova");
    expect(queryRequests()).toHaveLength(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3100);
    });

    expect(queryRequests()).toHaveLength(2);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10000);
    });
    expect(queryRequests()).toHaveLength(2);
  });

  it("lists the failed sections and hints at the FSE login", async () => {
    mockQuery(
      body({
        fseUnavailable: true,
        failures: [
          { kind: "rsa", kindLabel: "RSA", message: "errore" },
          { kind: "rap", kindLabel: "RAP", message: "errore" },
        ],
      }),
    );
    render(<ReportsView />);

    expect(
      await screen.findByText(
        "Alcune sezioni non sono state caricate: RSA, RAP",
      ),
    ).toBeVisible();
    expect(
      screen.getByText(
        "Accedi al Fascicolo Sanitario per vedere anche i referti del portale regionale.",
      ),
    ).toBeVisible();
  });

  it("asks to log in when the backend reports login_required", async () => {
    mockQuery(body({ rows: [], total: 0, issue: "login_required" }));
    render(<ReportsView />);

    expect(
      await screen.findByText("Accedi per vedere i referti."),
    ).toBeVisible();
  });

  it("shows an error when the portal is unavailable", async () => {
    mockQuery(body({ rows: [], total: 0, issue: "unavailable" }));
    render(<ReportsView />);

    expect(
      await screen.findByText(
        "Il portale regionale non risponde. Riprova tra poco.",
      ),
    ).toBeVisible();
  });

  it("asks to log in on 401", async () => {
    mocks.post.mockResolvedValue({
      data: undefined,
      response: { status: 401 },
    });
    render(<ReportsView />);

    expect(
      await screen.findByText("Accedi per vedere i referti."),
    ).toBeVisible();
  });

  it("shows an error when the request fails", async () => {
    mocks.post.mockRejectedValue(new Error("down"));
    render(<ReportsView />);

    expect(
      await screen.findByText(
        "Il portale regionale non risponde. Riprova tra poco.",
      ),
    ).toBeVisible();
  });

  it("shows the empty state", async () => {
    mockQuery(
      body({
        rows: [],
        total: 0,
        updated: { at: null, label: "" },
      }),
    );
    render(<ReportsView />);

    expect(await screen.findByText("Nessun referto disponibile")).toBeVisible();
  });

  it("mounts the upload dialog only while open and re-queries after an upload", async () => {
    mockQuery(body());
    render(<ReportsView />);
    await screen.findByText("Clinica Prova");
    expect(screen.queryByRole("dialog", { name: "Carica referto" })).toBeNull();
    const before = queryRequests().length;

    await userEvent.click(
      screen.getByRole("button", { name: "Carica referto" }),
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Simula caricamento" }),
    );

    await waitFor(() => {
      expect(queryRequests().length).toBe(before + 1);
    });

    await userEvent.click(
      screen.getByRole("button", { name: "Chiudi finestra" }),
    );
    expect(screen.queryByRole("dialog", { name: "Carica referto" })).toBeNull();
  });

  it("opens the drawer for the clicked row with its ref", async () => {
    mocks.post.mockImplementation((path: string) => {
      if (path === "/api/reports/query") {
        return Promise.resolve({ data: body(), response: { status: 200 } });
      }
      if (path === "/api/reports/row") {
        return Promise.resolve({
          data: { row: lab, refreshing: false },
          response: { status: 200 },
        });
      }
      return Promise.resolve({
        data: { documents: [] },
        response: { status: 200 },
      });
    });
    render(<ReportsView />);
    await screen.findByText("Ospedale Esempio");
    expect(screen.queryByRole("dialog")).toBeNull();

    await userEvent.click(screen.getByText("Ospedale Esempio"));

    expect(await screen.findByRole("dialog")).toBeVisible();
    await waitFor(() => {
      const call = mocks.post.mock.calls.find(
        (item) => item[0] === "/api/reports/row",
      );
      expect(call?.[1]).toMatchObject({
        body: {
          ref: { kind: "lab", abstractId: "a1", detailId: "d1", repCode: "r1" },
        },
      });
    });
  });
});
