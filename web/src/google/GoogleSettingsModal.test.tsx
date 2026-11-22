import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { GoogleSettingsBody } from "../api/client";
import { render } from "../test/render";
import { GoogleSettingsModal } from "./GoogleSettingsModal";
import type { GoogleStatusBody } from "../api/client";
import { googleConsoleLinks } from "./googleConsoleLinks";
import { GoogleContext } from "./useGoogle";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
}));

vi.mock("../api/client", () => ({
  api: { GET: mocks.get, PUT: mocks.put },
}));

const savedSettings: GoogleSettingsBody = {
  clientId: "123-abc.apps.googleusercontent.com",
  calendarId: "primary",
  driveFolderId: "",
  apiKey: "key-1",
  guests: ["ospite@example.com"],
  hasSecret: true,
  javascriptOrigin: "http://localhost:8800",
};

const emptySettings: GoogleSettingsBody = {
  clientId: "",
  calendarId: "",
  driveFolderId: "",
  apiKey: "",
  guests: [],
  hasSecret: false,
  javascriptOrigin: "http://localhost:8800",
};

interface Rendered {
  refresh: () => void;
  onClose: () => void;
  signIn: () => Promise<boolean>;
  disconnect: () => Promise<void>;
}

function renderModal(status?: GoogleStatusBody, error?: string): Rendered {
  const refresh = vi.fn();
  const onClose = vi.fn();
  const signIn = vi.fn().mockResolvedValue(true);
  const disconnect = vi.fn().mockResolvedValue(undefined);
  render(
    <GoogleContext
      value={{
        status,
        loading: false,
        error,
        refresh,
        signIn,
        disconnect,
        openSettings: vi.fn(),
      }}
    >
      <GoogleSettingsModal opened onClose={onClose} />
    </GoogleContext>,
  );
  return { refresh, onClose, signIn, disconnect };
}

describe("GoogleSettingsModal", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    mocks.put.mockReset();
    mocks.put.mockResolvedValue({ response: { status: 204 } });
  });

  it("prefills the saved settings and leaves the secret empty", async () => {
    mocks.get.mockResolvedValue({ data: savedSettings });
    renderModal();

    expect(await screen.findByLabelText("Client ID")).toHaveValue(
      "123-abc.apps.googleusercontent.com",
    );
    expect(screen.getByLabelText("Client secret")).toHaveValue("");
    expect(screen.getByLabelText("Client secret")).toHaveAttribute(
      "placeholder",
      "Salvato — lascia vuoto per non cambiarlo",
    );
    expect(
      screen.getByRole("textbox", { name: "Calendario", hidden: true }),
    ).toHaveValue("primary");
    expect(screen.getByLabelText("Cartella Drive")).toHaveValue("");
    expect(screen.getByLabelText("API key (Picker)")).toHaveValue("key-1");
    expect(screen.getByText("ospite@example.com")).toBeInTheDocument();
  });

  it("defaults the calendar to primary", async () => {
    mocks.get.mockResolvedValue({ data: emptySettings });
    renderModal();

    expect(
      await screen.findByRole("textbox", { name: "Calendario", hidden: true }),
    ).toHaveValue("primary");
    expect(screen.getByLabelText("Client secret")).not.toHaveAttribute(
      "placeholder",
    );
  });

  it("has no App ID field", async () => {
    mocks.get.mockResolvedValue({ data: savedSettings });
    renderModal();

    await screen.findByLabelText("Client ID");

    expect(screen.queryByLabelText("App ID (numero progetto)")).toBeNull();
  });

  it("shows the JavaScript origin with a copy button", async () => {
    mocks.get.mockResolvedValue({ data: savedSettings });
    renderModal();

    expect(
      await screen.findByRole("button", {
        name: `Copia origine JavaScript ${savedSettings.javascriptOrigin}`,
      }),
    ).toBeVisible();
  });

  it("shows an alert when the settings cannot be read", async () => {
    mocks.get.mockRejectedValue(new Error("network"));
    renderModal();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Impossibile leggere le impostazioni Google",
    );
  });

  it("requires the client id and the secret when none is saved", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: emptySettings });
    renderModal();

    await user.click(await screen.findByRole("button", { name: "Salva" }));

    expect(screen.getByText("Il Client ID è obbligatorio")).toBeVisible();
    expect(screen.getByText("Il client secret è obbligatorio")).toBeVisible();
    expect(mocks.put).not.toHaveBeenCalled();
  });

  it("does not require the secret when one is saved", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: savedSettings });
    renderModal();

    await user.click(await screen.findByRole("button", { name: "Salva" }));

    expect(mocks.put).toHaveBeenCalled();
  });

  it("omits the empty secret, drive folder, and api key from the body", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({
      data: { ...savedSettings, apiKey: "" },
    });
    renderModal();

    await user.click(await screen.findByRole("button", { name: "Salva" }));

    expect(mocks.put).toHaveBeenCalledWith("/api/google/settings", {
      body: {
        clientId: "123-abc.apps.googleusercontent.com",
        calendarId: "primary",
        guests: ["ospite@example.com"],
      },
    });
  });

  it("sends every filled field", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: emptySettings });
    renderModal();

    await user.type(
      await screen.findByLabelText("Client ID"),
      "123-abc.apps.googleusercontent.com",
    );
    await user.type(screen.getByLabelText("Client secret"), "s3cret");
    await user.click(screen.getByRole("tab", { name: "Drive" }));
    await user.type(screen.getByLabelText("Cartella Drive"), "folder-1");
    await user.type(screen.getByLabelText("API key (Picker)"), "key-2");
    await user.click(screen.getByRole("button", { name: "Salva" }));

    expect(mocks.put).toHaveBeenCalledWith("/api/google/settings", {
      body: {
        clientId: "123-abc.apps.googleusercontent.com",
        clientSecret: "s3cret",
        calendarId: "primary",
        driveFolderId: "folder-1",
        apiKey: "key-2",
        guests: [],
      },
    });
  });

  it("shows the unconfigured badge without a sign-in button", async () => {
    mocks.get.mockResolvedValue({ data: savedSettings });
    renderModal({
      configured: false,
      connected: false,
      clientId: "",
      drive: false,
    });

    expect(await screen.findByText("Non configurato")).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Accedi con Google" }),
    ).toBeNull();
    expect(screen.queryByRole("button", { name: "Scollega" })).toBeNull();
  });

  it("signs in when configured and not connected", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: savedSettings });
    const { signIn } = renderModal(
      { configured: true, connected: false, clientId: "x", drive: false },
      "accesso negato",
    );

    expect(await screen.findByText("Non collegato")).toBeVisible();
    expect(screen.getByText("accesso negato")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Accedi con Google" }));

    expect(signIn).toHaveBeenCalledTimes(1);
  });

  it("disconnects when connected", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: savedSettings });
    const { disconnect } = renderModal({
      configured: true,
      connected: true,
      clientId: "x",
      drive: true,
    });

    expect(await screen.findByText("Collegato")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Scollega" }));

    expect(disconnect).toHaveBeenCalledTimes(1);
  });

  it("switches tabs", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: savedSettings });
    renderModal();

    await screen.findByLabelText("Client ID");
    expect(
      screen.getByRole("textbox", { name: "Calendario", hidden: true }),
    ).not.toBeVisible();
    await user.click(screen.getByRole("tab", { name: "Calendario" }));

    expect(
      screen.getByRole("textbox", { name: "Calendario", hidden: true }),
    ).toBeVisible();
    expect(screen.getByLabelText("Client ID")).not.toBeVisible();
  });

  it("disables Salva for an invalid guest email", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: savedSettings });
    renderModal();

    await user.click(await screen.findByRole("tab", { name: "Calendario" }));
    await user.type(
      screen.getByRole("combobox", { name: "Invitati predefiniti" }),
      "non-valida,",
    );

    expect(screen.getByText("Email non valida")).toBeVisible();
    expect(screen.getByRole("button", { name: "Salva" })).toBeDisabled();
  });

  it("sends the folder id extracted from a pasted URL", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: savedSettings });
    renderModal();

    await user.click(await screen.findByRole("tab", { name: "Drive" }));
    await user.click(screen.getByLabelText("Cartella Drive"));
    await user.paste(
      "https://drive.google.com/drive/folders/1AbC_d-9?usp=sharing",
    );

    expect(screen.getByText("ID: 1AbC_d-9")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Salva" }));

    expect(mocks.put).toHaveBeenCalledWith("/api/google/settings", {
      body: {
        clientId: "123-abc.apps.googleusercontent.com",
        calendarId: "primary",
        driveFolderId: "1AbC_d-9",
        apiKey: "key-1",
        guests: ["ospite@example.com"],
      },
    });
  });

  it("renders the six useful links", async () => {
    mocks.get.mockResolvedValue({ data: savedSettings });
    renderModal();

    await screen.findByLabelText("Client ID");
    const group = within(screen.getByRole("group", { name: "Link utili" }));

    expect(group.getByRole("link", { name: "API abilitate" })).toHaveAttribute(
      "href",
      googleConsoleLinks.dashboard,
    );
    expect(group.getByRole("link", { name: "Libreria API" })).toHaveAttribute(
      "href",
      googleConsoleLinks.library,
    );
    expect(
      group.getByRole("link", { name: "Credenziali (API key)" }),
    ).toHaveAttribute("href", googleConsoleLinks.credentials);
    expect(group.getByRole("link", { name: "Client OAuth" })).toHaveAttribute(
      "href",
      googleConsoleLinks.clients,
    );
    expect(
      group.getByRole("link", { name: "Accesso ai dati (scope)" }),
    ).toHaveAttribute("href", googleConsoleLinks.scopes);
    expect(
      group.getByRole("link", { name: "Pubblico e utenti di test" }),
    ).toHaveAttribute("href", googleConsoleLinks.audience);
  });

  it("shows the validation message on 422", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: savedSettings });
    mocks.put.mockResolvedValue({
      error: {
        detail: "validation failed",
        errors: [{ message: "client id non valido" }],
      },
      response: { status: 422 },
    });
    const { onClose } = renderModal();

    await user.click(await screen.findByRole("button", { name: "Salva" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "client id non valido",
    );
    expect(onClose).not.toHaveBeenCalled();
  });

  it("falls back to the problem detail", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: savedSettings });
    mocks.put.mockResolvedValue({
      error: { detail: "calendario non valido" },
      response: { status: 422 },
    });
    renderModal();

    await user.click(await screen.findByRole("button", { name: "Salva" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "calendario non valido",
    );
  });

  it("closes, notifies and refreshes the status on success", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: savedSettings });
    const { onClose, refresh } = renderModal();

    await user.click(await screen.findByRole("button", { name: "Salva" }));

    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
    expect(refresh).toHaveBeenCalled();
    expect(
      await screen.findByText("Impostazioni Google salvate"),
    ).toBeVisible();
  });
});
