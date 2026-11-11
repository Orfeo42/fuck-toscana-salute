import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { SessionBody } from "../api/client";
import { SessionProvider } from "../session/SessionProvider";
import { render } from "../test/render";
import { SessionNotice } from "./SessionNotice";

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
    { portal: "cup", state: "active", login: { phase: "idle" } },
    { portal: "fse", state: "active", login: { phase: "idle" } },
  ],
};

const noGestioneSession: SessionBody = {
  gestioneReady: false,
  portals: [
    { portal: "cup", state: "active", login: { phase: "idle" } },
    { portal: "fse", state: "active", login: { phase: "idle" } },
  ],
};

const loggedOutSession: SessionBody = {
  gestioneReady: false,
  portals: [
    { portal: "cup", state: "logged_out", login: { phase: "idle" } },
    { portal: "fse", state: "logged_out", login: { phase: "idle" } },
  ],
};

function renderNotice(): void {
  render(
    <SessionProvider>
      <SessionNotice />
    </SessionProvider>,
  );
}

describe("SessionNotice", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    mocks.post.mockReset();
  });

  it("renders nothing while the session is loading", () => {
    mocks.get.mockReturnValue(new Promise(() => undefined));
    renderNotice();

    expect(screen.queryByText(/Accedi/)).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows the login help when logged out", async () => {
    mocks.get.mockResolvedValue({ data: loggedOutSession });
    renderNotice();

    expect(
      await screen.findByText("Accedi per vedere i tuoi dati"),
    ).toBeInTheDocument();
    expect(screen.getByText(/SPID, poi Gestione Prenotazioni/)).toBeVisible();
  });

  it("warns when appointment dates are unavailable", async () => {
    mocks.get.mockResolvedValue({ data: noGestioneSession });
    renderNotice();

    expect(
      await screen.findByText(
        /Date e orari degli appuntamenti non disponibili/,
      ),
    ).toBeInTheDocument();
  });

  it("renders nothing when everything is available", async () => {
    mocks.get.mockResolvedValue({ data: activeSession });
    const { container } = render(
      <SessionProvider>
        <SessionNotice />
      </SessionProvider>,
    );

    await vi.waitFor(() => {
      expect(mocks.get).toHaveBeenCalled();
    });
    expect(container).not.toHaveTextContent("Accedi");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows an error when the server cannot be reached", async () => {
    mocks.get.mockRejectedValue(new Error("network"));
    renderNotice();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Impossibile contattare il server",
    );
  });

  it("shows an error when the session cannot be read", async () => {
    mocks.get.mockResolvedValue({ error: { status: 500 } });
    renderNotice();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Impossibile leggere lo stato della sessione",
    );
  });
});
