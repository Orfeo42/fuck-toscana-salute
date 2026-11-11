import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { JSX } from "react";
import {
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type MockInstance,
} from "vitest";

import type { AppointmentBody, RowBody, SessionBody } from "../api/client";
import { HealthDataProvider } from "../data/HealthDataProvider";
import { GoogleContext } from "../google/useGoogle";
import { formatDate } from "../lib/format";
import { SessionContext } from "../session/useSession";
import { callsTo } from "../test/calls";
import { VisitsView } from "./VisitsView";
import { render } from "../test/render";

vi.mock("../lib/pdf", () => ({
  loadPdf: () => ({
    promise: new Promise<never>(() => undefined),
    destroy: () => Promise.resolve(),
  }),
}));

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
}));

vi.mock("../api/client", () => ({
  api: { GET: mocks.get },
  prescriptionPdfUrl: (ref: string) => `/api/documents/prescription?ref=${ref}`,
  bookingPdfUrl: (id: string) => `/api/documents/booking?id=${id}`,
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
      postalCode: "50100",
      city: "Firenze",
      province: "FI",
    },
  };
}

function fseVisit(
  nre: string,
  description: string,
  expiresOn: string,
): RowBody {
  return {
    nre,
    fse: {
      id: `f-${nre}`,
      nre,
      description,
      kind: "visit",
      status: 3,
      date: "2026-11-01T09:00:00Z",
      expiresOn,
      documentRef: "",
    },
  };
}

function mockRows(
  prescriptions: RowBody[],
  appointments: AppointmentBody[],
): void {
  mocks.get.mockImplementation((path: string) => {
    if (path === "/api/appointments") {
      return Promise.resolve({ data: { appointments, failed: [] } });
    }
    return Promise.resolve({ data: { prescriptions } });
  });
}

function stubScroll(scroll: () => void): MockInstance {
  return vi
    .spyOn(HTMLElement.prototype, "scrollIntoView")
    .mockImplementation(scroll);
}

function mockData(appointments: AppointmentBody[], unbooked: string[]): void {
  mocks.get.mockImplementation((path: string) => {
    if (path === "/api/appointments") {
      return Promise.resolve({
        data: {
          appointments,
          failed: [],
          unbooked,
        },
      });
    }
    return Promise.resolve({ data: { prescriptions: rows } });
  });
}

const session: SessionBody = {
  gestioneReady: true,
  loginMode: "window",
  viewer: false,
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
      <GoogleContext.Provider
        value={{
          status: { configured: true, connected: true },
          loading: false,
          error: undefined,
          refresh: vi.fn(),
          connect: vi.fn(),
          disconnect: vi.fn(),
        }}
      >
        <HealthDataProvider>
          <VisitsView now={now} />
        </HealthDataProvider>
      </GoogleContext.Provider>
    </SessionContext.Provider>
  );
}

function renderView() {
  return render(<Harness />);
}

function bodyRows(): HTMLElement[] {
  return screen.getAllByRole("row").slice(1);
}

type User = ReturnType<typeof userEvent.setup>;

async function openStatusFilter(user: User): Promise<void> {
  const header = sortHeader("Stato");
  await user.click(within(header).getByRole("button"));
}

async function pickStatuses(user: User, name: string): Promise<void> {
  await openStatusFilter(user);
  await user.click(await screen.findByRole("checkbox", { name }));
}

function sortHeader(name: string): HTMLElement {
  return screen.getByRole("button", { name: new RegExp(`^${name}`) });
}

function sortLabel(name: string): string | null {
  return within(sortHeader(name)).getByRole("img").getAttribute("aria-label");
}

function titles(): string[] {
  return bodyRows().map((row) => {
    const cell = row.querySelectorAll("td")[1];
    return cell?.textContent ?? "";
  });
}

async function openStatusInfo(user: User, row: HTMLElement): Promise<void> {
  await user.click(within(row).getByRole("button", { name: "Dettagli stato" }));
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

  it("renders the columns and excludes drugs", async () => {
    renderView();

    expect(await screen.findByText("Visita esempio")).toBeInTheDocument();
    expect(
      [...screen.getByRole("table").querySelectorAll("th")].map(
        (header) => header.textContent,
      ),
    ).toEqual([
      "Prescritta",
      "Prestazione",
      "Stato",
      "Appuntamento",
      "Luogo",
      "NRE",
      "Azioni",
    ]);
    const row = rowOf("Visita esempio");
    expect(screen.queryByText("Farmaco esempio")).not.toBeInTheDocument();
    expect(within(row).getByText("Da prenotare")).toBeInTheDocument();
    expect(within(row).getByText("0900A0000000001")).toBeInTheDocument();
    expect(
      within(row).getByRole("button", { name: "Copia NRE 0900A0000000001" }),
    ).toHaveTextContent("0900A0000000001");
    expect(
      within(row).getByRole("button", { name: "Apri ricetta (PDF)" }),
    ).toBeInTheDocument();
    const book = within(row).getByRole("link", { name: "Prenota sul CUP" });
    expect(book).toHaveAttribute("href", "https://prenota.sanita.toscana.it/");
    expect(book).toHaveAttribute("target", "_blank");
  });

  async function bookedRowOf(): Promise<HTMLElement> {
    return waitFor(() => {
      const found = bodyRows().find(
        (r) => within(r).queryByText("Prenotata") !== null,
      );
      if (found === undefined) {
        throw new Error("not booked yet");
      }
      return found;
    });
  }

  it("shows a booked row with one-line appointment and structure and calendar button", async () => {
    mockData([appointment("0900A0000000001", "2026-11-12T09:30:00Z")], []);
    renderView();

    const row = await bookedRowOf();
    expect(within(row).getByText("gio 12/11 · 10:30")).toBeVisible();
    expect(within(row).queryByText("tra 2 giorni")).not.toBeInTheDocument();
    expect(
      within(row).getByRole("link", {
        name: "Apri in Google Maps: Unita esempio",
      }),
    ).toHaveAttribute(
      "href",
      "https://www.google.com/maps/search/?api=1&query=Via%20Esempio%201%2C%2050100%20Firenze%20FI",
    );
    expect(within(row).queryByText("Unita esempio")).not.toBeInTheDocument();
    expect(
      within(row).queryByText("Via Esempio 1, Firenze, FI"),
    ).not.toBeInTheDocument();
    expect(
      within(row).queryByRole("link", { name: "Apri su mappa" }),
    ).not.toBeInTheDocument();
    expect(
      within(row).getByRole("button", { name: "Dettagli appuntamento" }),
    ).toBeVisible();
    expect(
      within(row).getByRole("link", { name: /^Apri in Google Maps/ }),
    ).toHaveAttribute("target", "_blank");
    expect(
      within(row).getByRole("link", { name: /^Apri in Google Maps/ }),
    ).toHaveAttribute("rel", "noopener noreferrer");
    expect(
      within(row).getByRole("button", { name: "Aggiungi a Google Calendar" }),
    ).toBeVisible();
    expect(
      within(row).queryByRole("link", { name: "Prenota sul CUP" }),
    ).not.toBeInTheDocument();
  });

  it("shows the full structure name in a tooltip", async () => {
    const user = userEvent.setup();
    mockData([appointment("0900A0000000001", "2026-11-12T09:30:00Z")], []);
    renderView();

    const row = await bookedRowOf();
    await user.hover(
      within(row).getByRole("link", {
        name: "Apri in Google Maps: Unita esempio",
      }),
    );

    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Unita esempio",
    );
  });

  it("opens the detail dialog with the maps links", async () => {
    const user = userEvent.setup();
    mockData(
      [
        {
          ...appointment("0900A0000000001", "2026-11-12T09:30:00Z"),
          unit: {
            name: "Unita esempio",
            description: "",
            address: "Via A&B 1/c",
            postalCode: "50100",
            city: "Firenze",
            province: "FI",
          },
        },
      ],
      [],
    );
    renderView();

    const row = await bookedRowOf();
    await user.click(
      within(row).getByRole("button", { name: "Dettagli appuntamento" }),
    );

    const dialog = await screen.findByRole("dialog", {
      name: "Visita esempio",
    });
    expect(within(dialog).getByText(/giovedì 12 novembre 2026/)).toBeVisible();
    expect(within(dialog).getByText("tra 2 giorni")).toBeVisible();
    expect(within(dialog).getByText("Unita esempio")).toBeVisible();
    expect(
      within(dialog).getByText("Via A&B 1/c, 50100 Firenze FI"),
    ).toBeVisible();
    expect(
      within(dialog).getByRole("button", {
        name: "Copia NRE 0900A0000000001",
      }),
    ).toBeVisible();
    const directions = within(dialog).getByRole("link", {
      name: "Indicazioni stradali",
    });
    expect(directions).toHaveAttribute(
      "href",
      "https://www.google.com/maps/dir/?api=1&destination=Via%20A%26B%201%2Fc%2C%2050100%20Firenze%20FI",
    );
    expect(directions).toHaveAttribute("target", "_blank");
    expect(directions).toHaveAttribute("rel", "noopener noreferrer");
    expect(
      within(dialog).getByRole("link", { name: "Apri in Google Maps" }),
    ).toHaveAttribute(
      "href",
      "https://www.google.com/maps/search/?api=1&query=Via%20A%26B%201%2Fc%2C%2050100%20Firenze%20FI",
    );
  });

  it("hides the maps links when the unit has no address", async () => {
    const user = userEvent.setup();
    mockData(
      [
        {
          ...appointment("0900A0000000001", "2026-11-12T09:30:00Z"),
          unit: {
            name: "Unita esempio",
            description: "",
            address: "",
            city: "",
            province: "",
          },
        },
      ],
      [],
    );
    renderView();

    const row = await bookedRowOf();
    await user.click(
      within(row).getByRole("button", { name: "Dettagli appuntamento" }),
    );

    const dialog = await screen.findByRole("dialog", {
      name: "Visita esempio",
    });
    expect(
      within(dialog).queryByRole("link", { name: "Indicazioni stradali" }),
    ).not.toBeInTheDocument();
    expect(
      within(dialog).queryByRole("link", { name: "Apri in Google Maps" }),
    ).not.toBeInTheDocument();
  });

  it("shows the status on one line with an info button only when there are details", async () => {
    mockData([], ["0900A0000000005"]);
    renderView();

    const verify = await waitFor(() => rowOf("Da verificare"));
    const plain = rowOf("Visita esempio");
    expect(
      within(verify).getByRole("button", { name: "Dettagli stato" }),
    ).toBeVisible();
    expect(
      within(plain).queryByRole("button", { name: "Dettagli stato" }),
    ).not.toBeInTheDocument();
    expect(within(verify).queryAllByText(/FSE la segna/)).toHaveLength(0);
  });

  it("shows the status details in a tooltip on hover", async () => {
    const user = userEvent.setup();
    mockData([], ["0900A0000000005"]);
    renderView();

    const verify = await waitFor(() => rowOf("Da verificare"));
    await user.hover(
      within(verify).getByRole("button", { name: "Dettagli stato" }),
    );

    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      /FSE la segna come prenotata/,
    );
  });

  it("shows the status details in a popover on click without opening the row modal", async () => {
    const user = userEvent.setup();
    mockData([], ["0900A0000000005"]);
    renderView();

    const verify = await waitFor(() => rowOf("Da verificare"));
    await openStatusInfo(user, verify);

    expect(
      await screen.findByText(/FSE la segna come prenotata/),
    ).toBeVisible();
    expect(
      screen.queryByRole("dialog", { name: "Controllo esempio" }),
    ).not.toBeInTheDocument();
  });

  it("shows no map link for a row without a booking address", async () => {
    mockData([], []);
    renderView();

    const row = await waitFor(() => rowOf("Visita esempio"));
    expect(
      within(row).queryByRole("link", { name: /^Apri in Google Maps/ }),
    ).not.toBeInTheDocument();
  });

  it("shows no map link when the booked unit has no address", async () => {
    mockData(
      [
        {
          ...appointment("0900A0000000001", "2026-11-12T09:30:00Z"),
          unit: {
            name: "Unita esempio",
            description: "",
            address: "",
            city: "",
            province: "",
          },
        },
      ],
      [],
    );
    renderView();

    const row = await bookedRowOf();
    expect(
      within(row).queryByRole("link", { name: /^Apri in Google Maps/ }),
    ).not.toBeInTheDocument();
  });

  it("opens the detail modal on row click for an unbooked row", async () => {
    const user = userEvent.setup();
    mockData([], ["0900A0000000005"]);
    renderView();

    const row = await waitFor(() => rowOf("Da verificare"));
    await user.click(within(row).getByText("Controllo esempio"));

    const dialog = await screen.findByRole("dialog", {
      name: "Controllo esempio",
    });
    expect(within(dialog).getByText("Stato")).toBeVisible();
    expect(within(dialog).getByText("Da verificare")).toBeVisible();
    expect(
      within(dialog).getByText(/FSE la segna come prenotata/),
    ).toBeVisible();
    expect(within(dialog).getByText("Prescrizione")).toBeVisible();
    expect(
      within(dialog).getByText(formatDate("2026-10-15T09:00:00Z")),
    ).toBeVisible();
    expect(within(dialog).getByText("Azioni")).toBeVisible();
    expect(within(dialog).queryByText("Appuntamento")).not.toBeInTheDocument();
    expect(
      within(dialog).queryByRole("link", { name: "Apri in Google Maps" }),
    ).not.toBeInTheDocument();
    expect(
      within(dialog).getByRole("link", { name: "Prenota sul CUP" }),
    ).toHaveTextContent("Prenota sul CUP");
  });

  it("opens the detail modal on row click for a booked row with appointment and maps", async () => {
    const user = userEvent.setup();
    mockData([appointment("0900A0000000001", "2026-11-12T09:30:00Z")], []);
    renderView();

    const row = await bookedRowOf();
    await user.click(within(row).getByText("Visita esempio"));

    const dialog = await screen.findByRole("dialog", {
      name: "Visita esempio",
    });
    expect(within(dialog).getByText("Appuntamento")).toBeVisible();
    expect(within(dialog).getByText(/giovedì 12 novembre 2026/)).toBeVisible();
    expect(
      within(dialog).getByRole("link", { name: "Indicazioni stradali" }),
    ).toBeVisible();
    expect(
      within(dialog).getByRole("link", { name: "Apri in Google Maps" }),
    ).toBeVisible();
    expect(
      within(dialog).getByRole("button", {
        name: "Aggiungi a Google Calendar",
      }),
    ).toHaveTextContent("Aggiungi a Google Calendar");
  });

  it("opens the detail modal with Enter on a focused row", async () => {
    const user = userEvent.setup();
    renderView();

    const row = await waitFor(() => rowOf("Visita esempio"));
    row.focus();
    await user.keyboard("{Enter}");

    expect(
      await screen.findByRole("dialog", { name: "Visita esempio" }),
    ).toBeInTheDocument();
  });

  it("runs the modal actions: opens the PDF and copies the NRE", async () => {
    const user = userEvent.setup();
    const writeText = vi
      .spyOn(navigator.clipboard, "writeText")
      .mockResolvedValue(undefined);
    renderView();

    const row = await waitFor(() => rowOf("Visita esempio"));
    await user.click(within(row).getByText("Visita esempio"));
    const dialog = await screen.findByRole("dialog", {
      name: "Visita esempio",
    });

    await user.click(
      within(dialog).getByRole("button", { name: "Copia NRE 0900A0000000001" }),
    );
    expect(writeText).toHaveBeenCalledWith("0900A0000000001");

    await user.click(
      within(dialog).getByRole("button", { name: "Apri ricetta (PDF)" }),
    );
    expect(
      await screen.findByRole("dialog", { name: "Ricetta" }),
    ).toBeInTheDocument();
  });

  it("does not open the detail modal from an Azioni icon", async () => {
    const user = userEvent.setup();
    renderView();

    const row = await waitFor(() => rowOf("Visita esempio"));
    await user.click(
      within(row).getByRole("button", { name: "Apri ricetta (PDF)" }),
    );

    expect(
      await screen.findByRole("dialog", { name: "Ricetta" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("dialog", { name: "Visita esempio" }),
    ).not.toBeInTheDocument();
  });

  it("shows no calendar button for past or unbooked rows", async () => {
    mockData([appointment("0900A0000000005", "2026-11-05T09:30:00Z")], []);
    renderView();

    await screen.findByText("Visita esempio");
    await screen.findByText("Controllo esempio");
    expect(
      screen.queryByRole("button", { name: "Aggiungi a Google Calendar" }),
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

  it("shows to verify, the note and Prenota sul CUP for an unbooked FSE 30 row", async () => {
    mockData([], ["0900A0000000005"]);
    renderView();

    const user = userEvent.setup();
    const row = await waitFor(() => rowOf("Da verificare"));
    expect(
      within(row).queryByText(/FSE la segna come prenotata/),
    ).not.toBeInTheDocument();
    await openStatusInfo(user, row);
    const note = await screen.findByText(
      "FSE la segna come prenotata, ma nel CUP non c'è un tuo appuntamento: può essere già gestita altrove (prenotazione o accettazione) o ancora prenotabile",
    );
    expect(note).toBeVisible();
    expect(
      within(row).getByRole("link", { name: "Prenota sul CUP" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Prenotata fuori dal CUP regionale"),
    ).not.toBeInTheDocument();
  });

  it("renders (stimata) for an estimated expiry", async () => {
    const estimated: RowBody = {
      nre: "0900A0000000070",
      fse: {
        id: "f70",
        nre: "0900A0000000070",
        description: "Visita stimata esempio",
        kind: "visit",
        status: 3,
        date: "2026-11-01T09:00:00Z",
        expiresOn: "2099-12-01T09:00:00Z",
        expiryEstimated: true,
        documentRef: "",
      },
    };
    const user = userEvent.setup();
    mockRows([estimated], []);
    renderView();

    await openStatusInfo(
      user,
      await waitFor(() => rowOf("Visita stimata esempio")),
    );

    expect(await screen.findByText(/scade il .*\(stimata\)/)).toBeVisible();
  });

  it("adds a row for an appointment without a prescription", async () => {
    mockData([appointment("0900A0000000099", "2026-11-15T09:00:00Z")], []);
    renderView();

    expect(await screen.findByText("0900A0000000099")).toBeInTheDocument();
  });

  it("hides Erogata, Annullata and Scaduta rows by default", async () => {
    const user = userEvent.setup();
    renderView();

    await screen.findByText("Visita esempio");
    expect(screen.queryByText("Esame chiuso esempio")).not.toBeInTheDocument();
    expect(screen.getByText("3 visite/esami")).toBeInTheDocument();
    expect(screen.queryByRole("radio")).not.toBeInTheDocument();

    await openStatusFilter(user);

    expect(
      await screen.findByRole("checkbox", { name: "Da prenotare (2)" }),
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Prenotata (1)" }),
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Erogata (1)" }),
    ).not.toBeChecked();
  });

  it("lists the statuses in the fixed order", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");

    await openStatusFilter(user);

    await screen.findByRole("checkbox", { name: "Erogata (1)" });
    expect(
      screen.getAllByRole("checkbox").map((box) => box.getAttribute("value")),
    ).toEqual(["Prenotata", "Da prenotare", "Erogata"]);
  });

  it("sorts the default view by prescription date desc", async () => {
    renderView();

    await screen.findByText("Visita esempio");
    expect(sortLabel("Prescritta")).toBe("Sorted descending");
    expect(titles()).toEqual([
      "Ecografia esempio, Prelievo esempio",
      "Visita esempio",
      "Controllo esempio",
    ]);
  });

  it("shows Erogata rows after ticking Erogata", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");

    await pickStatuses(user, "Erogata (1)");

    expect(titles()).toHaveLength(4);
    expect(screen.getByText("Esame chiuso esempio")).toBeInTheDocument();
  });

  it("shows every row with Tutti", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");
    await openStatusFilter(user);

    await user.click(await screen.findByRole("button", { name: "Tutti" }));

    expect(titles()).toHaveLength(4);
    expect(screen.getByText("4 visite/esami")).toBeInTheDocument();
  });

  it("shows the empty text with Nessuno", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");
    await openStatusFilter(user);

    await user.click(await screen.findByRole("button", { name: "Nessuno" }));

    expect(
      screen.getByText("Nessuna visita o esame in questa vista."),
    ).toBeInTheDocument();
    expect(screen.getByText("0 visite/esami")).toBeInTheDocument();
  });

  it("hides a row again after unticking its status", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");

    await pickStatuses(user, "Prenotata (1)");

    expect(screen.queryByText("Controllo esempio")).not.toBeInTheDocument();
    expect(titles()).toHaveLength(2);
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

    await openStatusFilter(user);

    expect(
      await screen.findByRole("checkbox", { name: "Da prenotare (1)" }),
    ).toBeVisible();
    expect(
      screen.queryByRole("checkbox", { name: /Prenotata/ }),
    ).not.toBeInTheDocument();
  });

  it("finds a row by unit city", async () => {
    const user = userEvent.setup();
    mockData([appointment("0900A0000000005", "2026-11-20T09:00:00Z")], []);
    renderView();
    await screen.findByText("Visita esempio");
    await screen.findByRole("link", {
      name: "Apri in Google Maps: Unita esempio",
    });

    await user.type(screen.getByRole("searchbox"), "firenze");

    expect(titles()).toEqual(["Controllo esempio"]);
  });

  it("sorts by a header click and shows the sort direction", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");

    await user.click(sortHeader("Prestazione"));

    expect(sortLabel("Prestazione")).toBe("Sorted ascending");
    expect(titles()).toEqual([
      "Controllo esempio",
      "Ecografia esempio, Prelievo esempio",
      "Visita esempio",
    ]);

    await user.click(sortHeader("Prestazione"));

    expect(sortLabel("Prestazione")).toBe("Sorted descending");
    expect(titles()[0]).toBe("Visita esempio");
  });

  it("keeps prescription date desc by default when appointments exist", async () => {
    mockData([appointment("0900A0000000005", "2026-11-20T09:00:00Z")], []);
    renderView();
    await screen.findByRole("link", {
      name: "Apri in Google Maps: Unita esempio",
    });

    expect(titles()).toEqual([
      "Ecografia esempio, Prelievo esempio",
      "Visita esempio",
      "Controllo esempio",
    ]);
    expect(sortLabel("Appuntamento")).toBe("Not sorted");
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
    const spy = stubScroll(scroll);
    window.location.hash = "#/visite/0900A0000000004";
    try {
      renderView();
      const row = (
        await screen.findByText("Ecografia esempio, Prelievo esempio")
      ).closest("tr");

      expect(row).toHaveAttribute("data-focused", "true");
      expect(row).toHaveFocus();
      expect(scroll).toHaveBeenCalled();
    } finally {
      spy.mockRestore();
    }
  });

  it("shows and focuses a deep-linked row whose status is excluded", async () => {
    const user = userEvent.setup();
    const spy = stubScroll(vi.fn());
    window.location.hash = "#/visite/0900A0000000003";
    try {
      renderView();
      const row = (await screen.findByText("Esame chiuso esempio")).closest(
        "tr",
      );

      expect(row).toHaveAttribute("data-focused", "true");
      expect(row).toHaveFocus();

      await openStatusFilter(user);

      expect(
        await screen.findByRole("checkbox", { name: "Erogata (1)" }),
      ).toBeChecked();
    } finally {
      spy.mockRestore();
    }
  });

  it("hides expired rows by default and shows them after ticking Scaduta", async () => {
    const user = userEvent.setup();
    mockRows(
      [
        fseVisit("0900A0000000063", "Visita scaduta", "2026-11-09T09:00:00Z"),
        fseVisit("0900A0000000064", "Visita valida", "2026-11-20T09:00:00Z"),
      ],
      [],
    );
    renderView();
    await screen.findByText("Visita valida");
    expect(titles()).toEqual(["Visita valida"]);

    await pickStatuses(user, "Scaduta (1)");

    expect(titles()).toHaveLength(2);
    const row = rowOf("Visita scaduta");
    expect(within(row).getByText("Scaduta")).toBeInTheDocument();
    expect(within(row).queryByText(/^scaduta il /)).not.toBeInTheDocument();
    await openStatusInfo(user, row);
    expect(await screen.findByText(/^scaduta il /)).toBeVisible();
    expect(
      within(row).queryByRole("link", { name: "Prenota sul CUP" }),
    ).not.toBeInTheDocument();
  });

  it("shows an expired FSE status 30 row as Scaduta, not Da verificare", async () => {
    const user = userEvent.setup();
    const expired30: RowBody = {
      nre: "0900A0000000066",
      fse: {
        id: "f66",
        nre: "0900A0000000066",
        description: "Controllo scaduto",
        kind: "visit",
        status: 30,
        date: "2026-11-01T09:00:00Z",
        expiresOn: "2026-11-09T09:00:00Z",
        expiryEstimated: true,
        documentRef: "",
      },
    };
    mockRows([expired30], []);
    mocks.get.mockImplementation((path: string) => {
      if (path === "/api/appointments") {
        return Promise.resolve({
          data: { appointments: [], failed: [], unbooked: ["0900A0000000066"] },
        });
      }
      return Promise.resolve({ data: { prescriptions: [expired30] } });
    });
    renderView();
    await screen.findByText("Nessuna visita o esame in questa vista.");

    await pickStatuses(user, "Scaduta (1)");

    const row = rowOf("Controllo scaduto");
    expect(within(row).getByText("Scaduta")).toBeInTheDocument();
    await openStatusInfo(user, row);
    expect(await screen.findByText(/\(stimata\)/)).toBeVisible();
    expect(within(row).queryByText("Da verificare")).not.toBeInTheDocument();
  });

  it("shows and focuses a deep-linked expired row", async () => {
    const spy = stubScroll(vi.fn());
    window.location.hash = "#/visite/0900A0000000065";
    mockRows(
      [fseVisit("0900A0000000065", "Visita scaduta", "2026-11-09T09:00:00Z")],
      [],
    );
    try {
      renderView();
      const row = (await screen.findByText("Visita scaduta")).closest("tr");

      expect(row).toHaveAttribute("data-focused", "true");
      expect(row).toHaveFocus();
    } finally {
      spy.mockRestore();
    }
  });

  it("clears the search when the focused row is filtered out by it", async () => {
    const user = userEvent.setup();
    const spy = stubScroll(vi.fn());
    renderView();
    await screen.findByText("Visita esempio");
    await user.type(
      screen.getByRole("searchbox", { name: "Cerca" }),
      "ECOGRAFIA",
    );
    expect(titles()).toHaveLength(1);

    act(() => {
      window.location.hash = "#/visite/0900A0000000001";
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });

    await waitFor(() => {
      expect(titles()).toHaveLength(3);
    });
    expect(screen.getByRole("searchbox", { name: "Cerca" })).toHaveValue("");
    spy.mockRestore();
  });

  it("shows scade il for a valid status 3 prescription", async () => {
    const user = userEvent.setup();
    mockRows(
      [fseVisit("0900A0000000062", "Visita valida", "2026-11-20T09:00:00Z")],
      [],
    );
    renderView();

    const row = await waitFor(() => rowOf("Visita valida"));
    await openStatusInfo(user, row);
    expect(await screen.findByText(/^scade il /)).toBeVisible();
    expect(within(row).getByText("Da prenotare")).toBeInTheDocument();
    expect(
      within(row).getByRole("link", { name: "Prenota sul CUP" }),
    ).toBeInTheDocument();
  });

  it("links the promemoria PDF on a booked row with a booking id", async () => {
    mockRows(
      [toBook],
      [
        {
          ...appointment("0900A0000000001", "2026-11-12T09:30:00Z"),
          bookingId: "ABC123456789",
        },
      ],
    );
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
    await userEvent.setup().click(
      within(row).getByRole("button", {
        name: "Apri promemoria prenotazione (PDF)",
      }),
    );
    expect(
      await screen.findByRole("dialog", { name: "Promemoria" }),
    ).toBeInTheDocument();
  });

  it("has no promemoria link on a booked row without a booking id", async () => {
    mockRows(
      [toBook],
      [appointment("0900A0000000001", "2026-11-12T09:30:00Z")],
    );
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
    expect(
      within(row).queryByRole("button", {
        name: "Apri promemoria prenotazione (PDF)",
      }),
    ).not.toBeInTheDocument();
  });
});
