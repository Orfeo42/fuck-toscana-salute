import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { SessionBody } from "../api/client";
import { SessionContext, type SessionValue } from "../session/useSession";
import { render } from "../test/render";
import { LinkView } from "./LinkView";

const mocks = vi.hoisted(() => ({
  post: vi.fn(),
  close: vi.fn(),
}));

vi.mock("../api/client", () => ({
  api: { POST: mocks.post },
}));

const cupOrigin = "https://prenota.sanita.toscana.it";
const fseOrigin = "https://fse.sanita.toscana.it";

const session: SessionBody = {
  gestioneReady: true,
  loginMode: "bookmarklet",
  viewer: false,
  portals: [
    { portal: "cup", state: "active", login: { phase: "idle" } },
    { portal: "fse", state: "logged_out", login: { phase: "idle" } },
  ],
};

const tokensData = {
  type: "fts-tokens",
  access: "access-token",
  refresh: "refresh-token",
  cf: "RSSMRA80A01H501U",
  tessera: "80380000000000000000",
};

function renderView(value: Partial<SessionValue> = {}): SessionValue {
  const full: SessionValue = {
    session,
    error: undefined,
    refresh: vi.fn(),
    loginAll: vi.fn(),
    logout: vi.fn(),
    ...value,
  };
  render(
    <SessionContext value={full}>
      <LinkView />
    </SessionContext>,
  );
  return full;
}

function postFrom(origin: string, data: unknown): void {
  act(() => {
    window.dispatchEvent(new MessageEvent("message", { origin, data }));
  });
}

describe("LinkView", () => {
  beforeEach(() => {
    mocks.post.mockReset();
    mocks.post.mockResolvedValue({ response: { status: 204 } });
    mocks.close.mockReset();
    vi.spyOn(window, "close").mockImplementation(mocks.close);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    Reflect.deleteProperty(window, "opener");
  });

  it("explains the steps and links to both portals", () => {
    renderView();

    expect(screen.getByRole("link", { name: /Apri CUP/ })).toHaveAttribute(
      "href",
      "https://auth.regione.toscana.it/auth/realms/rt/protocol/openid-connect/auth?response_type=code&client_id=r_toscan__portale_cup2_0&redirect_uri=https://prenota.sanita.toscana.it&scope=default",
    );
    expect(screen.getByRole("link", { name: /Apri FSE/ })).toHaveAttribute(
      "href",
      "https://auth.regione.toscana.it/auth/realms/rt/protocol/openid-connect/auth?response_type=code&client_id=r_toscan__fse_web&redirect_uri=https://fse.sanita.toscana.it/&scope=profile+sdm-rs/cittadino+sisorchestrator-rs/cittadino+vaccini-rs/cittadino",
    );
    expect(screen.getAllByRole("listitem")).toHaveLength(4);
  });

  it("renders the bookmarklet link with a javascript href for this origin", () => {
    renderView();

    const href = screen
      .getByText("Invia a Toscana Salute", { selector: "a" })
      .getAttribute("href");
    expect(href).toContain("javascript:");
    expect(href).toContain(JSON.stringify(window.location.origin));
  });

  it("copies the bookmarklet code", async () => {
    const user = userEvent.setup();
    renderView();

    await user.click(screen.getByRole("button", { name: "Copia il codice" }));

    expect(await navigator.clipboard.readText()).toMatch(/^javascript:/);
    expect(
      screen.getByRole("button", { name: "Codice copiato" }),
    ).toBeInTheDocument();
  });

  it("shows the per-portal status", () => {
    renderView();

    expect(screen.getAllByText("collegato")).toHaveLength(1);
    expect(screen.getAllByText("non collegato")).toHaveLength(1);
    expect(screen.getByText("disponibili")).toBeInTheDocument();
  });

  it("shows not connected before the session loads", () => {
    renderView({ session: undefined });

    expect(screen.getAllByText("non collegato")).toHaveLength(2);
    expect(screen.getByText("non disponibili")).toBeInTheDocument();
  });

  it("tells the opener it is ready without data", () => {
    const postMessage = vi.fn();
    Object.defineProperty(window, "opener", {
      value: { postMessage },
      configurable: true,
    });
    renderView();

    expect(postMessage).toHaveBeenCalledWith({ type: "fts-ready" }, "*");
  });

  it("imports tokens sent from the CUP origin", async () => {
    const value = renderView();

    postFrom(cupOrigin, tokensData);

    await waitFor(() => {
      expect(mocks.post).toHaveBeenCalledWith("/api/login/import", {
        body: {
          portal: "cup",
          access: "access-token",
          refresh: "refresh-token",
          cf: "RSSMRA80A01H501U",
          tessera: "80380000000000000000",
        },
      });
    });
    expect(await screen.findAllByText("CUP collegato")).not.toHaveLength(0);
    expect(screen.getByText(/Puoi chiudere questa scheda/)).toBeInTheDocument();
    expect(value.refresh).toHaveBeenCalled();
    expect(mocks.close).toHaveBeenCalled();
  });

  it("omits an empty tessera and maps the FSE origin", async () => {
    renderView();

    postFrom(fseOrigin, { ...tokensData, tessera: "" });

    await waitFor(() => {
      expect(mocks.post).toHaveBeenCalledWith("/api/login/import", {
        body: {
          portal: "fse",
          access: "access-token",
          refresh: "refresh-token",
          cf: "RSSMRA80A01H501U",
        },
      });
    });
    expect(await screen.findAllByText("FSE collegato")).not.toHaveLength(0);
  });

  it("ignores messages from other origins", () => {
    renderView();

    postFrom("https://evil.example", tokensData);
    postFrom("https://prenota.sanita.toscana.it.evil.example", tokensData);

    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("ignores malformed or unrelated messages", () => {
    renderView();

    postFrom(cupOrigin, { type: "fts-ready" });
    postFrom(cupOrigin, { ...tokensData, access: 42 });
    postFrom(cupOrigin, "fts-tokens");

    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("shows the API error message", async () => {
    mocks.post.mockResolvedValue({
      error: { detail: "Il token è scaduto" },
      response: { status: 422 },
    });
    const value = renderView();

    postFrom(cupOrigin, tokensData);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Il token è scaduto",
    );
    expect(value.refresh).not.toHaveBeenCalled();
    expect(mocks.close).not.toHaveBeenCalled();
  });

  it("shows a generic error when the request fails", async () => {
    mocks.post.mockRejectedValue(new Error("network"));
    renderView();

    postFrom(cupOrigin, tokensData);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Impossibile collegare il portale",
    );
  });

  it("stops listening after unmount", () => {
    const view = render(
      <SessionContext
        value={{
          session,
          error: undefined,
          refresh: vi.fn(),
          loginAll: vi.fn(),
          logout: vi.fn(),
        }}
      >
        <LinkView />
      </SessionContext>,
    );
    view.unmount();

    postFrom(cupOrigin, tokensData);

    expect(mocks.post).not.toHaveBeenCalled();
  });
});
