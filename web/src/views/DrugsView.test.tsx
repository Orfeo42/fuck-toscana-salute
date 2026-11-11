import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { JSX } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { RowBody, SessionBody } from "../api/client";
import { HealthDataProvider } from "../data/HealthDataProvider";
import { SessionContext } from "../session/useSession";
import { callsTo } from "../test/calls";
import { DrugsView } from "./DrugsView";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
}));

vi.mock("../api/client", () => ({
  api: { GET: mocks.get },
  prescriptionPdfUrl: (ref: string) => `/api/documents/prescription?ref=${ref}`,
}));

function drug(
  suffix: string,
  description: string,
  status: number,
  date: string,
): RowBody {
  const nre = `0900A00000000${suffix}`;
  return {
    nre,
    fse: {
      id: `f${suffix}`,
      nre,
      description,
      kind: "drug",
      status,
      date,
      documentRef: `ref-${suffix}`,
    },
  };
}

const pickupOld = drug(
  "01",
  "Farmaco esempio antico",
  3,
  "2026-10-01T09:00:00Z",
);
const pickupNew = drug("02", "Farmaco esempio", 3, "2026-11-02T09:00:00Z");
const done = drug("03", "Farmaco erogato esempio", 8, "2026-09-01T09:00:00Z");
const visit: RowBody = {
  nre: "0900A0000000004",
  fse: {
    id: "f4",
    nre: "0900A0000000004",
    description: "Visita esempio",
    kind: "visit",
    status: 3,
    date: "2026-11-04T09:00:00Z",
    documentRef: "ref-4",
  },
};

const rows = [pickupOld, pickupNew, done, visit];

function stubScroll(scroll: () => void): void {
  Object.defineProperty(Element.prototype, "scrollIntoView", {
    configurable: true,
    writable: true,
    value: scroll,
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
        <DrugsView />
      </HealthDataProvider>
    </SessionContext.Provider>
  );
}

function at(items: HTMLElement[], index: number): HTMLElement {
  const item = items[index];
  if (item === undefined) {
    throw new Error(`no item at ${String(index)}`);
  }
  return item;
}

function bodyRows(): HTMLElement[] {
  return screen.getAllByRole("row").slice(1);
}

function descriptions(): (string | null)[] {
  return bodyRows().map(
    (row) => within(row).getAllByRole("cell")[1]?.textContent ?? null,
  );
}

describe("DrugsView", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    window.location.hash = "";
    mocks.get.mockResolvedValue({ data: { prescriptions: rows } });
  });

  it("shows the loading state", () => {
    mocks.get.mockReturnValue(new Promise(() => undefined));
    render(<Harness />);

    expect(screen.getByText("Carico i farmaci…")).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Farmaci" })).toHaveAttribute(
      "aria-busy",
      "true",
    );
  });

  it("excludes visit rows", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await screen.findByText("Farmaco esempio");

    await user.click(screen.getByRole("radio", { name: /Tutte/ }));

    expect(screen.queryByText("Visita esempio")).not.toBeInTheDocument();
    expect(bodyRows()).toHaveLength(3);
  });

  it("renders the columns with data-labels", async () => {
    render(<Harness />);
    await screen.findByText("Farmaco esempio");

    const headers = screen
      .getAllByRole("columnheader")
      .map((h) => h.textContent);
    expect(headers).toEqual(["Data▼", "Farmaco", "Stato", "NRE", "Azioni"]);
    const cells = within(at(bodyRows(), 0)).getAllByRole("cell");
    expect(cells.map((cell) => cell.getAttribute("data-label"))).toEqual([
      "Data",
      "Farmaco",
      "Stato",
      "NRE",
      "Azioni",
    ]);
    expect(within(at(cells, 2)).getByText("Da ritirare")).toBeVisible();
    expect(within(at(cells, 3)).getByText("0900A0000000002")).toBeVisible();
  });

  it("shows Da ritirare by default with counts, newest first", async () => {
    render(<Harness />);
    await screen.findByText("Farmaco esempio");

    expect(descriptions()).toEqual([
      "Farmaco esempio",
      "Farmaco esempio antico",
    ]);
    expect(
      screen.getByRole("radio", { name: "Da ritirare (2)" }),
    ).toBeChecked();
    expect(screen.getByRole("radio", { name: "Tutte (3)" })).toBeVisible();
    expect(screen.getByText("2 farmaci")).toBeInTheDocument();
  });

  it("switches to Tutte", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await screen.findByText("Farmaco esempio");

    await user.click(screen.getByRole("radio", { name: /Tutte/ }));

    expect(bodyRows()).toHaveLength(3);
    expect(screen.getByText("Erogata")).toBeInTheDocument();
    expect(screen.getByText("3 farmaci")).toBeInTheDocument();
  });

  it("narrows the list with the search box", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await screen.findByText("Farmaco esempio");

    await user.type(screen.getByRole("searchbox", { name: "Cerca" }), "ANTICO");

    expect(descriptions()).toEqual(["Farmaco esempio antico"]);
    expect(screen.getByText("1 farmaci")).toBeInTheDocument();
  });

  it("finds a row by NRE", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await screen.findByText("Farmaco esempio");

    await user.type(screen.getByRole("searchbox"), "0900A0000000002");

    expect(descriptions()).toEqual(["Farmaco esempio"]);
  });

  it("sorts by header with aria-sort", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await screen.findByText("Farmaco esempio");
    const dataHeader = screen.getByRole("columnheader", { name: /Data/ });
    expect(dataHeader).toHaveAttribute("aria-sort", "descending");

    await user.click(within(dataHeader).getByRole("button"));

    expect(dataHeader).toHaveAttribute("aria-sort", "ascending");
    expect(descriptions()).toEqual([
      "Farmaco esempio antico",
      "Farmaco esempio",
    ]);

    await user.click(screen.getByRole("button", { name: "NRE" }));

    expect(screen.getByRole("columnheader", { name: /NRE/ })).toHaveAttribute(
      "aria-sort",
      "ascending",
    );
    expect(dataHeader).toHaveAttribute("aria-sort", "none");
    expect(descriptions()).toEqual([
      "Farmaco esempio antico",
      "Farmaco esempio",
    ]);
  });

  it("links the PDF", async () => {
    render(<Harness />);
    await screen.findByText("Farmaco esempio");

    expect(
      within(at(bodyRows(), 0)).getByRole("link", {
        name: "Apri ricetta (PDF)",
      }),
    ).toHaveAttribute("href", "/api/documents/prescription?ref=ref-02");
  });

  it("renders the copy button", async () => {
    const user = userEvent.setup();
    const writeText = vi.spyOn(navigator.clipboard, "writeText");
    writeText.mockResolvedValue(undefined);
    render(<Harness />);
    await screen.findByText("Farmaco esempio");

    await user.click(
      within(at(bodyRows(), 0)).getByRole("button", {
        name: "Copia NRE 0900A0000000002",
      }),
    );

    expect(writeText).toHaveBeenCalledWith("0900A0000000002");
  });

  it("shows the login message on 401", async () => {
    mocks.get.mockResolvedValue({
      error: { detail: "no session" },
      response: { status: 401 },
    });
    render(<Harness />);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Accedi per vedere i farmaci.",
    );
  });

  it("shows the portal message when unavailable", async () => {
    mocks.get.mockResolvedValue({
      error: { detail: "bad gateway" },
      response: { status: 502 },
    });
    render(<Harness />);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Il portale regionale non risponde. Riprova tra poco.",
    );
  });

  it("refetches on Aggiorna", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await screen.findByText("Farmaco esempio");
    expect(callsTo(mocks.get, "/api/prescriptions")).toBe(1);

    await user.click(screen.getByRole("button", { name: "Aggiorna" }));

    await waitFor(() => {
      expect(callsTo(mocks.get, "/api/prescriptions")).toBe(2);
    });
  });

  it("treats a null list as empty", async () => {
    mocks.get.mockResolvedValue({ data: { prescriptions: null } });
    render(<Harness />);

    expect(
      await screen.findByText("Nessun farmaco in questa vista."),
    ).toBeInTheDocument();
    expect(screen.getByText("0 farmaci")).toBeInTheDocument();
  });

  it("focuses the row of a deep-linked NRE", async () => {
    const scroll = vi.fn();
    stubScroll(scroll);
    window.location.hash = "#/farmaci/0900A0000000002";
    try {
      render(<Harness />);
      const row = (await screen.findByText("Farmaco esempio")).closest("tr");

      expect(row).toHaveClass("is-focused");
      expect(row).toHaveFocus();
      expect(scroll).toHaveBeenCalled();
    } finally {
      Reflect.deleteProperty(Element.prototype, "scrollIntoView");
    }
  });

  it("shows Tutte when the focused row is hidden", async () => {
    stubScroll(vi.fn());
    window.location.hash = "#/farmaci/0900A0000000003";
    try {
      render(<Harness />);
      const row = (await screen.findByText("Farmaco erogato esempio")).closest(
        "tr",
      );

      expect(row).toHaveClass("is-focused");
      expect(screen.getByRole("radio", { name: /Tutte/ })).toBeChecked();
    } finally {
      Reflect.deleteProperty(Element.prototype, "scrollIntoView");
    }
  });
});
