import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { SessionBody } from "../api/client";
import { GoogleContext } from "../google/useGoogle";
import { SessionProvider } from "../session/SessionProvider";
import { SessionBar } from "./SessionBar";

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
    { portal: "fse", state: "expired", login: { phase: "idle" } },
  ],
};

const waitingSession: SessionBody = {
  gestioneReady: false,
  portals: [
    { portal: "cup", state: "logged_out", login: { phase: "waiting" } },
    { portal: "fse", state: "logged_out", login: { phase: "idle" } },
  ],
};

function failedSession(reason: string): SessionBody {
  return {
    gestioneReady: false,
    portals: [
      {
        portal: "cup",
        state: "logged_out",
        login: { phase: "failed", error: reason },
      },
      { portal: "fse", state: "logged_out", login: { phase: "idle" } },
    ],
  };
}

function renderBar(): void {
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
        <SessionBar />
      </SessionProvider>
    </GoogleContext>,
  );
}

describe("SessionBar", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    mocks.post.mockReset();
    mocks.post.mockResolvedValue({ response: { status: 202 } });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders portal states in Italian", async () => {
    mocks.get.mockResolvedValue({ data: activeSession });
    renderBar();

    expect(await screen.findByText("attivo")).toBeInTheDocument();
    expect(screen.getByText("scaduto")).toBeInTheDocument();
    expect(screen.getByText("disponibile")).toBeInTheDocument();
    expect(
      screen.getByText("scade ven 20 nov 2026, 09:30"),
    ).toBeInTheDocument();
  });

  it("renders the logged out state and unavailable appointments", async () => {
    mocks.get.mockResolvedValue({ data: failedSession("timeout") });
    renderBar();

    expect(
      await screen.findByText("non connesso (tempo scaduto)"),
    ).toBeInTheDocument();
    expect(screen.getByText("non disponibile")).toBeInTheDocument();
  });

  it("maps each failure reason to Italian", async () => {
    mocks.get.mockResolvedValue({ data: failedSession("window_closed") });
    renderBar();
    expect(
      await screen.findByText("non connesso (finestra chiusa)"),
    ).toBeInTheDocument();
  });

  it("maps interaction_required to Italian", async () => {
    mocks.get.mockResolvedValue({
      data: failedSession("interaction_required"),
    });
    renderBar();
    expect(
      await screen.findByText("non connesso (serve un nuovo accesso)"),
    ).toBeInTheDocument();
  });

  it("maps an unknown failure to a generic error", async () => {
    mocks.get.mockResolvedValue({ data: failedSession("boom") });
    renderBar();
    expect(
      await screen.findByText("non connesso (errore)"),
    ).toBeInTheDocument();
  });

  it("shows login in progress and disables Accedi while waiting", async () => {
    mocks.get.mockResolvedValue({ data: waitingSession });
    renderBar();

    expect(await screen.findByText("accesso in corso…")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Accedi" })).toBeDisabled();
  });

  it("calls POST /api/login and refreshes on Accedi", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: activeSession });
    renderBar();
    await screen.findByText("attivo");
    const before = mocks.get.mock.calls.length;

    await user.click(screen.getByRole("button", { name: "Accedi" }));

    expect(mocks.post).toHaveBeenCalledWith("/api/login");
    await waitFor(() => {
      expect(mocks.get.mock.calls.length).toBeGreaterThan(before);
    });
  });

  it("calls POST /api/logout on Esci", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: activeSession });
    renderBar();
    await screen.findByText("attivo");

    await user.click(screen.getByRole("button", { name: "Esci" }));

    expect(mocks.post).toHaveBeenCalledWith("/api/logout");
  });

  it("renders the Google connect control in the bar", async () => {
    mocks.get.mockResolvedValue({ data: activeSession });
    renderBar();

    expect(await screen.findByText("Google Calendar collegato")).toBeVisible();
  });

  it("shows an error when the server cannot be reached", async () => {
    mocks.get.mockRejectedValue(new Error("network"));
    renderBar();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Impossibile contattare il server",
    );
  });

  it("shows an error when the session cannot be read", async () => {
    mocks.get.mockResolvedValue({ error: { status: 500 } });
    renderBar();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Impossibile leggere lo stato della sessione",
    );
  });

  it("polls every 2 seconds while waiting", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mocks.get.mockResolvedValue({ data: waitingSession });
    renderBar();
    await screen.findByText("accesso in corso…");
    const before = mocks.get.mock.calls.length;

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2100);
    });

    expect(mocks.get.mock.calls.length).toBeGreaterThan(before);
  });

  it("polls every 15 seconds when idle", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mocks.get.mockResolvedValue({ data: activeSession });
    renderBar();
    await screen.findByText("attivo");
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
