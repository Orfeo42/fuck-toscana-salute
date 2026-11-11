import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { SessionBody } from "../api/client";
import { GoogleContext } from "../google/useGoogle";
import { SessionProvider } from "../session/SessionProvider";
import { render } from "../test/render";
import { SessionMenu } from "./SessionMenu";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}));

vi.mock("../api/client", () => ({
  api: { GET: mocks.get, POST: mocks.post },
}));

const activeSession: SessionBody = {
  gestioneReady: true,
  portals: [
    {
      portal: "cup",
      state: "active",
      login: { phase: "idle" },
      refreshExpiresAt: "2026-11-20T08:30:00Z",
    },
    { portal: "fse", state: "active", login: { phase: "idle" } },
  ],
};

const degradedSession: SessionBody = {
  gestioneReady: false,
  portals: [
    {
      portal: "cup",
      state: "active",
      login: { phase: "idle" },
      refreshExpiresAt: "2026-11-20T08:30:00Z",
    },
    { portal: "fse", state: "expired", login: { phase: "idle" } },
  ],
};

const loggedOutSession: SessionBody = {
  gestioneReady: false,
  portals: [
    { portal: "cup", state: "logged_out", login: { phase: "idle" } },
    { portal: "fse", state: "logged_out", login: { phase: "idle" } },
  ],
};

const waitingLoggedOutSession: SessionBody = {
  gestioneReady: false,
  portals: [
    { portal: "cup", state: "logged_out", login: { phase: "waiting" } },
    { portal: "fse", state: "logged_out", login: { phase: "idle" } },
  ],
};

const waitingLoggedInSession: SessionBody = {
  gestioneReady: false,
  portals: [
    { portal: "cup", state: "active", login: { phase: "idle" } },
    { portal: "fse", state: "expired", login: { phase: "waiting" } },
  ],
};

function failedSession(reason: string): SessionBody {
  return {
    gestioneReady: false,
    portals: [
      { portal: "cup", state: "active", login: { phase: "idle" } },
      {
        portal: "fse",
        state: "logged_out",
        login: { phase: "failed", error: reason },
      },
    ],
  };
}

function renderMenu(): void {
  render(
    <GoogleContext
      value={{
        status: { configured: true, connected: true },
        loading: false,
        error: undefined,
        refresh: vi.fn(),
        connect: vi.fn(),
        disconnect: vi.fn(),
      }}
    >
      <SessionProvider>
        <SessionMenu />
      </SessionProvider>
    </GoogleContext>,
  );
}

async function openMenu(): Promise<void> {
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "Account" }));
}

describe("SessionMenu", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    mocks.post.mockReset();
    mocks.post.mockResolvedValue({ response: { status: 202 } });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows a loader while the session is loading", () => {
    mocks.get.mockReturnValue(new Promise(() => undefined));
    renderMenu();

    expect(screen.getByLabelText("Caricamento sessione")).toBeInTheDocument();
  });

  it("shows only Accedi when logged out", async () => {
    mocks.get.mockResolvedValue({ data: loggedOutSession });
    renderMenu();

    expect(await screen.findByRole("button", { name: "Accedi" })).toBeEnabled();
    expect(
      screen.queryByRole("button", { name: "Account" }),
    ).not.toBeInTheDocument();
  });

  it("calls POST /api/login and refreshes on Accedi", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: loggedOutSession });
    renderMenu();
    const button = await screen.findByRole("button", { name: "Accedi" });
    const before = mocks.get.mock.calls.length;

    await user.click(button);

    expect(mocks.post).toHaveBeenCalledWith("/api/login");
    await waitFor(() => {
      expect(mocks.get.mock.calls.length).toBeGreaterThan(before);
    });
  });

  it("disables Accedi while a login is waiting", async () => {
    mocks.get.mockResolvedValue({ data: waitingLoggedOutSession });
    renderMenu();

    expect(
      await screen.findByRole("button", { name: "Accedi" }),
    ).toBeDisabled();
  });

  it("renders portal states in Italian", async () => {
    mocks.get.mockResolvedValue({ data: degradedSession });
    renderMenu();
    await openMenu();

    expect(screen.getByText("attivo")).toBeInTheDocument();
    expect(screen.getByText("scaduto")).toBeInTheDocument();
    expect(screen.getByText("non disponibile")).toBeInTheDocument();
    expect(
      screen.getByText("scade ven 20 nov 2026, 09:30"),
    ).toBeInTheDocument();
  });

  it("shows appointment dates as available", async () => {
    mocks.get.mockResolvedValue({ data: activeSession });
    renderMenu();
    await openMenu();

    expect(screen.getByText("disponibile")).toBeInTheDocument();
  });

  it("maps a timeout failure to Italian", async () => {
    mocks.get.mockResolvedValue({ data: failedSession("timeout") });
    renderMenu();
    await openMenu();

    expect(
      screen.getByText("non connesso (tempo scaduto)"),
    ).toBeInTheDocument();
  });

  it("maps window_closed to Italian", async () => {
    mocks.get.mockResolvedValue({ data: failedSession("window_closed") });
    renderMenu();
    await openMenu();

    expect(
      screen.getByText("non connesso (finestra chiusa)"),
    ).toBeInTheDocument();
  });

  it("maps interaction_required to Italian", async () => {
    mocks.get.mockResolvedValue({
      data: failedSession("interaction_required"),
    });
    renderMenu();
    await openMenu();

    expect(
      screen.getByText("non connesso (serve un nuovo accesso)"),
    ).toBeInTheDocument();
  });

  it("maps an unknown failure to a generic error", async () => {
    mocks.get.mockResolvedValue({ data: failedSession("boom") });
    renderMenu();
    await openMenu();

    expect(screen.getByText("non connesso (errore)")).toBeInTheDocument();
  });

  it("shows login in progress and disables Accedi di nuovo while waiting", async () => {
    mocks.get.mockResolvedValue({ data: waitingLoggedInSession });
    renderMenu();
    await openMenu();

    expect(screen.getByText("accesso in corso…")).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Accedi di nuovo" }),
    ).toBeDisabled();
  });

  it("renders the Google connect control in the menu", async () => {
    mocks.get.mockResolvedValue({ data: activeSession });
    renderMenu();
    await openMenu();

    expect(
      screen.getByRole("menuitem", { name: "Scollega Google Calendar" }),
    ).toBeVisible();
  });

  it("hides Accedi di nuovo when everything is fine", async () => {
    mocks.get.mockResolvedValue({ data: activeSession });
    renderMenu();
    await openMenu();

    expect(
      screen.queryByRole("menuitem", { name: "Accedi di nuovo" }),
    ).not.toBeInTheDocument();
  });

  it("calls POST /api/login from Accedi di nuovo when degraded", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: degradedSession });
    renderMenu();
    await openMenu();

    await user.click(screen.getByRole("menuitem", { name: "Accedi di nuovo" }));

    expect(mocks.post).toHaveBeenCalledWith("/api/login");
  });

  it("calls POST /api/logout on Esci", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: activeSession });
    renderMenu();
    await openMenu();

    await user.click(screen.getByRole("menuitem", { name: "Esci" }));

    expect(mocks.post).toHaveBeenCalledWith("/api/logout");
  });

  it("polls every 2 seconds while waiting", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mocks.get.mockResolvedValue({ data: waitingLoggedOutSession });
    renderMenu();
    await screen.findByRole("button", { name: "Accedi" });
    const before = mocks.get.mock.calls.length;

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2100);
    });

    expect(mocks.get.mock.calls.length).toBeGreaterThan(before);
  });

  it("polls every 15 seconds when idle", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mocks.get.mockResolvedValue({ data: activeSession });
    renderMenu();
    await screen.findByRole("button", { name: "Account" });
    const before = mocks.get.mock.calls.length;

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2100);
    });
    expect(mocks.get.mock.calls.length).toBe(before);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(13000);
    });
    expect(mocks.get.mock.calls.length).toBeGreaterThan(before);
  });
});
