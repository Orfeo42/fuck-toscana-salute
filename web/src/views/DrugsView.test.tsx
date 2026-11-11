import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { JSX } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { RowBody, SessionBody } from "../api/client";
import { HealthDataProvider } from "../data/HealthDataProvider";
import { SessionContext } from "../session/useSession";
import { callsTo } from "../test/calls";
import { DrugsView } from "./DrugsView";
import { render } from "../test/render";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
}));

vi.mock("../lib/pdf", () => ({
  loadPdf: () => ({
    promise: new Promise<never>(() => undefined),
    destroy: () => Promise.resolve(),
  }),
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
  vi.spyOn(HTMLElement.prototype, "scrollIntoView").mockImplementation(scroll);
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

type User = ReturnType<typeof userEvent.setup>;

function sortHeader(name: string): HTMLElement {
  return screen.getByRole("button", { name: new RegExp(`^${name}`) });
}

function sortLabel(name: string): string | null {
  return within(sortHeader(name)).getByRole("img").getAttribute("aria-label");
}

async function openStatusFilter(user: User): Promise<void> {
  await user.click(within(sortHeader("Stato")).getByRole("button"));
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

    await openStatusFilter(user);
    await user.click(await screen.findByRole("button", { name: "Tutti" }));

    expect(screen.queryByText("Visita esempio")).not.toBeInTheDocument();
    expect(bodyRows()).toHaveLength(3);
  });

  it("renders the columns", async () => {
    render(<Harness />);
    await screen.findByText("Farmaco esempio");

    const headers = [...screen.getByRole("table").querySelectorAll("th")].map(
      (h) => h.textContent,
    );
    expect(headers).toEqual(["Data", "Farmaco", "Stato", "NRE", "Azioni"]);
    const cells = within(at(bodyRows(), 0)).getAllByRole("cell");
    expect(cells).toHaveLength(5);
    expect(within(at(cells, 2)).getByText("Da ritirare")).toBeVisible();
    expect(within(at(cells, 3)).getByText("0900A0000000002")).toBeVisible();
  });

  it("hides closed statuses by default, newest first", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await screen.findByText("Farmaco esempio");

    expect(descriptions()).toEqual([
      "Farmaco esempio",
      "Farmaco esempio antico",
    ]);
    expect(screen.getByText("2 farmaci")).toBeInTheDocument();

    await openStatusFilter(user);

    expect(
      await screen.findByRole("checkbox", { name: "Da ritirare (2)" }),
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Erogata (1)" }),
    ).not.toBeChecked();
  });

  it("shows closed rows after ticking Erogata", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await screen.findByText("Farmaco esempio");
    await openStatusFilter(user);

    await user.click(
      await screen.findByRole("checkbox", { name: "Erogata (1)" }),
    );

    expect(bodyRows()).toHaveLength(3);
    expect(screen.getByText("Farmaco erogato esempio")).toBeInTheDocument();
    expect(screen.getByText("3 farmaci")).toBeInTheDocument();
  });

  it("shows the empty text with Nessuno", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await screen.findByText("Farmaco esempio");
    await openStatusFilter(user);

    await user.click(await screen.findByRole("button", { name: "Nessuno" }));

    expect(
      screen.getByText("Nessun farmaco in questa vista."),
    ).toBeInTheDocument();
    expect(screen.getByText("0 farmaci")).toBeInTheDocument();
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

  it("sorts by header and shows the sort direction", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await screen.findByText("Farmaco esempio");
    expect(sortLabel("Data")).toBe("Sorted descending");

    await user.click(sortHeader("Data"));

    expect(sortLabel("Data")).toBe("Sorted ascending");
    expect(descriptions()).toEqual([
      "Farmaco esempio antico",
      "Farmaco esempio",
    ]);

    await user.click(sortHeader("NRE"));

    expect(sortLabel("NRE")).toBe("Sorted ascending");
    expect(sortLabel("Data")).toBe("Not sorted");
    expect(descriptions()).toEqual([
      "Farmaco esempio antico",
      "Farmaco esempio",
    ]);
  });

  it("links the PDF", async () => {
    render(<Harness />);
    await screen.findByText("Farmaco esempio");

    await userEvent.setup().click(
      within(at(bodyRows(), 0)).getByRole("button", {
        name: "Apri ricetta (PDF)",
      }),
    );
    expect(
      await screen.findByRole("dialog", { name: "Ricetta" }),
    ).toBeInTheDocument();
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

  it("opens the detail modal on row click", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await screen.findByText("Farmaco esempio");

    await user.click(within(at(bodyRows(), 0)).getByText("Farmaco esempio"));

    const dialog = await screen.findByRole("dialog", {
      name: "Farmaco esempio",
    });
    expect(within(dialog).getByText("Da ritirare")).toBeVisible();
    expect(
      within(dialog).getByRole("button", {
        name: "Copia NRE 0900A0000000002",
      }),
    ).toBeVisible();
    await user.click(
      within(dialog).getByRole("button", { name: "Apri ricetta (PDF)" }),
    );
    expect(
      await screen.findByRole("dialog", { name: "Ricetta" }),
    ).toBeInTheDocument();
  });

  it("opens the detail modal with Enter on a focused row", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await screen.findByText("Farmaco esempio");

    at(bodyRows(), 0).focus();
    await user.keyboard("{Enter}");

    expect(
      await screen.findByRole("dialog", { name: "Farmaco esempio" }),
    ).toBeInTheDocument();
  });

  it("does not open the detail modal from the NRE copy button", async () => {
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue(undefined);
    render(<Harness />);
    await screen.findByText("Farmaco esempio");

    await user.click(
      within(at(bodyRows(), 0)).getByRole("button", {
        name: "Copia NRE 0900A0000000002",
      }),
    );

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
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

      expect(row).toHaveAttribute("data-focused", "true");
      expect(row).toHaveFocus();
      expect(scroll).toHaveBeenCalled();
    } finally {
      vi.restoreAllMocks();
    }
  });

  it("shows and focuses a deep-linked row whose status is excluded", async () => {
    const user = userEvent.setup();
    stubScroll(vi.fn());
    window.location.hash = "#/farmaci/0900A0000000003";
    try {
      render(<Harness />);
      const row = (await screen.findByText("Farmaco erogato esempio")).closest(
        "tr",
      );

      expect(row).toHaveAttribute("data-focused", "true");
      expect(row).toHaveFocus();

      await openStatusFilter(user);

      expect(
        await screen.findByRole("checkbox", { name: "Erogata (1)" }),
      ).toBeChecked();
    } finally {
      vi.restoreAllMocks();
    }
  });
});
