import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { JSX } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AppointmentBody, RowBody, SessionBody } from "../api/client";
import { HealthDataProvider } from "../data/HealthDataProvider";
import { SessionContext } from "../session/useSession";
import { callsTo } from "../test/calls";
import { VisitsView } from "./VisitsView";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
}));

vi.mock("../api/client", () => ({
  api: { GET: mocks.get },
  prescriptionPdfUrl: (ref: string) => `/api/documents/prescription?ref=${ref}`,
}));

const now = new Date("2026-11-10T09:00:00Z");

const toBook: RowBody = {
  nre: "0900A0000000001",
  cup: {
    nre: "0900A0000000001",
    kind: "visit",
    status: 3,
    items: ["Prestazione esempio"],
    prescribedOn: "2026-11-05T09:00:00Z",
  },
  fse: {
    id: "f1",
    nre: "0900A0000000001",
    description: "Visita esempio",
    kind: "visit",
    status: 3,
    date: "2026-11-04T09:00:00Z",
    documentRef: "ref-1",
  },
};

const drug: RowBody = {
  nre: "0900A0000000002",
  fse: {
    id: "f2",
    nre: "0900A0000000002",
    description: "Farmaco esempio",
    kind: "drug",
    status: 5,
    date: "2026-11-02T09:00:00Z",
    documentRef: "ref-2",
  },
};

const done: RowBody = {
  nre: "0900A0000000003",
  fse: {
    id: "f3",
    nre: "0900A0000000003",
    description: "Esame chiuso esempio",
    kind: "visit",
    status: 8,
    date: "2026-10-01T09:00:00Z",
    documentRef: "",
  },
};

const cupOnly: RowBody = {
  nre: "0900A0000000004",
  cup: {
    nre: "0900A0000000004",
    kind: "visit",
    status: 3,
    items: ["Ecografia esempio", "Prelievo esempio"],
    prescribedOn: "2026-11-06T09:00:00Z",
  },
};

const bookedRow: RowBody = {
  nre: "0900A0000000005",
  fse: {
    id: "f5",
    nre: "0900A0000000005",
    description: "Controllo esempio",
    kind: "visit",
    status: 30,
    date: "2026-10-15T09:00:00Z",
    documentRef: "",
  },
};

const rows = [toBook, drug, done, cupOnly, bookedRow];

function appointment(
  nre: string,
  at: string,
  status = "PRENOTATO",
): AppointmentBody {
  return {
    id: `a-${nre}`,
    nre,
    status,
    at,
    unit: {
      name: "Unita esempio",
      description: "",
      address: "Via Esempio 1",
      city: "Firenze",
      province: "FI",
    },
  };
}

function stubScroll(scroll: () => void): void {
  Object.defineProperty(Element.prototype, "scrollIntoView", {
    configurable: true,
    writable: true,
    value: scroll,
  });
}

function mockData(appointments: AppointmentBody[], notInCup: string[]): void {
  mocks.get.mockImplementation((path: string) => {
    if (path === "/api/appointments") {
      return Promise.resolve({
        data: {
          appointments,
          failed: notInCup.map((nre) => ({ nre, reason: "not_in_cup" })),
        },
      });
    }
    return Promise.resolve({ data: { prescriptions: rows } });
  });
}

const session: SessionBody = {
  gestioneReady: true,
  portals: [
    { portal: "cup", state: "active", login: { phase: "idle" } },
    { portal: "fse", state: "active", login: { phase: "idle" } },
  ],
};

function Harness(): JSX.Element {
  return (
    <SessionContext.Provider
      value={{
        session,
        error: undefined,
        refresh: vi.fn(),
        loginAll: vi.fn(),
        logout: vi.fn(),
      }}
    >
      <HealthDataProvider>
        <VisitsView now={now} />
      </HealthDataProvider>
    </SessionContext.Provider>
  );
}

function renderView() {
  return render(<Harness />);
}

function bodyRows(): HTMLElement[] {
  return screen.getAllByRole("row").slice(1);
}

function titles(): string[] {
  return bodyRows().map((row) => {
    const cell = row.querySelector('td[data-label="Prestazione"]');
    return cell?.textContent ?? "";
  });
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

describe("VisitsView", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    window.location.hash = "";
    mockData([], []);
  });

  it("shows the loading state", () => {
    mocks.get.mockReturnValue(new Promise(() => undefined));
    renderView();

    expect(screen.getByText("Carico visite ed esami…")).toBeInTheDocument();
    expect(
      screen.getByRole("region", { name: "Visite ed esami" }),
    ).toHaveAttribute("aria-busy", "true");
  });

  it("renders the columns with data-labels and excludes drugs", async () => {
    renderView();

    expect(await screen.findByText("Visita esempio")).toBeInTheDocument();
    expect(
      screen
        .getAllByRole("columnheader")
        .map((header) => header.textContent.replace(/[▲▼]/, "")),
    ).toEqual([
      "Prescritta",
      "Prestazione",
      "Stato",
      "Appuntamento",
      "Struttura",
      "NRE",
      "Azioni",
    ]);
    const row = rowOf("Visita esempio");
    expect(
      Array.from(row.querySelectorAll("td")).map((td) =>
        td.getAttribute("data-label"),
      ),
    ).toEqual([
      "Prescritta",
      "Prestazione",
      "Stato",
      "Appuntamento",
      "Struttura",
      "NRE",
      "Azioni",
    ]);
    expect(screen.queryByText("Farmaco esempio")).not.toBeInTheDocument();
    expect(within(row).getByText("Da prenotare")).toBeInTheDocument();
    expect(within(row).getByText("0900A0000000001")).toBeInTheDocument();
    expect(
      within(row).getByRole("button", { name: "Copia NRE 0900A0000000001" }),
    ).toHaveTextContent("0900A0000000001");
    expect(
      within(row).getByRole("link", { name: "Apri ricetta (PDF)" }),
    ).toHaveAttribute("href", "/api/documents/prescription?ref=ref-1");
    const book = within(row).getByRole("link", { name: "Prenota sul CUP" });
    expect(book).toHaveAttribute("href", "https://prenota.sanita.toscana.it/");
    expect(book).toHaveAttribute("target", "_blank");
  });

  it("shows a booked row with appointment, map and calendar links", async () => {
    mockData([appointment("0900A0000000001", "2026-11-12T09:30:00Z")], []);
    renderView();

    const row = await waitFor(() => {
      const found = bodyRows().find(
        (r) => within(r).queryByText("Prenotata") !== null,
      );
      if (found === undefined) {
        throw new Error("not booked yet");
      }
      return found;
    });
    expect(within(row).getByText(/10:30/)).toBeVisible();
    expect(within(row).getByText("tra 2 giorni")).toBeVisible();
    expect(within(row).getByText("Unita esempio")).toBeVisible();
    expect(within(row).getByText("Via Esempio 1, Firenze, FI")).toBeVisible();
    const map = within(row).getByRole("link", { name: "Apri su mappa" });
    expect(map.getAttribute("href")).toContain("openstreetmap.org");
    expect(map).toHaveAttribute("target", "_blank");
    expect(map).toHaveAttribute("rel", "noopener noreferrer");
    const calendar = within(row).getByRole("link", {
      name: "Aggiungi a Google Calendar",
    });
    expect(calendar.getAttribute("href")).toContain("calendar.google.com");
    expect(calendar).toHaveAttribute("rel", "noopener noreferrer");
    expect(
      within(row).queryByRole("link", { name: "Prenota sul CUP" }),
    ).not.toBeInTheDocument();
  });

  it("shows ricerca while appointments are pending", async () => {
    mocks.get.mockImplementation((path: string) => {
      if (path === "/api/appointments") {
        return new Promise(() => undefined);
      }
      return Promise.resolve({ data: { prescriptions: rows } });
    });
    renderView();

    await screen.findByText("Visita esempio");
    const cell = within(rowOf("Visita esempio")).getByText("ricerca…");
    expect(cell.closest("td")).toHaveAttribute("aria-busy", "true");
  });

  it("shows a dash when appointments failed", async () => {
    mocks.get.mockImplementation((path: string) => {
      if (path === "/api/appointments") {
        return Promise.resolve({
          error: { detail: "bad gateway" },
          response: { status: 502 },
        });
      }
      return Promise.resolve({ data: { prescriptions: rows } });
    });
    renderView();

    await screen.findByText("Visita esempio");
    expect(
      await within(rowOf("Visita esempio")).findByText("—", {
        selector: "td",
      }),
    ).toBeInTheDocument();
  });

  it("shows the booked elsewhere badge and hides Prenota sul CUP", async () => {
    mockData([], ["0900A0000000004"]);
    renderView();

    const badge = await screen.findByText("Prenotata fuori dal CUP regionale");
    const row = badge.closest("tr");
    expect(row).not.toBeNull();
    expect(
      within(row as HTMLElement).queryByRole("link", {
        name: "Prenota sul CUP",
      }),
    ).not.toBeInTheDocument();
  });

  it("adds a row for an appointment without a prescription", async () => {
    mockData([appointment("0900A0000000099", "2026-11-15T09:00:00Z")], []);
    renderView();

    expect(await screen.findByText("0900A0000000099")).toBeInTheDocument();
  });

  it("shows the open view by default with counts", async () => {
    renderView();

    await screen.findByText("Visita esempio");
    expect(screen.getByRole("radio", { name: "Aperte (3)" })).toBeChecked();
    expect(screen.getByRole("radio", { name: "Prossimi (0)" })).toBeVisible();
    expect(
      screen.getByRole("radio", { name: "Da prenotare (2)" }),
    ).toBeVisible();
    expect(screen.getByRole("radio", { name: "Tutte (4)" })).toBeVisible();
    expect(screen.getByText("3 visite/esami")).toBeInTheDocument();
  });

  it("sorts the default view by prescription date desc without appointments", async () => {
    renderView();

    await screen.findByText("Visita esempio");
    expect(titles()).toEqual([
      "Ecografia esempio, Prelievo esempio",
      "Visita esempio",
      "Controllo esempio",
    ]);
  });

  it("filters Prossimi", async () => {
    const user = userEvent.setup();
    mockData([appointment("0900A0000000005", "2026-11-20T09:00:00Z")], []);
    renderView();
    await screen.findByText("Visita esempio");

    await user.click(
      await screen.findByRole("radio", { name: "Prossimi (1)" }),
    );

    expect(titles()).toEqual(["Controllo esempio"]);
  });

  it("filters Da prenotare", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");

    await user.click(screen.getByRole("radio", { name: /Da prenotare/ }));

    expect(titles()).toEqual([
      "Ecografia esempio, Prelievo esempio",
      "Visita esempio",
    ]);
  });

  it("filters Tutte", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");

    await user.click(screen.getByRole("radio", { name: /Tutte/ }));

    expect(titles()).toHaveLength(4);
    expect(screen.getByText("Esame chiuso esempio")).toBeInTheDocument();
  });

  it("narrows the list with the search box and recounts", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");

    await user.type(
      screen.getByRole("searchbox", { name: "Cerca" }),
      "ECOGRAFIA",
    );

    expect(titles()).toEqual(["Ecografia esempio, Prelievo esempio"]);
    expect(screen.getByText("1 visite/esami")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Tutte (1)" })).toBeVisible();
  });

  it("finds a row by unit city", async () => {
    const user = userEvent.setup();
    mockData([appointment("0900A0000000005", "2026-11-20T09:00:00Z")], []);
    renderView();
    await screen.findByText("Visita esempio");
    await screen.findByRole("radio", { name: "Prossimi (1)" });

    await user.type(screen.getByRole("searchbox"), "firenze");

    expect(titles()).toEqual(["Controllo esempio"]);
  });

  it("sorts by a header click and sets aria-sort", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");

    await user.click(screen.getByRole("button", { name: "Prestazione" }));

    expect(
      screen.getByRole("columnheader", { name: /Prestazione/ }),
    ).toHaveAttribute("aria-sort", "ascending");
    expect(titles()).toEqual([
      "Controllo esempio",
      "Ecografia esempio, Prelievo esempio",
      "Visita esempio",
    ]);

    await user.click(screen.getByRole("button", { name: /Prestazione/ }));

    expect(
      screen.getByRole("columnheader", { name: /Prestazione/ }),
    ).toHaveAttribute("aria-sort", "descending");
    expect(titles()[0]).toBe("Visita esempio");
  });

  it("sorts upcoming appointments first by default", async () => {
    mockData([appointment("0900A0000000005", "2026-11-20T09:00:00Z")], []);
    renderView();
    await screen.findByText("Visita esempio");

    await waitFor(() => {
      expect(titles()[0]).toBe("Controllo esempio");
    });
    expect(
      screen.getByRole("columnheader", { name: /Appuntamento/ }),
    ).toHaveAttribute("aria-sort", "ascending");
  });

  it("shows a notice for appointment errors and keeps the table", async () => {
    mocks.get.mockImplementation((path: string) => {
      if (path === "/api/appointments") {
        return Promise.resolve({
          error: { detail: "gestione" },
          response: { status: 409 },
        });
      }
      return Promise.resolve({ data: { prescriptions: rows } });
    });
    renderView();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Per vedere date e orari serve l'accesso a Gestione Prenotazioni: premi Accedi.",
    );
    expect(screen.getByText("Visita esempio")).toBeInTheDocument();
  });

  it("shows the login notice for appointments on 401", async () => {
    mocks.get.mockImplementation((path: string) => {
      if (path === "/api/appointments") {
        return Promise.resolve({
          error: { detail: "no session" },
          response: { status: 401 },
        });
      }
      return Promise.resolve({ data: { prescriptions: rows } });
    });
    renderView();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Accedi per vedere gli appuntamenti.",
    );
  });

  it("shows the unavailable notice for appointments", async () => {
    mocks.get.mockImplementation((path: string) => {
      if (path === "/api/appointments") {
        return Promise.reject(new Error("network"));
      }
      return Promise.resolve({ data: { prescriptions: rows } });
    });
    renderView();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Appuntamenti non disponibili: il portale regionale non risponde.",
    );
    expect(screen.getByText("Visita esempio")).toBeInTheDocument();
  });

  it("shows the login message on prescriptions 401", async () => {
    mocks.get.mockResolvedValue({
      error: { detail: "no session" },
      response: { status: 401 },
    });
    renderView();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Accedi per vedere visite ed esami.",
    );
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows the portal message on prescriptions 502", async () => {
    mocks.get.mockResolvedValue({
      error: { detail: "bad gateway" },
      response: { status: 502 },
    });
    renderView();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Il portale regionale non risponde. Riprova tra poco.",
    );
  });

  it("refetches both resources on Aggiorna", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");
    expect(callsTo(mocks.get, "/api/prescriptions")).toBe(1);
    expect(callsTo(mocks.get, "/api/appointments")).toBe(1);

    await user.click(screen.getByRole("button", { name: "Aggiorna" }));

    await waitFor(() => {
      expect(callsTo(mocks.get, "/api/prescriptions")).toBe(2);
      expect(callsTo(mocks.get, "/api/appointments")).toBe(2);
    });
  });

  it("shows the empty state for a view with no rows", async () => {
    mocks.get.mockResolvedValue({
      data: { prescriptions: [done], appointments: [], failed: [] },
    });
    renderView();

    expect(
      await screen.findByText("Nessuna visita o esame in questa vista."),
    ).toBeInTheDocument();
  });

  it("focuses the row of a deep-linked NRE", async () => {
    const scroll = vi.fn();
    stubScroll(scroll);
    window.location.hash = "#/visite/0900A0000000004";
    try {
      renderView();
      const row = (
        await screen.findByText("Ecografia esempio, Prelievo esempio")
      ).closest("tr");

      expect(row).toHaveClass("is-focused");
      expect(row).toHaveFocus();
      expect(scroll).toHaveBeenCalled();
    } finally {
      Reflect.deleteProperty(Element.prototype, "scrollIntoView");
    }
  });

  it("switches to Tutte when the focused row is hidden", async () => {
    stubScroll(vi.fn());
    window.location.hash = "#/visite/0900A0000000003";
    try {
      renderView();
      const row = (await screen.findByText("Esame chiuso esempio")).closest(
        "tr",
      );

      expect(row).toHaveClass("is-focused");
      expect(screen.getByRole("radio", { name: /Tutte/ })).toBeChecked();
    } finally {
      Reflect.deleteProperty(Element.prototype, "scrollIntoView");
    }
  });
});
