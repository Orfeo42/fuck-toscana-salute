import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { SessionBody } from "../api/client";
import { SessionProvider } from "../session/SessionProvider";
import { render } from "../test/render";
import { LoginViewer } from "./LoginViewer";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}));

vi.mock("../api/client", () => ({
  api: { GET: mocks.get, POST: mocks.post },
}));

const waitingSession: SessionBody = {
  gestioneReady: false,
  loginMode: "window",
  viewer: true,
  portals: [
    { portal: "cup", state: "logged_out", login: { phase: "waiting" } },
    { portal: "fse", state: "logged_out", login: { phase: "idle" } },
  ],
};

const idleSession: SessionBody = {
  ...waitingSession,
  portals: [
    { portal: "cup", state: "logged_out", login: { phase: "idle" } },
    { portal: "fse", state: "logged_out", login: { phase: "idle" } },
  ],
};

function renderViewer(): void {
  render(
    <SessionProvider>
      <LoginViewer />
    </SessionProvider>,
  );
}

describe("LoginViewer", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    mocks.post.mockReset();
  });

  it("shows the remote browser while a portal login is waiting", async () => {
    mocks.get.mockResolvedValue({ data: waitingSession });
    renderViewer();

    expect(
      await screen.findByRole("dialog", { name: "Accesso ai portali" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Accedi con SPID\/CIE/)).toBeInTheDocument();
    expect(screen.getByTitle("Browser di accesso")).toHaveAttribute(
      "src",
      "/viewer/vnc.html?autoconnect=true&resize=remote&reconnect=true&path=viewer/websockify",
    );
  });

  it("stays hidden when the viewer is not available", async () => {
    mocks.get.mockResolvedValue({
      data: { ...waitingSession, viewer: false },
    });
    renderViewer();

    await vi.waitFor(() => {
      expect(mocks.get).toHaveBeenCalled();
    });
    expect(screen.queryByTitle("Browser di accesso")).not.toBeInTheDocument();
  });

  it("stays hidden when no login is waiting", async () => {
    mocks.get.mockResolvedValue({ data: idleSession });
    renderViewer();

    await vi.waitFor(() => {
      expect(mocks.get).toHaveBeenCalled();
    });
    expect(screen.queryByTitle("Browser di accesso")).not.toBeInTheDocument();
  });

  it("hides on the close button without cancelling the login", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: waitingSession });
    renderViewer();

    await screen.findByTitle("Browser di accesso");
    await user.click(screen.getByRole("button", { name: "Chiudi" }));

    await vi.waitFor(() => {
      expect(screen.queryByTitle("Browser di accesso")).not.toBeInTheDocument();
    });
    expect(mocks.post).not.toHaveBeenCalled();
  });
});
