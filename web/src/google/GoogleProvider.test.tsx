import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type MockInstance,
} from "vitest";

import type { GoogleStatusBody } from "../api/client";
import { GoogleConnect } from "./GoogleConnect";
import { GoogleProvider } from "./GoogleProvider";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  del: vi.fn(),
}));

vi.mock("../api/client", () => ({
  api: { GET: mocks.get, POST: mocks.post, DELETE: mocks.del },
}));

function renderConnect(): void {
  render(
    <GoogleProvider>
      <GoogleConnect />
    </GoogleProvider>,
  );
}

function mockStatus(status: GoogleStatusBody): void {
  mocks.get.mockResolvedValue({ data: status });
}

describe("GoogleConnect", () => {
  let open: MockInstance<typeof window.open>;

  beforeEach(() => {
    mocks.get.mockReset();
    mocks.post.mockReset();
    mocks.del.mockReset();
    mocks.post.mockResolvedValue({
      data: { authUrl: "https://accounts.example.com/auth" },
    });
    mocks.del.mockResolvedValue({ response: { status: 204 } });
    open = vi.spyOn(window, "open").mockReturnValue(null);
  });

  afterEach(() => {
    vi.useRealTimers();
    open.mockRestore();
  });

  it("shows the not configured text with the hint tooltip", async () => {
    mockStatus({ configured: false, connected: false });
    renderConnect();

    const text = await screen.findByText("Google Calendar non configurato");
    expect(text).toHaveAttribute(
      "title",
      "Crea ~/.config/fuck-toscana-salute/google-client.json (vedi README)",
    );
  });

  it("shows the connect button when configured but not connected", async () => {
    mockStatus({ configured: true, connected: false });
    renderConnect();

    expect(
      await screen.findByRole("button", { name: "Collega Google Calendar" }),
    ).toBeVisible();
  });

  it("shows the connected text with a disconnect button", async () => {
    mockStatus({ configured: true, connected: true });
    renderConnect();

    expect(await screen.findByText("Google Calendar collegato")).toBeVisible();
    expect(screen.getByRole("button", { name: "Scollega" })).toBeVisible();
  });

  it("shows an alert when the status cannot be read", async () => {
    mocks.get.mockRejectedValue(new Error("network"));
    renderConnect();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Impossibile contattare il server",
    );
  });

  it("POSTs connect and opens the auth window", async () => {
    const user = userEvent.setup();
    mockStatus({ configured: true, connected: false });
    renderConnect();

    await user.click(
      await screen.findByRole("button", { name: "Collega Google Calendar" }),
    );

    expect(mocks.post).toHaveBeenCalledWith("/api/google/connect");
    expect(open).toHaveBeenCalledWith(
      "https://accounts.example.com/auth",
      "_blank",
      "noopener",
    );
  });

  it("shows an alert and does not open a window when connect fails", async () => {
    const user = userEvent.setup();
    mockStatus({ configured: true, connected: false });
    mocks.post.mockResolvedValue({
      error: { detail: "google_not_configured" },
      response: { status: 409 },
    });
    renderConnect();

    await user.click(
      await screen.findByRole("button", { name: "Collega Google Calendar" }),
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Impossibile avviare il collegamento a Google Calendar",
    );
    expect(open).not.toHaveBeenCalled();
  });

  it("refreshes the status on window focus", async () => {
    mockStatus({ configured: true, connected: false });
    renderConnect();
    await screen.findByRole("button", { name: "Collega Google Calendar" });
    mockStatus({ configured: true, connected: true });

    act(() => {
      window.dispatchEvent(new Event("focus"));
    });

    expect(await screen.findByText("Google Calendar collegato")).toBeVisible();
  });

  it("polls every 3 seconds after connect until connected", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mockStatus({ configured: true, connected: false });
    renderConnect();
    const connect = await screen.findByRole("button", {
      name: "Collega Google Calendar",
    });
    fireEvent.click(connect);
    await waitFor(() => {
      expect(open).toHaveBeenCalled();
    });
    const before = mocks.get.mock.calls.length;
    mockStatus({ configured: true, connected: true });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3100);
    });

    expect(await screen.findByText("Google Calendar collegato")).toBeVisible();
    const afterConnected = mocks.get.mock.calls.length;
    expect(afterConnected).toBeGreaterThan(before);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(9000);
    });
    expect(mocks.get.mock.calls.length).toBe(afterConnected);
  });

  it("sends DELETE on Scollega and refreshes", async () => {
    const user = userEvent.setup();
    mockStatus({ configured: true, connected: true });
    renderConnect();
    await screen.findByText("Google Calendar collegato");
    const before = mocks.get.mock.calls.length;
    mockStatus({ configured: true, connected: false });

    await user.click(screen.getByRole("button", { name: "Scollega" }));

    expect(mocks.del).toHaveBeenCalledWith("/api/google");
    await waitFor(() => {
      expect(mocks.get.mock.calls.length).toBeGreaterThan(before);
    });
    expect(
      await screen.findByRole("button", { name: "Collega Google Calendar" }),
    ).toBeVisible();
  });
});
