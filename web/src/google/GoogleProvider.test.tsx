import { MantineProvider, Menu, Modal } from "@mantine/core";
import {
  act,
  render as rtlRender,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import type { JSX } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { GoogleStatusBody } from "../api/client";
import { render } from "../test/render";
import { theme } from "../theme";
import { GoogleConnect } from "./GoogleConnect";
import { GoogleProvider } from "./GoogleProvider";
import { useGoogle } from "./useGoogle";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  del: vi.fn(),
  loadIdentity: vi.fn(),
  createRequester: vi.fn(),
  requester: vi.fn(),
}));

vi.mock("../api/client", () => ({
  api: { GET: mocks.get, POST: mocks.post, DELETE: mocks.del },
}));

vi.mock("../lib/googleSignIn", () => ({
  loadGoogleIdentity: mocks.loadIdentity,
  createCodeRequester: mocks.createRequester,
}));

const clientId = "123456789012-abc.apps.googleusercontent.com";

function renderConnect(): void {
  render(
    <GoogleProvider>
      <Menu opened>
        <Menu.Target>
          <button type="button">t</button>
        </Menu.Target>
        <Menu.Dropdown>
          <GoogleConnect />
        </Menu.Dropdown>
      </Menu>
    </GoogleProvider>,
  );
}

function SettingsOpener(): JSX.Element {
  const { openSettings } = useGoogle();
  return (
    <button type="button" onClick={openSettings}>
      Apri impostazioni
    </button>
  );
}

function LateModal(): JSX.Element {
  const [open, setOpen] = useState(false);
  if (!open) {
    return (
      <button
        type="button"
        onClick={() => {
          setOpen(true);
        }}
      >
        Apri sotto
      </button>
    );
  }
  return (
    <Modal opened onClose={() => undefined} title="Sotto">
      <SettingsOpener />
    </Modal>
  );
}

function status(configured: boolean, connected: boolean): GoogleStatusBody {
  return { configured, connected, clientId, drive: false };
}

function mockStatus(next: GoogleStatusBody): void {
  mocks.get.mockResolvedValue({ data: next });
}

async function clickSignIn(): Promise<void> {
  const user = userEvent.setup();
  await waitFor(() => {
    expect(mocks.createRequester).toHaveBeenCalled();
  });
  await user.click(
    await screen.findByRole("menuitem", { name: "Accedi con Google" }),
  );
}

describe("GoogleProvider", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    mocks.post.mockReset();
    mocks.del.mockReset();
    mocks.loadIdentity.mockReset();
    mocks.createRequester.mockReset();
    mocks.requester.mockReset();
    mocks.loadIdentity.mockResolvedValue({ initCodeClient: vi.fn() });
    mocks.createRequester.mockReturnValue(mocks.requester);
    mocks.requester.mockResolvedValue("fake-code");
    mocks.post.mockResolvedValue({ response: { status: 204 } });
    mocks.del.mockResolvedValue({ response: { status: 204 } });
  });

  it("opens the settings modal from the configure button", async () => {
    const user = userEvent.setup();
    mocks.get.mockImplementation((path: string) =>
      Promise.resolve(
        path === "/api/google/settings"
          ? {
              data: {
                clientId: "",
                calendarId: "primary",
                driveFolderId: "",
                apiKey: "",
                hasSecret: false,
                javascriptOrigin: "http://localhost:8800",
              },
            }
          : { data: { ...status(false, false), clientId: "" } },
      ),
    );
    renderConnect();

    await user.click(await screen.findByRole("button", { name: "Configura" }));

    expect(
      await screen.findByRole("dialog", { name: "Configura Google" }),
    ).toBeVisible();
    expect(mocks.loadIdentity).not.toHaveBeenCalled();
  });

  it("stacks the settings modal above a modal that opened it", async () => {
    const user = userEvent.setup();
    mocks.get.mockImplementation((path: string) =>
      Promise.resolve(
        path === "/api/google/settings"
          ? {
              data: {
                clientId: "",
                calendarId: "primary",
                driveFolderId: "",
                apiKey: "",
                hasSecret: false,
                javascriptOrigin: "http://localhost:8800",
              },
            }
          : { data: { ...status(false, false), clientId: "" } },
      ),
    );
    rtlRender(
      <MantineProvider theme={theme}>
        <GoogleProvider>
          <LateModal />
        </GoogleProvider>
      </MantineProvider>,
    );
    await user.click(await screen.findByRole("button", { name: "Apri sotto" }));

    expect(
      screen.queryByRole("dialog", { name: "Configura Google" }),
    ).toBeNull();
    await user.click(
      await screen.findByRole("button", { name: "Apri impostazioni" }),
    );

    await screen.findByRole("dialog", { name: "Configura Google" });
    expect(screen.getAllByRole("dialog").at(-1)).toHaveAccessibleName(
      "Configura Google",
    );
  });

  it("prepares the code requester when configured", async () => {
    mockStatus(status(true, false));
    renderConnect();

    await waitFor(() => {
      expect(mocks.createRequester).toHaveBeenCalledWith(
        { initCodeClient: expect.any(Function) as unknown },
        clientId,
      );
    });
  });

  it("shows an alert when the identity script cannot load", async () => {
    mockStatus(status(true, false));
    mocks.loadIdentity.mockRejectedValue(new Error("blocked"));
    renderConnect();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Impossibile caricare l'accesso Google",
    );
  });

  it("shows the connected text with a disconnect button", async () => {
    mockStatus(status(true, true));
    renderConnect();

    expect(await screen.findByText("Collegato")).toBeVisible();
    expect(
      screen.getByRole("menuitem", { name: "Scollega Google" }),
    ).toBeVisible();
  });

  it("shows an alert when the status cannot be read", async () => {
    mocks.get.mockRejectedValue(new Error("network"));
    renderConnect();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Impossibile contattare il server",
    );
  });

  it("posts the code with the header and reloads the status", async () => {
    mockStatus(status(true, false));
    renderConnect();
    await screen.findByRole("menuitem", { name: "Accedi con Google" });
    mockStatus(status(true, true));

    await clickSignIn();

    expect(mocks.post).toHaveBeenCalledWith("/api/google/code", {
      params: { header: { "X-Requested-With": "XmlHttpRequest" } },
      body: { code: "fake-code" },
    });
    expect(await screen.findByText("Collegato")).toBeVisible();
  });

  it("returns false without posting when the popup is closed", async () => {
    mockStatus(status(true, false));
    mocks.requester.mockResolvedValue(null);
    renderConnect();

    await clickSignIn();

    expect(mocks.requester).toHaveBeenCalled();
    expect(mocks.post).not.toHaveBeenCalled();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows the popup error when the requester rejects", async () => {
    mockStatus(status(true, false));
    mocks.requester.mockRejectedValue(
      new Error(
        "Il browser ha bloccato la finestra di Google: consenti i popup",
      ),
    );
    renderConnect();

    await clickSignIn();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Il browser ha bloccato la finestra di Google: consenti i popup",
    );
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("explains a reset grant", async () => {
    mockStatus(status(true, false));
    mocks.post.mockResolvedValue({
      error: { detail: "google_grant_reset" },
      response: { status: 409 },
    });
    renderConnect();

    await clickSignIn();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Google non ha concesso l'accesso permanente: riprova",
    );
  });

  it("explains missing scopes", async () => {
    mockStatus(status(true, false));
    mocks.post.mockResolvedValue({
      error: { detail: "google_scopes_missing" },
      response: { status: 403 },
    });
    renderConnect();

    await clickSignIn();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Consenti l'accesso a Calendar e Drive per continuare",
    );
  });

  it("asks to retry when the requester is not ready", async () => {
    const user = userEvent.setup();
    mockStatus(status(true, false));
    mocks.loadIdentity.mockReturnValue(new Promise(() => undefined));
    renderConnect();

    await user.click(
      await screen.findByRole("menuitem", { name: "Accedi con Google" }),
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Accesso Google non ancora pronto, riprova",
    );
    expect(mocks.requester).not.toHaveBeenCalled();
  });

  it("refreshes the status on window focus", async () => {
    mockStatus(status(true, false));
    renderConnect();
    await screen.findByRole("menuitem", { name: "Accedi con Google" });
    mockStatus(status(true, true));

    act(() => {
      window.dispatchEvent(new Event("focus"));
    });

    expect(await screen.findByText("Collegato")).toBeVisible();
  });

  it("sends DELETE on Scollega and refreshes", async () => {
    const user = userEvent.setup();
    mockStatus(status(true, true));
    renderConnect();
    await screen.findByText("Collegato");
    const before = mocks.get.mock.calls.length;
    mockStatus(status(true, false));

    await user.click(screen.getByRole("menuitem", { name: "Scollega Google" }));

    expect(mocks.del).toHaveBeenCalledWith("/api/google");
    await waitFor(() => {
      expect(mocks.get.mock.calls.length).toBeGreaterThan(before);
    });
    expect(
      await screen.findByRole("menuitem", { name: "Accedi con Google" }),
    ).toBeVisible();
  });
});
