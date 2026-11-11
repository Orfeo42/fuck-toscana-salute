import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { AppointmentBody, SessionBody } from "../api/client";
import { callsTo } from "../test/calls";
import { HealthDataProvider } from "../data/HealthDataProvider";
import { SessionContext } from "../session/useSession";
import { AppointmentsView } from "./AppointmentsView";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
}));

vi.mock("../api/client", () => ({
  api: { GET: mocks.get },
  prescriptionPdfUrl: (ref: string) => `/pdf?ref=${encodeURIComponent(ref)}`,
}));

const now = new Date("2026-11-10T10:00:00Z");

const session: SessionBody = {
  gestioneReady: true,
  portals: [
    { portal: "cup", state: "active", login: { phase: "idle" } },
    { portal: "fse", state: "active", login: { phase: "idle" } },
  ],
};

function appointment(
  id: string,
  overrides: Partial<AppointmentBody> = {},
): AppointmentBody {
  return {
    id,
    nre: "0900A0000000001",
    status: "PRENOTATO",
    at: "2026-11-12T09:30:00Z",
    unit: {
      name: "OSP ESEMPIO",
      description: "Ospedale Esempio",
      address: "Via Roma 1",
      city: "Firenze",
      province: "FI",
    },
    ...overrides,
  };
}

function renderView(): void {
  render(
    <SessionContext
      value={{
        session,
        error: undefined,
        refresh: vi.fn(),
        loginAll: vi.fn(),
        logout: vi.fn(),
      }}
    >
      <HealthDataProvider>
        <AppointmentsView now={now} />
      </HealthDataProvider>
    </SessionContext>,
  );
}

function mockApi(
  appointments: AppointmentBody[],
  failed: { nre: string; reason: string }[],
  prescriptionNres: string[],
): void {
  mocks.get.mockImplementation((path: string) =>
    Promise.resolve(
      path === "/api/prescriptions"
        ? {
            data: {
              prescriptions: prescriptionNres.map((nre) => ({ nre })),
            },
          }
        : { data: { appointments, failed } },
    ),
  );
}

describe("AppointmentsView", () => {
  beforeEach(() => {
    mocks.get.mockReset();
  });

  afterEach(() => {
    window.location.hash = "";
  });

  it("shows the busy state while loading", () => {
    mocks.get.mockReturnValue(new Promise(() => undefined));
    renderView();

    expect(
      screen.getByText("Cerco gli appuntamenti… (qualche secondo)"),
    ).toBeInTheDocument();
    expect(screen.getByRole("region", { busy: true })).toBeInTheDocument();
  });

  it("lists upcoming appointments sorted with relative day and unit", async () => {
    mocks.get.mockResolvedValue({
      data: {
        appointments: [
          appointment("late", { at: "2026-11-20T09:30:00Z" }),
          appointment("soon", { at: "2026-11-11T09:30:00Z" }),
        ],
        failed: [],
      },
    });
    renderView();

    expect(
      await screen.findByRole("heading", {
        name: "Prossimi appuntamenti (2)",
      }),
    ).toBeInTheDocument();
    expect(
      screen
        .getAllByText(/^(domani|tra \d+ giorni)$/)
        .map((element) => element.textContent),
    ).toEqual(["domani", "tra 10 giorni"]);
    expect(screen.getAllByText("Ospedale Esempio")).toHaveLength(2);
    expect(screen.getAllByText("10:30")).toHaveLength(2);
    expect(screen.getAllByText("Prestazione")).toHaveLength(2);
  });

  it("shows today for same-day appointments", async () => {
    mocks.get.mockResolvedValue({
      data: {
        appointments: [appointment("a", { at: "2026-11-10T15:00:00Z" })],
        failed: [],
      },
    });
    renderView();

    expect(await screen.findByText("oggi")).toBeInTheDocument();
  });

  it("moves cancelled and past appointments to the past section", async () => {
    mocks.get.mockResolvedValue({
      data: {
        appointments: [
          appointment("up"),
          appointment("cancelled", { status: "DISDETTO" }),
          appointment("old", { at: "2026-10-01T09:30:00Z" }),
        ],
        failed: [],
      },
    });
    renderView();

    expect(
      await screen.findByRole("heading", {
        name: "Prossimi appuntamenti (1)",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Passati e disdetti (2)")).toBeInTheDocument();
    expect(screen.getByText("Disdetto")).toBeInTheDocument();
  });

  it("shows the empty message", async () => {
    mocks.get.mockResolvedValue({ data: { appointments: [], failed: [] } });
    renderView();

    expect(
      await screen.findByText("Nessun appuntamento in programma."),
    ).toBeInTheDocument();
  });

  it("treats null arrays as empty", async () => {
    mocks.get.mockResolvedValue({
      data: { appointments: null, failed: null },
    });
    renderView();

    expect(
      await screen.findByText("Nessun appuntamento in programma."),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Prenotazioni non trovate/)).toBeNull();
  });

  it("shows failed lookups with the not_in_cup label", async () => {
    mocks.get.mockResolvedValue({
      data: {
        appointments: [],
        failed: [{ nre: "0900A0000000002", reason: "not_in_cup" }],
      },
    });
    renderView();

    expect(
      await screen.findByText("Prenotazioni non trovate nel CUP regionale (1)"),
    ).toBeInTheDocument();
    expect(screen.getByText("0900A0000000002")).toBeInTheDocument();
    expect(
      screen.getByText(/forse fatta per telefono o allo sportello/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Controlla l'SMS o il promemoria ricevuto alla prenotazione.",
      ),
    ).toBeInTheDocument();
  });

  it("shows the 401 message", async () => {
    mocks.get.mockResolvedValue({ error: {}, response: { status: 401 } });
    renderView();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Accedi per vedere gli appuntamenti.",
    );
  });

  it("shows the 409 message", async () => {
    mocks.get.mockResolvedValue({ error: {}, response: { status: 409 } });
    renderView();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Per vedere date e orari serve l'accesso a Gestione Prenotazioni: premi Accedi.",
    );
  });

  it("shows the 502 message", async () => {
    mocks.get.mockResolvedValue({ error: {}, response: { status: 502 } });
    renderView();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Il portale regionale non risponde. Riprova tra poco.",
    );
  });

  it("shows the unavailable message when the request throws", async () => {
    mocks.get.mockRejectedValue(new Error("network"));
    renderView();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Il portale regionale non risponde. Riprova tra poco.",
    );
  });

  it("refetches on Aggiorna", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: { appointments: [], failed: [] } });
    renderView();
    await screen.findByText("Nessun appuntamento in programma.");
    expect(callsTo(mocks.get, "/api/appointments")).toBe(1);

    await user.click(screen.getByRole("button", { name: "Aggiorna" }));

    await screen.findByText("Nessun appuntamento in programma.");
    expect(callsTo(mocks.get, "/api/appointments")).toBe(2);
  });

  it("shows the PDF link when documentRef is set", async () => {
    mocks.get.mockResolvedValue({
      data: {
        appointments: [
          appointment("a", {
            fse: {
              description: "Visita cardiologica",
              documentRef: "doc1",
              date: "2026-11-01T00:00:00Z",
              status: 1,
              kind: "x",
              id: "f1",
              nre: "0900A0000000001",
            },
          }),
        ],
        failed: [],
      },
    });
    renderView();

    expect(
      await screen.findByRole("link", { name: "Apri ricetta (PDF)" }),
    ).toHaveAttribute("href", "/pdf?ref=doc1");
    expect(screen.getByText("Visita cardiologica")).toBeInTheDocument();
  });

  it("builds an encoded map link", async () => {
    mocks.get.mockResolvedValue({
      data: {
        appointments: [
          appointment("a", {
            unit: {
              name: "X",
              description: "",
              address: "Via dell'Orto 5/A",
              city: "Pisa",
              province: "PI",
            },
          }),
        ],
        failed: [],
      },
    });
    renderView();

    const link = await screen.findByRole("link", { name: "Mappa" });
    expect(link).toHaveAttribute(
      "href",
      "https://www.openstreetmap.org/search?query=Via%20dell'Orto%205%2FA%20Pisa",
    );
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("shows the Google Calendar link on upcoming booked cards only", async () => {
    mockApi(
      [
        appointment("up"),
        appointment("cancelled", {
          status: "DISDETTO",
          nre: "0900A0000000003",
        }),
      ],
      [],
      [],
    );
    renderView();

    const links = await screen.findAllByRole("link", {
      name: "Aggiungi a Google Calendar",
    });
    expect(links).toHaveLength(1);
    expect(links[0]).toHaveAttribute(
      "href",
      expect.stringContaining("https://calendar.google.com/calendar/render?"),
    );
    expect(links[0]).toHaveAttribute("target", "_blank");
    expect(links[0]).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("links to the prescription only when its NRE is in the prescriptions", async () => {
    mockApi(
      [appointment("a"), appointment("b", { nre: "0900A0000000004" })],
      [
        { nre: "0900A0000000002", reason: "not_in_cup" },
        { nre: "0900A0000000005", reason: "not_in_cup" },
      ],
      ["0900a0000000001", "0900A0000000002"],
    );
    renderView();

    await screen.findByText("Prenotazioni non trovate nel CUP regionale (2)");
    const links = screen.getAllByRole("link", { name: "Vai alla ricetta" });
    expect(links.map((link) => link.getAttribute("href"))).toEqual([
      "#/ricette/0900A0000000001",
      "#/ricette/0900A0000000002",
    ]);
  });

  it("highlights and focuses the card targeted by the hash", async () => {
    const scrollIntoView = vi.fn();
    Object.defineProperty(Element.prototype, "scrollIntoView", {
      value: scrollIntoView,
      configurable: true,
      writable: true,
    });
    window.location.hash = "#/appuntamenti/0900A0000000001";
    mockApi([appointment("a")], [], []);
    try {
      renderView();

      await screen.findByRole("heading", { name: "Prossimi appuntamenti (1)" });
      const card = document.getElementById("appuntamento-0900A0000000001");
      expect(card).toHaveClass("is-focused");
      expect(card).toHaveFocus();
      expect(scrollIntoView).toHaveBeenCalled();
    } finally {
      Reflect.deleteProperty(Element.prototype, "scrollIntoView");
    }
  });

  it("opens the past section and focuses a past row from the hash", async () => {
    Object.defineProperty(Element.prototype, "scrollIntoView", {
      value: vi.fn(),
      configurable: true,
      writable: true,
    });
    window.location.hash = "#/appuntamenti/0900A0000000006";
    mockApi(
      [
        appointment("old", {
          nre: "0900A0000000006",
          at: "2026-10-01T09:30:00Z",
        }),
      ],
      [],
      [],
    );
    try {
      renderView();

      await screen.findByText("Passati e disdetti (1)");
      const row = document.getElementById("appuntamento-0900A0000000006");
      expect(row?.closest("details")).toHaveAttribute("open");
      expect(row).toHaveClass("is-focused");
      expect(row).toHaveFocus();
    } finally {
      Reflect.deleteProperty(Element.prototype, "scrollIntoView");
    }
  });

  it("opens the failed section and focuses a failed item from the hash", async () => {
    Object.defineProperty(Element.prototype, "scrollIntoView", {
      value: vi.fn(),
      configurable: true,
      writable: true,
    });
    window.location.hash = "#/appuntamenti/0900A0000000002";
    mockApi([], [{ nre: "0900A0000000002", reason: "not_in_cup" }], []);
    try {
      renderView();

      await screen.findByText("Prenotazioni non trovate nel CUP regionale (1)");
      const item = document.getElementById("appuntamento-0900A0000000002");
      expect(item?.closest("details")).toHaveAttribute("open");
      expect(item).toHaveClass("is-focused");
      expect(item).toHaveFocus();
    } finally {
      Reflect.deleteProperty(Element.prototype, "scrollIntoView");
    }
  });

  it("keeps sections closed without a matching focus", async () => {
    mockApi([appointment("old", { at: "2026-10-01T09:30:00Z" })], [], []);
    renderView();

    await screen.findByText("Passati e disdetti (1)");
    expect(
      screen.getByText("Passati e disdetti (1)").closest("details"),
    ).not.toHaveAttribute("open");
  });
});
