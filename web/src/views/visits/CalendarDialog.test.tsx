import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AppointmentBody, GoogleStatusBody } from "../../api/client";
import { GoogleContext } from "../../google/useGoogle";
import { CalendarDialog } from "./CalendarDialog";
import { render } from "../../test/render";

const mocks = vi.hoisted(() => ({
  post: vi.fn(),
  get: vi.fn(),
  lookup: vi.fn(),
}));

vi.mock("../../api/client", () => ({
  api: {
    POST: (path: string, init: object): Promise<unknown> =>
      path === "/api/calendar/events/lookup"
        ? (mocks.lookup(path, init) as Promise<unknown>)
        : (mocks.post(path, init) as Promise<unknown>),
    GET: mocks.get,
  },
}));

function appointment(withBooking = true): AppointmentBody {
  return {
    id: "APP1",
    nre: "0900A0000000001",
    status: "PRENOTATO",
    at: "2026-11-12T09:30:00+01:00",
    ...(withBooking ? { bookingId: "ABC123456789" } : {}),
    unit: {
      name: "OSP ESEMPIO",
      description: "Ospedale Esempio",
      address: "Via Roma 1",
      city: "Firenze",
      province: "FI",
    },
  };
}

const clientId = "123456789012-abc.apps.googleusercontent.com";
const connected: GoogleStatusBody = {
  configured: true,
  connected: true,
  clientId,
  drive: false,
};

function renderDialog(
  options: {
    status?: GoogleStatusBody;
    appt?: AppointmentBody;
    signIn?: () => Promise<boolean>;
    refresh?: () => void;
    openSettings?: () => void;
    onClose?: () => void;
  } = {},
): void {
  render(
    <GoogleContext
      value={{
        status: options.status ?? connected,
        loading: false,
        error: undefined,
        refresh: options.refresh ?? vi.fn(),
        signIn: options.signIn ?? vi.fn().mockResolvedValue(true),
        disconnect: vi.fn(),
        openSettings: options.openSettings ?? vi.fn(),
      }}
    >
      <CalendarDialog
        appointment={options.appt ?? appointment()}
        title="Visita cardiologica"
        onClose={options.onClose ?? vi.fn()}
      />
    </GoogleContext>,
  );
}

function failWith(status: number, detail: string): void {
  mocks.post.mockResolvedValue({ error: { detail }, response: { status } });
}

const existingLink = "https://calendar.example.com/existing";

function lookupExists(): void {
  mocks.lookup.mockResolvedValue({
    data: { exists: true, htmlLink: existingLink },
  });
}

async function findAdd(): Promise<HTMLElement> {
  const button = await screen.findByRole("button", { name: "Aggiungi" });
  await waitFor(() => {
    expect(button).toBeEnabled();
  });
  return button;
}

describe("CalendarDialog", () => {
  beforeEach(() => {
    mocks.post.mockReset();
    mocks.get.mockReset();
    mocks.lookup.mockReset();
    mocks.get.mockResolvedValue({ data: { guests: [] } });
    mocks.lookup.mockResolvedValue({
      data: { exists: false, htmlLink: "" },
    });
  });

  it("shows the modal and the the summary", () => {
    renderDialog();

    expect(
      screen.getByRole("dialog", { name: "Aggiungi a Google Calendar" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Visita cardiologica")).toBeInTheDocument();
    expect(
      screen.getByText("Ospedale Esempio, Via Roma 1, Firenze, FI"),
    ).toBeInTheDocument();
  });

  it("ticks the attachment by default", () => {
    renderDialog();

    expect(
      screen.getByRole("checkbox", { name: "Allega promemoria (PDF)" }),
    ).toBeChecked();
  });

  it("hides the attachment without a booking id", () => {
    renderDialog({ appt: appointment(false) });

    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  });

  it("sends the exact body with the attachment by default", async () => {
    const user = userEvent.setup();
    mocks.post.mockResolvedValue({
      data: { created: true, htmlLink: "https://calendar.example.com/e1" },
    });
    renderDialog();

    await user.click(await findAdd());

    expect(mocks.post).toHaveBeenCalledWith("/api/calendar/events", {
      body: {
        appointmentId: "APP1",
        title: "Visita cardiologica",
        start: "2026-11-12T09:30:00+01:00",
        durationMinutes: 60,
        location: "Ospedale Esempio, Via Roma 1, Firenze, FI",
        details: "Prenotazione CUP Regione Toscana",
        bookingId: "ABC123456789",
        attachPromemoria: true,
        guests: [],
      },
    });
    expect(JSON.stringify(mocks.post.mock.calls[0])).not.toContain(
      "0900A0000000001",
    );
  });

  it("sends attachPromemoria false after unticking", async () => {
    const user = userEvent.setup();
    mocks.post.mockResolvedValue({
      data: { created: true, htmlLink: "https://calendar.example.com/e1" },
    });
    renderDialog();

    await user.click(
      screen.getByRole("checkbox", { name: "Allega promemoria (PDF)" }),
    );
    await user.click(await findAdd());

    expect(mocks.post.mock.calls[0]?.[1]).toMatchObject({
      body: { attachPromemoria: false },
    });
  });

  it("omits the booking id when there is none", async () => {
    const user = userEvent.setup();
    mocks.post.mockResolvedValue({
      data: { created: true, htmlLink: "https://calendar.example.com/e1" },
    });
    renderDialog({ appt: appointment(false) });

    await user.click(await findAdd());

    const body = (mocks.post.mock.calls[0]?.[1] as { body: object }).body;
    expect(body).not.toHaveProperty("bookingId");
    expect(body).toMatchObject({ attachPromemoria: false });
  });

  it("is busy and disabled while the request runs", async () => {
    const user = userEvent.setup();
    mocks.post.mockReturnValue(new Promise(() => undefined));
    renderDialog();

    await user.click(await findAdd());

    const add = screen.getByRole("button", { name: "Aggiungi" });
    expect(add).toBeDisabled();
    expect(add).toHaveAttribute("data-loading", "true");
  });

  it("shows the created message with the link and closes", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    mocks.post.mockResolvedValue({
      data: { created: true, htmlLink: "https://calendar.example.com/e1" },
    });
    renderDialog({ onClose });

    await user.click(await findAdd());

    expect(await screen.findByText(/Aggiunto al calendario/)).toBeVisible();
    const link = screen.getByRole("link", { name: "Apri in Google Calendar" });
    expect(link).toHaveAttribute("href", "https://calendar.example.com/e1");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
    await user.click(screen.getByRole("button", { name: "Chiudi" }));
    expect(onClose).toHaveBeenCalled();
  });

  it("shows the already present message", async () => {
    const user = userEvent.setup();
    mocks.post.mockResolvedValue({
      data: { created: false, htmlLink: "https://calendar.example.com/e1" },
    });
    renderDialog();

    await user.click(await findAdd());

    expect(
      await screen.findByText(/Già presente nel calendario/),
    ).toBeVisible();
  });

  it("calls onClose on Annulla", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderDialog({ onClose });

    await user.click(screen.getByRole("button", { name: "Annulla" }));

    expect(onClose).toHaveBeenCalled();
  });

  it("refreshes and asks to retry on 401", async () => {
    const user = userEvent.setup();
    const refresh = vi.fn();
    failWith(401, "google_not_connected");
    renderDialog({ refresh });

    await user.click(await findAdd());

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Accesso a Google scaduto: premi di nuovo Aggiungi",
    );
    expect(refresh).toHaveBeenCalled();
  });

  it("shows the missing promemoria error on 404", async () => {
    const user = userEvent.setup();
    failWith(404, "not found");
    renderDialog();

    await user.click(await findAdd());

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Promemoria non disponibile: riprova senza allegato.",
    );
  });

  it("offers Configura on 409 google_not_configured", async () => {
    const user = userEvent.setup();
    failWith(409, "google_not_configured");
    const openSettings = vi.fn();
    renderDialog({ openSettings });

    await user.click(await findAdd());

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Google non è configurato");
    await user.click(within(alert).getByRole("button", { name: "Configura" }));
    expect(openSettings).toHaveBeenCalled();
  });

  it("shows the gestione error on any other 409", async () => {
    const user = userEvent.setup();
    failWith(409, "complete the Gestione Prenotazioni login step");
    renderDialog();

    await user.click(await findAdd());

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Serve l'accesso a Gestione Prenotazioni: premi Accedi.",
    );
  });

  it("shows the generic error on 502", async () => {
    const user = userEvent.setup();
    failWith(502, "bad gateway");
    renderDialog();

    await user.click(await findAdd());

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Google o il portale regionale non rispondono. Riprova tra poco.",
    );
  });

  it("shows the generic error when the request throws", async () => {
    const user = userEvent.setup();
    mocks.post.mockRejectedValue(new Error("network"));
    renderDialog();

    await user.click(await findAdd());

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Google o il portale regionale non rispondono. Riprova tra poco.",
    );
  });

  it("signs in before posting the event when not connected", async () => {
    const user = userEvent.setup();
    const signIn = vi.fn().mockResolvedValue(true);
    mocks.post.mockResolvedValue({
      data: { created: true, htmlLink: "https://calendar.example.com/e/1" },
    });
    renderDialog({
      status: { ...connected, connected: false },
      signIn,
    });

    await user.click(await findAdd());

    expect(signIn).toHaveBeenCalled();
    expect(await screen.findByText(/Aggiunto al calendario/)).toBeVisible();
    expect(mocks.post).toHaveBeenCalledTimes(1);
  });

  it("does not post the event when the sign-in fails", async () => {
    const user = userEvent.setup();
    const signIn = vi.fn().mockResolvedValue(false);
    renderDialog({
      status: { ...connected, connected: false },
      signIn,
    });

    await user.click(await findAdd());

    expect(signIn).toHaveBeenCalled();
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("shows Configura and disables Aggiungi when not configured", async () => {
    const user = userEvent.setup();
    const openSettings = vi.fn();
    const onClose = vi.fn();
    renderDialog({
      status: { ...connected, configured: false, connected: false },
      openSettings,
      onClose,
    });

    expect(screen.getByText("Google non è configurato")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Configura" }));
    expect(onClose).toHaveBeenCalled();
    expect(openSettings).toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Aggiungi" })).toBeDisabled();
  });

  it("shows the existing event with a link and no Aggiungi", async () => {
    lookupExists();
    renderDialog();

    expect(await screen.findByText("Già nel calendario")).toBeVisible();
    const link = screen.getByRole("link", { name: "Apri in Google Calendar" });
    expect(link).toHaveAttribute("href", existingLink);
    expect(
      screen.queryByRole("button", { name: "Aggiungi" }),
    ).not.toBeInTheDocument();
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("regenerates the event with the guests", async () => {
    const user = userEvent.setup();
    lookupExists();
    mocks.get.mockResolvedValue({ data: { guests: ["guest@example.com"] } });
    mocks.post.mockResolvedValue({
      data: { created: true, htmlLink: "https://calendar.example.com/e2" },
    });
    renderDialog();

    await screen.findByText("Già nel calendario");
    await screen.findByText("guest@example.com");
    await user.click(screen.getByRole("button", { name: "Rigenera" }));

    expect(mocks.post.mock.calls[0]?.[1]).toMatchObject({
      body: { regenerate: true, guests: ["guest@example.com"] },
    });
    expect(await screen.findByText(/Aggiunto al calendario/)).toBeVisible();
  });

  it("renders Aggiungi as an icon button without visible text", async () => {
    renderDialog();

    const add = await findAdd();
    expect(add).toHaveAccessibleName("Aggiungi");
    expect(within(add).queryByText("Aggiungi")).not.toBeInTheDocument();
    expect(add.querySelector("svg")).not.toBeNull();
  });

  it("does not look up when not connected", async () => {
    renderDialog({ status: { ...connected, connected: false } });

    expect(
      await screen.findByRole("button", { name: "Aggiungi" }),
    ).toBeEnabled();
    expect(mocks.lookup).not.toHaveBeenCalled();
  });

  it("refreshes and offers Aggiungi when the lookup returns 401", async () => {
    const refresh = vi.fn();
    mocks.lookup.mockResolvedValue({
      error: { detail: "google_not_connected" },
      response: { status: 401 },
    });
    renderDialog({ refresh });

    expect(await findAdd()).toBeVisible();
    expect(refresh).toHaveBeenCalled();
  });

  it("falls back to Aggiungi on another lookup error", async () => {
    mocks.lookup.mockRejectedValue(new Error("network"));
    renderDialog();

    expect(await findAdd()).toBeVisible();
  });

  it("pre-fills the guests from the settings and sends them", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: { guests: ["guest@example.com"] } });
    mocks.post.mockResolvedValue({
      data: { created: true, htmlLink: "https://calendar.example.com/e1" },
    });
    renderDialog();

    await screen.findByText("guest@example.com");
    await user.click(await findAdd());

    const body = (mocks.post.mock.calls[0]?.[1] as { body: object }).body;
    expect(body).toMatchObject({ guests: ["guest@example.com"] });
    expect(body).not.toHaveProperty("regenerate");
  });

  it("sends an empty guest list after removing all", async () => {
    const user = userEvent.setup();
    mocks.get.mockResolvedValue({ data: { guests: ["guest@example.com"] } });
    mocks.post.mockResolvedValue({
      data: { created: true, htmlLink: "https://calendar.example.com/e1" },
    });
    renderDialog();

    await screen.findByText("guest@example.com");
    await user.click(screen.getByRole("combobox", { name: "Invitati" }));
    await user.keyboard("{Backspace}");
    await user.click(await findAdd());

    expect(mocks.post.mock.calls[0]?.[1]).toMatchObject({
      body: { guests: [] },
    });
  });

  it("flags an invalid guest and disables Aggiungi", async () => {
    const user = userEvent.setup();
    renderDialog();

    await findAdd();
    await user.type(
      screen.getByRole("combobox", { name: "Invitati" }),
      "nonsense{Enter}",
    );

    expect(await screen.findByText("Email non valida")).toBeVisible();
    expect(screen.getByRole("button", { name: "Aggiungi" })).toBeDisabled();
  });

  it("omits guests when the settings fail to load", async () => {
    const user = userEvent.setup();
    mocks.get.mockRejectedValue(new Error("network"));
    mocks.post.mockResolvedValue({
      data: { created: true, htmlLink: "https://calendar.example.com/e1" },
    });
    renderDialog();

    await user.click(await findAdd());

    const body = (mocks.post.mock.calls[0]?.[1] as { body: object }).body;
    expect(body).not.toHaveProperty("guests");
  });

  it("shows the server detail on 422", async () => {
    const user = userEvent.setup();
    failWith(422, "Indirizzo email dell'invitato non valido");
    renderDialog();

    await user.click(await findAdd());

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Indirizzo email dell'invitato non valido",
    );
  });
});
