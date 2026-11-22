import {
  act,
  render as rtlRender,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { JSX } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type {
  LinkedReportBody,
  ReportRefBody,
  VisitRowBody,
  VisitsQueryBody,
  VisitsQueryRequest,
} from "../api/client";
import { clearRequestedVisit, requestOpenVisit } from "../lib/openVisit";
import { render } from "../test/render";
import { TestProviders } from "../test/TestProviders";
import {
  appointment,
  bookedRow,
  nre1,
  nre2,
  nre3,
  queryBody,
  rowResponse,
  unit,
  visitRow,
} from "../test/visits";
import { WithSession } from "../test/WithSession";
import { VisitsView } from "./VisitsView";

const mocks = vi.hoisted(() => ({
  post: vi.fn(),
  startRefresh: vi.fn(),
}));

vi.mock("../api/client", () => ({
  api: { POST: mocks.post, PUT: vi.fn(), DELETE: vi.fn() },
  prescriptionPdfUrl: (ref: string) => `/api/documents/prescription?ref=${ref}`,
  bookingPdfUrl: (id: string) => `/api/documents/booking?id=${id}`,
}));

vi.mock("../data/refresh", () => ({ startRefresh: mocks.startRefresh }));

vi.mock("../lib/pdf", () => ({
  loadPdf: () => ({
    promise: new Promise<never>(() => undefined),
    destroy: () => Promise.resolve(),
  }),
}));

vi.mock("./reports/ReportDetailModal", () => ({
  ReportDetailModal: ({
    reportRef,
  }: {
    reportRef: ReportRefBody;
  }): JSX.Element => <p>{`dettaglio ${reportRef.abstractId}`}</p>,
}));

const toBook = visitRow();
const booked = bookedRow();
const defaultRows = [toBook, booked];

let queries: { status: number; data?: VisitsQueryBody }[] = [];
let rows: Record<string, VisitRowBody> = {};

function respond(...bodies: VisitsQueryBody[]): void {
  queries = bodies.map((data) => ({ status: 200, data }));
}

function queryCalls(): VisitsQueryRequest[] {
  return mocks.post.mock.calls
    .filter((call) => call[0] === "/api/visits/query")
    .map((call) => (call[1] as { body: VisitsQueryRequest }).body);
}

function lastQuery(): VisitsQueryRequest | undefined {
  return queryCalls().at(-1);
}

function viewTree(): JSX.Element {
  return (
    <WithSession>
      <VisitsView />
    </WithSession>
  );
}

function renderView(): void {
  render(viewTree());
}

function bodyRows(): HTMLElement[] {
  return screen.getAllByRole("row").slice(1);
}

function rowOf(text: string): HTMLElement {
  const found = bodyRows().find(
    (row) => within(row).queryByText(text) !== null,
  );
  if (found === undefined) {
    throw new Error(`no row with ${text}`);
  }
  return found;
}

function sortHeader(name: string): HTMLElement {
  return screen.getByRole("button", { name: new RegExp(`^${name}`) });
}

async function openStatusFilter(
  user: ReturnType<typeof userEvent.setup>,
): Promise<void> {
  await user.click(within(sortHeader("Stato")).getByRole("button"));
}

function report(abstractId: string): LinkedReportBody {
  return {
    ref: { kind: "lab", abstractId, detailId: "D", repCode: "R" },
    label: `Esame ${abstractId}`,
    date: "2026-11-05T09:00:00Z",
    provider: "Laboratorio Esempio",
    origin: "remote",
    available: true,
  };
}

describe("VisitsView", () => {
  beforeEach(() => {
    mocks.post.mockReset();
    mocks.startRefresh.mockReset();
    mocks.startRefresh.mockResolvedValue(true);
    window.location.hash = "";
    rows = {};
    let served = 0;
    queries = [];
    respond(queryBody(defaultRows));
    mocks.post.mockImplementation(
      (path: string, init: { body: { nre?: string } }) => {
        if (path === "/api/visits/query") {
          const index = Math.min(served, queries.length - 1);
          served += 1;
          const answer = queries[index];
          return Promise.resolve({
            data: answer?.data,
            response: { status: answer?.status ?? 500 },
          });
        }
        if (path === "/api/visits/row") {
          const row = rows[init.body.nre ?? ""];
          return Promise.resolve({
            data: row === undefined ? undefined : rowResponse(row),
            response: { status: row === undefined ? 404 : 200 },
          });
        }
        return Promise.resolve({
          data: { options: [] },
          response: { status: 200 },
        });
      },
    );
  });

  afterEach(() => {
    clearRequestedVisit();
    window.location.hash = "";
    vi.useRealTimers();
  });

  it("shows the loading state", () => {
    renderView();

    expect(screen.getByText("Carico visite ed esami…")).toBeInTheDocument();
  });

  it("first posts sort, dir, page and pageSize without search or exclusions", async () => {
    renderView();
    await screen.findByText("Visita esempio");

    expect(queryCalls()).toEqual([
      { dir: "desc", page: 1, pageSize: 50, sort: "prescribed" },
    ]);
  });

  it("renders the rows exactly as the backend returns them", async () => {
    renderView();
    await screen.findByText("Visita esempio");

    expect(bodyRows()).toHaveLength(2);
    expect(screen.getByText("2 visite/esami")).toBeInTheDocument();
    expect(screen.getByText("Aggiornato alle 09:00")).toBeInTheDocument();
    const bookedRowEl = rowOf("Controllo esempio");
    expect(within(bookedRowEl).getByText("Prenotata")).toBeInTheDocument();
    expect(within(bookedRowEl).getByText("tra 3 giorni")).toBeInTheDocument();
    expect(
      within(bookedRowEl).getByRole("link", {
        name: "Apri in Google Maps: Ospedale Esempio",
      }),
    ).toHaveAttribute("href", "https://maps.example.com/search?q=finta");
    expect(
      within(bookedRowEl).getByRole("button", {
        name: "Apri promemoria prenotazione (PDF)",
      }),
    ).toBeInTheDocument();
    expect(
      within(rowOf("Visita esempio")).getByRole("link", {
        name: "Prenota sul CUP",
      }),
    ).toBeInTheDocument();
    expect(
      within(rowOf("Visita esempio")).getByRole("button", {
        name: `Copia NRE ${nre1}`,
      }),
    ).toBeInTheDocument();
  });

  it("hides the freshness text when the label is empty", async () => {
    respond(queryBody(defaultRows, { updated: { at: null, label: "" } }));
    renderView();
    await screen.findByText("Visita esempio");

    expect(screen.queryByText(/Aggiornato/)).not.toBeInTheDocument();
  });

  it("shows no place link without an address", async () => {
    respond(
      queryBody([
        bookedRow({
          appointment: appointment({ unit: unit({ address: "" }) }),
        }),
      ]),
    );
    renderView();
    await screen.findByText("Controllo esempio");

    expect(
      screen.queryByRole("link", { name: /Apri in Google Maps/ }),
    ).not.toBeInTheDocument();
  });

  it("shows the calendar action only for an upcoming appointment", async () => {
    respond(
      queryBody([
        bookedRow({ appointment: appointment({ upcoming: false }) }),
        bookedRow({ nre: nre3, title: "Altro esempio" }),
      ]),
    );
    renderView();
    await screen.findByText("Controllo esempio");

    expect(
      within(rowOf("Controllo esempio")).queryByRole("button", {
        name: "Aggiungi a Google Calendar",
      }),
    ).not.toBeInTheDocument();
    expect(
      within(rowOf("Altro esempio")).getByRole("button", {
        name: "Aggiungi a Google Calendar",
      }),
    ).toBeInTheDocument();
  });

  it("shows a manual status with its appointment", async () => {
    respond(
      queryBody([
        bookedRow({
          status: { ...booked.status, manual: true, label: "Erogata" },
        }),
      ]),
    );
    renderView();
    await screen.findByText("Controllo esempio");

    const row = rowOf("Controllo esempio");
    expect(
      within(row).getByRole("img", { name: "Stato impostato a mano" }),
    ).toBeInTheDocument();
    expect(within(row).getByText("tra 3 giorni")).toBeInTheDocument();
  });

  it("shows the status info only when there are details", async () => {
    const user = userEvent.setup();
    respond(
      queryBody([
        visitRow({ status: { ...toBook.status, details: ["Scade il 2 gen"] } }),
        bookedRow(),
      ]),
    );
    renderView();
    await screen.findByText("Visita esempio");

    expect(
      within(rowOf("Controllo esempio")).queryByRole("button", {
        name: "Dettagli stato",
      }),
    ).not.toBeInTheDocument();
    await user.click(
      within(rowOf("Visita esempio")).getByRole("button", {
        name: "Dettagli stato",
      }),
    );
    expect(await screen.findAllByText("Scade il 2 gen")).not.toHaveLength(0);
    expect(
      screen.queryByRole("dialog", { name: "Visita esempio" }),
    ).not.toBeInTheDocument();
  });

  it("shows the row spinner while a row refreshes", async () => {
    respond(queryBody([visitRow({ refreshing: true }), bookedRow()]));
    renderView();
    await screen.findByText("Visita esempio");

    expect(
      within(rowOf("Visita esempio")).getByLabelText("Aggiorno"),
    ).toBeInTheDocument();
    expect(
      within(rowOf("Controllo esempio")).queryByLabelText("Aggiorno"),
    ).not.toBeInTheDocument();
  });

  it("shows ricerca while the appointment lookup is checking", async () => {
    respond(queryBody([visitRow({ appointmentLookup: "checking" })]));
    renderView();
    await screen.findByText("Visita esempio");

    expect(
      within(rowOf("Visita esempio")).getByText("ricerca…"),
    ).toBeInTheDocument();
  });

  it("shows a dash when the appointment lookup failed", async () => {
    respond(queryBody([visitRow({ appointmentLookup: "failed" })]));
    renderView();
    await screen.findByText("Visita esempio");

    expect(within(rowOf("Visita esempio")).getByText("—")).toBeInTheDocument();
  });

  it("lists the statuses from the backend with their counts", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");

    await openStatusFilter(user);

    expect(
      await screen.findByRole("checkbox", { name: "Prenotata (1)" }),
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Erogata (1)" }),
    ).not.toBeChecked();
  });

  it("posts the excluded keys chosen in the filter and resets the page", async () => {
    const user = userEvent.setup();
    respond(queryBody(defaultRows, { total: 120, page: 1 }));
    renderView();
    await screen.findByText("Visita esempio");
    await user.click(await screen.findByRole("button", { name: "2" }));
    await waitFor(() => {
      expect(lastQuery()?.page).toBe(2);
    });

    await openStatusFilter(user);
    await user.click(
      await screen.findByRole("checkbox", { name: "Prenotata (1)" }),
    );

    await waitFor(() => {
      expect(lastQuery()).toEqual({
        dir: "desc",
        page: 1,
        pageSize: 50,
        sort: "prescribed",
        excludeStatuses: ["booked", "done"],
      });
    });
  });

  it("posts an empty exclusion list with Tutti", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");
    await openStatusFilter(user);

    await user.click(await screen.findByRole("button", { name: "Tutti" }));

    await waitFor(() => {
      expect(lastQuery()?.excludeStatuses).toEqual([]);
    });
  });

  it("posts the search text after the debounce and resets the page", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");

    await user.type(screen.getByRole("searchbox", { name: "Cerca" }), "eco");

    await waitFor(() => {
      expect(lastQuery()).toEqual({
        dir: "desc",
        page: 1,
        pageSize: 50,
        sort: "prescribed",
        search: "eco",
      });
    });
    expect(
      queryCalls().filter((call) => call.search !== undefined),
    ).toHaveLength(1);
  });

  it("posts the sort on a header click and toggles the direction", async () => {
    const user = userEvent.setup();
    respond(queryBody(defaultRows, { total: 120 }));
    renderView();
    await screen.findByText("Visita esempio");
    await user.click(await screen.findByRole("button", { name: "2" }));
    await waitFor(() => {
      expect(lastQuery()?.page).toBe(2);
    });

    await user.click(sortHeader("Prestazione"));

    await waitFor(() => {
      expect(lastQuery()).toMatchObject({ sort: "title", dir: "asc", page: 1 });
    });
    await user.click(sortHeader("Prestazione"));
    await waitFor(() => {
      expect(lastQuery()).toMatchObject({ sort: "title", dir: "desc" });
    });
  });

  it("posts the page the user selects", async () => {
    const user = userEvent.setup();
    respond(queryBody(defaultRows, { total: 120 }));
    renderView();
    await screen.findByText("Visita esempio");

    await user.click(await screen.findByRole("button", { name: "3" }));

    await waitFor(() => {
      expect(lastQuery()?.page).toBe(3);
    });
  });

  it("posts the page size and resets the page", async () => {
    const user = userEvent.setup();
    respond(queryBody(defaultRows, { total: 300 }));
    renderView();
    await screen.findByText("Visita esempio");
    await user.click(await screen.findByRole("button", { name: "2" }));
    await waitFor(() => {
      expect(lastQuery()?.page).toBe(2);
    });

    await user.click(screen.getByRole("button", { name: "50" }));
    await user.click(await screen.findByRole("menuitem", { name: "25" }));

    await waitFor(() => {
      expect(lastQuery()).toMatchObject({ pageSize: 25, page: 1 });
    });
  });

  it("Aggiorna tutto starts a full refresh and reloads the query", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");
    const before = queryCalls().length;

    await user.click(screen.getByRole("button", { name: "Aggiorna tutto" }));

    expect(mocks.startRefresh).toHaveBeenCalledWith({ all: true });
    await waitFor(() => {
      expect(queryCalls().length).toBeGreaterThan(before);
    });
  });

  it("refreshes the selected rows by NRE and clears the selection", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");
    expect(
      screen.queryByRole("button", { name: /^Aggiorna selezionate/ }),
    ).not.toBeInTheDocument();

    await user.click(within(rowOf("Visita esempio")).getByRole("checkbox"));
    await user.click(
      screen.getByRole("button", { name: "Aggiorna selezionate (1)" }),
    );

    expect(mocks.startRefresh).toHaveBeenCalledWith({ nres: [nre1] });
    await waitFor(() => {
      expect(
        screen.queryByRole("button", { name: /^Aggiorna selezionate/ }),
      ).not.toBeInTheDocument();
    });
  });

  it("notifies a failed refresh start", async () => {
    const user = userEvent.setup();
    mocks.startRefresh.mockResolvedValue(false);
    renderView();
    await screen.findByText("Visita esempio");

    await user.click(screen.getByRole("button", { name: "Aggiorna tutto" }));

    expect(
      await screen.findByText("Impossibile avviare l'aggiornamento"),
    ).toBeInTheDocument();
  });

  it("polls every 3 seconds while refreshing and stops when done", async () => {
    vi.useFakeTimers();
    respond(
      queryBody(defaultRows, { refreshing: true }),
      queryBody(defaultRows, { refreshing: false }),
    );
    rtlRender(viewTree(), { wrapper: TestProviders });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(queryCalls()).toHaveLength(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000);
    });
    expect(queryCalls()).toHaveLength(2);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(9000);
    });
    expect(queryCalls()).toHaveLength(2);
  });

  it("shows the empty text", async () => {
    respond(queryBody([], { statuses: [] }));
    renderView();

    expect(
      await screen.findByText("Nessuna visita o esame in questa vista."),
    ).toBeInTheDocument();
    expect(screen.getByText("0 visite/esami")).toBeInTheDocument();
  });

  it("shows the login notice for the lists on 401", async () => {
    queries = [{ status: 401 }];
    renderView();

    expect(
      await screen.findByText("Accedi per vedere visite ed esami."),
    ).toBeInTheDocument();
  });

  it("shows the portal notice when the query is unavailable", async () => {
    queries = [{ status: 502 }];
    renderView();

    expect(
      await screen.findByText(
        "Il portale regionale non risponde. Riprova tra poco.",
      ),
    ).toBeInTheDocument();
  });

  it("shows the Gestione notice on a 409 from the query", async () => {
    queries = [{ status: 409 }];
    renderView();

    expect(
      await screen.findByText(/serve l'accesso a Gestione Prenotazioni/),
    ).toBeInTheDocument();
  });

  it("shows the lists issues and keeps the table", async () => {
    respond(
      queryBody(defaultRows, {
        issues: { lists: "login_required", appointments: "none" },
      }),
    );
    renderView();

    expect(
      await screen.findByText("Accedi per vedere visite ed esami."),
    ).toBeInTheDocument();
    expect(screen.getByText("Visita esempio")).toBeInTheDocument();
  });

  it("shows the lists unavailable issue", async () => {
    respond(
      queryBody(defaultRows, {
        issues: { lists: "unavailable", appointments: "none" },
      }),
    );
    renderView();

    expect(
      await screen.findByText(
        "Il portale regionale non risponde. Riprova tra poco.",
      ),
    ).toBeInTheDocument();
  });

  it("shows the appointments login issue", async () => {
    respond(
      queryBody(defaultRows, {
        issues: { lists: "none", appointments: "login_required" },
      }),
    );
    renderView();

    expect(
      await screen.findByText("Accedi per vedere gli appuntamenti."),
    ).toBeInTheDocument();
  });

  it("shows the Gestione issue for the appointments", async () => {
    respond(
      queryBody(defaultRows, {
        issues: { lists: "none", appointments: "gestione_required" },
      }),
    );
    renderView();

    expect(
      await screen.findByText(/serve l'accesso a Gestione Prenotazioni/),
    ).toBeInTheDocument();
    expect(screen.getByText("Visita esempio")).toBeInTheDocument();
  });

  it("shows the appointments unavailable issue", async () => {
    respond(
      queryBody(defaultRows, {
        issues: { lists: "none", appointments: "unavailable" },
      }),
    );
    renderView();

    expect(
      await screen.findByText(
        "Appuntamenti non disponibili: il portale regionale non risponde.",
      ),
    ).toBeInTheDocument();
  });

  it("opens the drawer on row click, fetching the row itself", async () => {
    const user = userEvent.setup();
    rows = { [nre1]: toBook };
    renderView();
    await screen.findByText("Visita esempio");

    await user.click(rowOf("Visita esempio"));

    expect(
      await screen.findByRole("dialog", { name: "Visita esempio" }),
    ).toBeInTheDocument();
    expect(mocks.post).toHaveBeenCalledWith(
      "/api/visits/row",
      expect.objectContaining({ body: { nre: nre1 } }),
    );
  });

  it("opens the drawer with Enter on a focused row", async () => {
    const user = userEvent.setup();
    rows = { [nre1]: toBook };
    renderView();
    await screen.findByText("Visita esempio");

    rowOf("Visita esempio").focus();
    await user.keyboard("{Enter}");

    expect(
      await screen.findByRole("dialog", { name: "Visita esempio" }),
    ).toBeInTheDocument();
  });

  it("does not open the drawer from an Azioni icon", async () => {
    const user = userEvent.setup();
    rows = { [nre2]: booked };
    renderView();
    await screen.findByText("Controllo esempio");

    await user.click(
      within(rowOf("Controllo esempio")).getByRole("button", {
        name: "Apri promemoria prenotazione (PDF)",
      }),
    );

    expect(
      screen.queryByRole("dialog", { name: "Controllo esempio" }),
    ).not.toBeInTheDocument();
  });

  it("has no linked reports icon when nothing is linked", async () => {
    renderView();
    await screen.findByText("Visita esempio");

    expect(
      screen.queryByRole("button", { name: /Referti collegati/ }),
    ).not.toBeInTheDocument();
  });

  it("opens the report directly when exactly one is linked", async () => {
    const user = userEvent.setup();
    respond(queryBody([visitRow({ reports: [report("A1")] })]));
    renderView();
    await screen.findByText("Visita esempio");

    await user.click(
      screen.getByRole("button", { name: "Referti collegati (1)" }),
    );

    expect(screen.getByText("dettaglio A1")).toBeInTheDocument();
  });

  it("opens the visit drawer when several reports are linked", async () => {
    const user = userEvent.setup();
    const row = visitRow({ reports: [report("A1"), report("B2")] });
    rows = { [nre1]: row };
    respond(queryBody([row]));
    renderView();
    await screen.findByText("Visita esempio");

    await user.click(
      screen.getByRole("button", { name: "Referti collegati (2)" }),
    );

    expect(
      await screen.findByRole("dialog", { name: "Visita esempio" }),
    ).toBeInTheDocument();
  });

  it("opens the drawer of a requested visit and sends its NRE once as focus", async () => {
    rows = { [nre3]: visitRow({ nre: nre3, title: "Esame richiesto" }) };
    respond(
      queryBody(defaultRows),
      queryBody([visitRow({ nre: nre3, title: "Esame richiesto" })], {
        page: 2,
        total: 120,
        search: "",
        excludedStatuses: [],
        focus: { nre: nre3, found: true },
      }),
      queryBody([visitRow({ nre: nre3, title: "Esame richiesto" })], {
        page: 2,
        total: 120,
        excludedStatuses: [],
      }),
    );
    act(() => {
      requestOpenVisit(nre3);
    });
    renderView();

    expect(
      await screen.findByRole("dialog", { name: "Esame richiesto" }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(lastQuery()).toEqual({
        dir: "desc",
        page: 2,
        pageSize: 50,
        sort: "prescribed",
        excludeStatuses: [],
      });
    });
    expect(queryCalls().filter((call) => call.focus === nre3)).toHaveLength(1);
  });

  it("ignores a requested visit the backend cannot find", async () => {
    respond(
      queryBody(defaultRows),
      queryBody(defaultRows, { focus: { nre: nre3, found: false } }),
    );
    act(() => {
      requestOpenVisit(nre3);
    });
    renderView();

    await waitFor(() => {
      expect(queryCalls().filter((call) => call.focus === nre3)).toHaveLength(
        1,
      );
    });
    await waitFor(() => {
      expect(lastQuery()?.focus).toBeUndefined();
    });
  });

  it("sends a deep-linked NRE as focus and marks its row", async () => {
    window.location.hash = `#/visite/${nre1}`;
    respond(
      queryBody(defaultRows),
      queryBody(defaultRows, { focus: { nre: nre1, found: true } }),
    );
    renderView();

    await waitFor(() => {
      expect(queryCalls().some((call) => call.focus === nre1)).toBe(true);
    });
    await waitFor(() => {
      expect(rowOf("Visita esempio")).toHaveAttribute("data-focused", "true");
    });
  });
});
