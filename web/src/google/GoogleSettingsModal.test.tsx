import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { GoogleSettingsBody } from "../api/client";
import { render } from "../test/render";
import { GoogleSettingsModal } from "./GoogleSettingsModal";
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
  hasSecret: true,
  redirectUri: "http://localhost:8080/api/google/callback",
};

const emptySettings: GoogleSettingsBody = {
  clientId: "",
  calendarId: "",
  driveFolderId: "",
  hasSecret: false,
  redirectUri: "http://localhost:8080/api/google/callback",
};

function renderModal(): { refresh: () => void; onClose: () => void } {
  const refresh = vi.fn();
  const onClose = vi.fn();
  render(
    <GoogleContext
      value={{
        status: undefined,
        loading: false,
        error: undefined,
        refresh,
        connect: vi.fn(),
        disconnect: vi.fn(),
        openSettings: vi.fn(),
      }}
    >
      <GoogleSettingsModal opened onClose={onClose} />
    </GoogleContext>,
  );
  return { refresh, onClose };
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
      "Già salvato — lascia vuoto per non cambiarlo",
    );
    expect(screen.getByLabelText("Calendario")).toHaveValue("primary");
    expect(screen.getByLabelText("Cartella Drive")).toHaveValue("");
  });

  it("defaults the calendar to primary", async () => {
    mocks.get.mockResolvedValue({ data: emptySettings });
    renderModal();

    expect(await screen.findByLabelText("Calendario")).toHaveValue("primary");
    expect(screen.getByLabelText("Client secret")).not.toHaveAttribute(
      "placeholder",
    );
  });

  it("shows the redirect URI with a copy button", async () => {
    mocks.get.mockResolvedValue({ data: savedSettings });
    renderModal();

    expect(
      await screen.findByRole("button", {
        name: `Copia redirect URI ${savedSettings.redirectUri}`,
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

  it("omits the empty secret and drive folder from the body", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: savedSettings });
    renderModal();

    await user.click(await screen.findByRole("button", { name: "Salva" }));

    expect(mocks.put).toHaveBeenCalledWith("/api/google/settings", {
      body: {
        clientId: "123-abc.apps.googleusercontent.com",
        calendarId: "primary",
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
    await user.type(screen.getByLabelText("Cartella Drive"), "folder-1");
    await user.click(screen.getByRole("button", { name: "Salva" }));

    expect(mocks.put).toHaveBeenCalledWith("/api/google/settings", {
      body: {
        clientId: "123-abc.apps.googleusercontent.com",
        clientSecret: "s3cret",
        calendarId: "primary",
        driveFolderId: "folder-1",
      },
    });
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
