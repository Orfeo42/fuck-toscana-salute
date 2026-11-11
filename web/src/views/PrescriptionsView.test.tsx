import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { JSX } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { RowBody, SessionBody } from "../api/client";
import { SessionContext } from "../session/useSession";
import { PrescriptionsView } from "./PrescriptionsView";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
}));

vi.mock("../api/client", () => ({
  api: { GET: mocks.get },
  prescriptionPdfUrl: (ref: string) => `/api/documents/prescription?ref=${ref}`,
}));

const bookedVisit: RowBody = {
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

const closed: RowBody = {
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

const rows = [bookedVisit, drug, closed, cupOnly];

function sessionWith(cupState: string): SessionBody {
  return {
    gestioneReady: true,
    portals: [
      {
        portal: "cup",
        state: cupState as "active",
        login: { phase: "idle" },
      },
      { portal: "fse", state: "active", login: { phase: "idle" } },
    ],
  };
}

function Harness({ session }: { session: SessionBody }): JSX.Element {
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
      <PrescriptionsView />
    </SessionContext.Provider>
  );
}

function renderView(session = sessionWith("active")) {
  return render(<Harness session={session} />);
}

function cardAt(index: number): HTMLElement {
  const card = screen.getAllByRole("listitem")[index];
  if (card === undefined) {
    throw new Error(`no card at ${String(index)}`);
  }
  return card;
}

function titles(): string[] {
  return screen
    .queryAllByRole("heading", { level: 3 })
    .map((h) => h.textContent);
}

describe("PrescriptionsView", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    mocks.get.mockResolvedValue({ data: { prescriptions: rows } });
  });

  it("shows the loading state", () => {
    mocks.get.mockReturnValue(new Promise(() => undefined));
    renderView();

    expect(screen.getByText("Carico le ricette…")).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Ricette" })).toHaveAttribute(
      "aria-busy",
      "true",
    );
  });

  it("shows the open view by default with counts, sorted by date", async () => {
    renderView();

    expect(await screen.findByText("Visita esempio")).toBeInTheDocument();
    expect(titles()).toEqual([
      "Ecografia esempio, Prelievo esempio",
      "Visita esempio",
      "Farmaco esempio",
    ]);
    expect(screen.getByRole("radio", { name: "Aperte (3)" })).toBeChecked();
    expect(
      screen.getByRole("radio", { name: "Da prenotare (2)" }),
    ).toBeVisible();
    expect(screen.getByRole("radio", { name: "Tutte (4)" })).toBeVisible();
    expect(screen.getByText("3 ricette")).toBeInTheDocument();
  });

  it("switches to Da prenotare", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");

    await user.click(screen.getByRole("radio", { name: /Da prenotare/ }));

    expect(titles()).toEqual([
      "Ecografia esempio, Prelievo esempio",
      "Visita esempio",
    ]);
  });

  it("switches to Tutte", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");

    await user.click(screen.getByRole("radio", { name: /Tutte/ }));

    expect(titles()).toHaveLength(4);
    expect(screen.getByText("Esame chiuso esempio")).toBeInTheDocument();
  });

  it("narrows the list with the search box", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");

    await user.type(
      screen.getByRole("searchbox", { name: "Cerca" }),
      "FARMACO",
    );

    expect(titles()).toEqual(["Farmaco esempio"]);
    expect(screen.getByText("1 ricetta")).toBeInTheDocument();
  });

  it("finds a row by NRE", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");

    await user.type(screen.getByRole("searchbox"), "0900A0000000001");

    expect(titles()).toEqual(["Visita esempio"]);
  });

  it("filters by kind", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");

    await user.selectOptions(screen.getByLabelText("Tipo"), "Farmaci");
    expect(titles()).toEqual(["Farmaco esempio"]);

    await user.selectOptions(screen.getByLabelText("Tipo"), "Visite ed esami");
    expect(titles()).toEqual([
      "Ecografia esempio, Prelievo esempio",
      "Visita esempio",
    ]);
  });

  it("copies the NRE and shows feedback", async () => {
    const user = userEvent.setup();
    const writeText = vi.spyOn(navigator.clipboard, "writeText");
    writeText.mockResolvedValue(undefined);
    renderView();
    await screen.findByText("Farmaco esempio");
    const card = cardAt(2);

    await user.click(within(card).getByRole("button", { name: "Copia" }));

    expect(writeText).toHaveBeenCalledWith("0900A0000000002");
    expect(await within(card).findByText("Copiato")).toBeInTheDocument();
  });

  it("shows a failure when the copy is rejected", async () => {
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, "writeText").mockRejectedValue(
      new Error("denied"),
    );
    renderView();
    await screen.findByText("Farmaco esempio");
    const card = cardAt(2);

    await user.click(within(card).getByRole("button", { name: "Copia" }));

    expect(
      await within(card).findByText("Copia non riuscita"),
    ).toBeInTheDocument();
  });

  it("shows Prenota sul CUP only on rows with cup", async () => {
    renderView();
    await screen.findByText("Visita esempio");
    const cupCard = cardAt(0);
    const bookedCard = cardAt(1);
    const drugCard = cardAt(2);

    const link = within(cupCard).getByRole("link", { name: "Prenota sul CUP" });
    expect(link).toHaveAttribute("href", "https://prenota.sanita.toscana.it/");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
    expect(
      within(bookedCard).getByRole("link", { name: "Prenota sul CUP" }),
    ).toBeVisible();
    expect(
      within(drugCard).queryByRole("link", { name: "Prenota sul CUP" }),
    ).not.toBeInTheDocument();
  });

  it("shows the PDF link only when a document exists", async () => {
    renderView();
    await screen.findByText("Visita esempio");
    const cupCard = cardAt(0);
    const bookedCard = cardAt(1);

    expect(
      within(bookedCard).getByRole("link", { name: "Apri ricetta (PDF)" }),
    ).toHaveAttribute("href", "/api/documents/prescription?ref=ref-1");
    expect(
      within(cupCard).queryByRole("link", { name: "Apri ricetta (PDF)" }),
    ).not.toBeInTheDocument();
  });

  it("uses the to-book label for a cup only row", async () => {
    renderView();
    await screen.findByText("Ecografia esempio, Prelievo esempio");
    const cupCard = cardAt(0);

    expect(within(cupCard).getByText("Da prenotare")).toBeInTheDocument();
    expect(within(cupCard).getByText("Visita/esame")).toBeInTheDocument();
  });

  it("shows the login message on 401", async () => {
    mocks.get.mockResolvedValue({
      error: { detail: "no session" },
      response: { status: 401 },
    });
    renderView();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Accedi per vedere le ricette.",
    );
  });

  it("shows the portal message on 502", async () => {
    mocks.get.mockResolvedValue({
      error: { detail: "bad gateway" },
      response: { status: 502 },
    });
    renderView();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Il portale regionale non risponde. Riprova tra poco.",
    );
  });

  it("shows the portal message when the request throws", async () => {
    mocks.get.mockRejectedValue(new Error("network"));
    renderView();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Il portale regionale non risponde.",
    );
  });

  it("refetches on Aggiorna", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Visita esempio");
    expect(mocks.get).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "Aggiorna" }));

    await waitFor(() => {
      expect(mocks.get).toHaveBeenCalledTimes(2);
    });
    expect(await screen.findByText("Visita esempio")).toBeInTheDocument();
  });

  it("refetches when a portal state changes", async () => {
    const { rerender } = renderView(sessionWith("active"));
    await screen.findByText("Visita esempio");
    expect(mocks.get).toHaveBeenCalledTimes(1);

    rerender(<Harness session={sessionWith("active")} />);
    expect(mocks.get).toHaveBeenCalledTimes(1);

    rerender(<Harness session={sessionWith("expired")} />);
    await waitFor(() => {
      expect(mocks.get).toHaveBeenCalledTimes(2);
    });
  });

  it("treats a null list as empty", async () => {
    mocks.get.mockResolvedValue({ data: { prescriptions: null } });
    renderView();

    expect(
      await screen.findByText("Nessuna ricetta in questa vista."),
    ).toBeInTheDocument();
    expect(screen.getByText("0 ricette")).toBeInTheDocument();
  });

  it("treats null cup items as empty", async () => {
    mocks.get.mockResolvedValue({
      data: {
        prescriptions: [
          {
            nre: "0900A0000000005",
            cup: {
              nre: "0900A0000000005",
              kind: "visit",
              status: 3,
              items: null,
            },
          },
        ],
      },
    });
    renderView();

    expect(await screen.findByText("Prescrizione")).toBeInTheDocument();
    expect(screen.getByText("0900A0000000005")).toBeInTheDocument();
  });

  it("shows the empty state for a filter with no rows", async () => {
    mocks.get.mockResolvedValue({ data: { prescriptions: [closed] } });
    renderView();

    expect(
      await screen.findByText("Nessuna ricetta in questa vista."),
    ).toBeInTheDocument();
  });
});
