import {
  act,
  render as rtlRender,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type {
  DrugRowBody,
  DrugRowResponseBody,
  DrugsQueryBody,
  DrugsQueryRequest,
} from "../api/client";
import { TestProviders } from "../test/TestProviders";
import { WithSession } from "../test/WithSession";
import { DrugsView } from "./DrugsView";

const mocks = vi.hoisted(() => ({
  post: vi.fn<
    (path: string, init: { body: DrugsQueryRequest }) => Promise<unknown>
  >(),
  startRefresh: vi.fn(),
}));

vi.mock("../data/refresh", () => ({ startRefresh: mocks.startRefresh }));

vi.mock("../lib/pdf", () => ({
  loadPdf: () => ({
    promise: new Promise<never>(() => undefined),
    destroy: () => Promise.resolve(),
  }),
}));

vi.mock("../api/client", () => ({
  api: { POST: mocks.post },
  prescriptionPdfUrl: (ref: string) => `/api/documents/prescription?ref=${ref}`,
}));

function drug(suffix: string, title: string, label: string): DrugRowBody {
  return {
    nre: `0900A00000000${suffix}`,
    title,
    date: "2026-11-02T09:00:00Z",
    documentRef: `ref-${suffix}`,
    status: { key: "taken_in_charge", label, tone: "tobook" },
  };
}

const first = drug("01", "Farmaco Esempio", "Da ritirare");
const second = drug("02", "Farmaco Esempio Antico", "Da ritirare");

function page(overrides: Partial<DrugsQueryBody> = {}): DrugsQueryBody {
  return {
    rows: [first, second],
    total: 2,
    page: 1,
    pageSize: 50,
    statuses: [
      {
        key: "taken_in_charge",
        label: "Da ritirare",
        tone: "tobook",
        count: 2,
        excluded: false,
      },
      {
        key: "done",
        label: "Erogata",
        tone: "done",
        count: 1,
        excluded: true,
      },
    ],
    excludedStatuses: ["done"],
    search: "",
    refreshing: false,
    updated: { at: null, label: "Aggiornato il 09/01 09:05" },
    issue: "none",
    ...overrides,
  };
}

function rowAnswer(row: DrugRowBody): DrugRowResponseBody {
  return { row, refreshing: false };
}

function queryBodies(): DrugsQueryRequest[] {
  return mocks.post.mock.calls
    .filter((call) => call[0] === "/api/drugs/query")
    .map((call) => call[1].body);
}

function answerQuery(body: DrugsQueryBody): void {
  mocks.post.mockImplementation((path: string) => {
    if (path === "/api/drugs/row") {
      return Promise.resolve({
        data: rowAnswer(first),
        response: { status: 200 },
      });
    }
    return Promise.resolve({ data: body, response: { status: 200 } });
  });
}

function failQuery(status: number): void {
  mocks.post.mockResolvedValue({
    data: undefined,
    error: { detail: "x" },
    response: { status },
  });
}

function renderView(): void {
  rtlRender(
    <WithSession>
      <DrugsView />
    </WithSession>,
    { wrapper: TestProviders },
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

function sortHeader(name: string): HTMLElement {
  return screen.getByRole("button", { name: new RegExp(`^${name}`) });
}

async function openStatusFilter(
  user: ReturnType<typeof userEvent.setup>,
): Promise<void> {
  await user.click(within(sortHeader("Stato")).getByRole("button"));
}

describe("DrugsView", () => {
  beforeEach(() => {
    mocks.post.mockReset();
    mocks.startRefresh.mockReset();
    mocks.startRefresh.mockResolvedValue(true);
    window.location.hash = "";
    answerQuery(page());
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("shows the loading state", () => {
    mocks.post.mockReturnValue(new Promise(() => undefined));
    renderView();

    expect(screen.getByText("Carico i farmaci…")).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Farmaci" })).toHaveAttribute(
      "aria-busy",
      "true",
    );
  });

  it("renders rows, status badge and freshness from the response", async () => {
    renderView();
    await screen.findByText("Farmaco Esempio");

    const headers = [...screen.getByRole("table").querySelectorAll("th")].map(
      (h) => h.textContent,
    );
    expect(headers).toEqual(["Data", "Farmaco", "Stato", "NRE", "Azioni"]);
    expect(bodyRows()).toHaveLength(2);
    expect(screen.getAllByText("Da ritirare")[0]).toBeVisible();
    expect(screen.getByText("0900A0000000001")).toBeVisible();
    expect(screen.getByText("2 farmaci")).toBeInTheDocument();
    expect(screen.getByText("Aggiornato il 09/01 09:05")).toBeVisible();
  });

  it("sends the default request with no excluded statuses", async () => {
    renderView();
    await screen.findByText("Farmaco Esempio");

    expect(queryBodies()[0]).toEqual({
      search: "",
      sort: "date",
      dir: "desc",
      page: 1,
      pageSize: 50,
    });
  });

  it("sends the chosen excluded statuses and resets the page", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Farmaco Esempio");
    await openStatusFilter(user);

    await user.click(
      await screen.findByRole("checkbox", { name: "Erogata (1)" }),
    );

    await waitFor(() => {
      expect(queryBodies().at(-1)).toMatchObject({
        excludeStatuses: [],
        page: 1,
      });
    });
  });

  it("sends Nessuno as every status key", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Farmaco Esempio");
    await openStatusFilter(user);

    await user.click(await screen.findByRole("button", { name: "Nessuno" }));

    await waitFor(() => {
      expect(queryBodies().at(-1)).toMatchObject({
        excludeStatuses: ["taken_in_charge", "done"],
      });
    });
  });

  it("sends the search text", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Farmaco Esempio");

    await user.type(screen.getByRole("searchbox", { name: "Cerca" }), "antico");

    await waitFor(() => {
      expect(queryBodies().at(-1)).toMatchObject({ search: "antico", page: 1 });
    });
  });

  it("sends the sort key and direction", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Farmaco Esempio");

    await user.click(sortHeader("NRE"));

    await waitFor(() => {
      expect(queryBodies().at(-1)).toMatchObject({
        sort: "nre",
        dir: "asc",
        page: 1,
      });
    });
  });

  it("sends the page and page size from the pager", async () => {
    const user = userEvent.setup();
    answerQuery(page({ total: 120 }));
    renderView();
    await screen.findByText("Farmaco Esempio");

    await user.click(screen.getByRole("button", { name: "2" }));

    await waitFor(() => {
      expect(queryBodies().at(-1)).toMatchObject({ page: 2, pageSize: 50 });
    });
  });

  it("shows the empty text", async () => {
    answerQuery(page({ rows: [], total: 0 }));
    renderView();

    expect(
      await screen.findByText("Nessun farmaco in questa vista."),
    ).toBeInTheDocument();
    expect(screen.getByText("0 farmaci")).toBeInTheDocument();
  });

  it("shows the login message on 401", async () => {
    failQuery(401);
    renderView();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Accedi per vedere i farmaci.",
    );
  });

  it("shows the portal message on a server error", async () => {
    failQuery(502);
    renderView();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Il portale regionale non risponde. Riprova tra poco.",
    );
  });

  it("shows the login message from the response issue", async () => {
    answerQuery(page({ issue: "login_required" }));
    renderView();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Accedi per vedere i farmaci.",
    );
    expect(screen.getByText("Farmaco Esempio")).toBeInTheDocument();
  });

  it("shows the portal message from the response issue", async () => {
    answerQuery(page({ issue: "unavailable" }));
    renderView();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Il portale regionale non risponde. Riprova tra poco.",
    );
  });

  it("starts a lists refresh then re-queries on Aggiorna", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Farmaco Esempio");
    expect(queryBodies()).toHaveLength(1);

    await user.click(screen.getByRole("button", { name: "Aggiorna" }));

    expect(mocks.startRefresh).toHaveBeenCalledWith({ lists: true });
    await waitFor(() => {
      expect(queryBodies()).toHaveLength(2);
    });
  });

  it("shows a notification and does not re-query when the refresh fails", async () => {
    const user = userEvent.setup();
    mocks.startRefresh.mockResolvedValue(false);
    renderView();
    await screen.findByText("Farmaco Esempio");

    await user.click(screen.getByRole("button", { name: "Aggiorna" }));

    expect(await screen.findByText("Aggiornamento non riuscito")).toBeVisible();
    expect(queryBodies()).toHaveLength(1);
  });

  it("polls while the response says refreshing", async () => {
    vi.useFakeTimers();
    mocks.post.mockResolvedValueOnce({
      data: page({ refreshing: true }),
      response: { status: 200 },
    });
    mocks.post.mockResolvedValue({
      data: page({ refreshing: false }),
      response: { status: 200 },
    });
    renderView();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(queryBodies()).toHaveLength(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000);
    });
    expect(queryBodies()).toHaveLength(2);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(6000);
    });
    expect(queryBodies()).toHaveLength(2);
  });

  it("opens the drawer on row click and renders the row", async () => {
    const user = userEvent.setup();
    renderView();
    await user.click(await screen.findByText("Farmaco Esempio Antico"));

    const dialog = await screen.findByRole("dialog", {
      name: "Farmaco Esempio",
    });
    expect(within(dialog).getByText("Da ritirare")).toBeVisible();
    expect(mocks.post).toHaveBeenCalledWith("/api/drugs/row", {
      body: { nre: "0900A0000000002" },
      signal: expect.any(AbortSignal) as AbortSignal,
    });
  });

  it("opens the drawer with Enter on a focused row", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Farmaco Esempio");

    at(bodyRows(), 0).focus();
    await user.keyboard("{Enter}");

    expect(await screen.findByRole("dialog")).toBeInTheDocument();
  });

  it("does not open the drawer from the NRE copy button", async () => {
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue(undefined);
    renderView();
    await screen.findByText("Farmaco Esempio");

    await user.click(
      screen.getByRole("button", { name: "Copia NRE 0900A0000000001" }),
    );

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("opens the ricetta PDF from the row", async () => {
    const user = userEvent.setup();
    renderView();
    await screen.findByText("Farmaco Esempio");

    await user.click(
      within(at(bodyRows(), 0)).getByRole("button", {
        name: "Apri ricetta (PDF)",
      }),
    );

    expect(
      await screen.findByRole("dialog", { name: "Ricetta" }),
    ).toBeInTheDocument();
  });

  it("sends the deep-linked NRE as focus once then follows the returned page", async () => {
    window.location.hash = "#/farmaci/0900A0000000002";
    mocks.post.mockResolvedValueOnce({
      data: page({
        page: 2,
        excludedStatuses: [],
        search: "",
        focus: { nre: "0900A0000000002", found: true },
      }),
      response: { status: 200 },
    });
    mocks.post.mockResolvedValue({
      data: page({ page: 2, excludedStatuses: [] }),
      response: { status: 200 },
    });
    renderView();
    const row = (await screen.findByText("Farmaco Esempio Antico")).closest(
      "tr",
    );

    expect(row).toHaveAttribute("data-focused", "true");
    await waitFor(() => {
      expect(queryBodies()).toHaveLength(2);
    });
    expect(queryBodies()[0]).toMatchObject({ focus: "0900A0000000002" });
    expect(queryBodies()[1]).toMatchObject({
      page: 2,
      excludeStatuses: [],
      search: "",
    });
    expect(queryBodies()[1]).not.toHaveProperty("focus");
  });
});
