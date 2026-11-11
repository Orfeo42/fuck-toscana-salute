import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AppointmentBody, GoogleStatusBody } from "../../api/client";
import { GoogleContext } from "../../google/useGoogle";
import { CalendarDialog } from "./CalendarDialog";

const mocks = vi.hoisted(() => ({
  post: vi.fn(),
}));

vi.mock("../../api/client", () => ({
  api: { POST: mocks.post },
}));

const showModal = vi.fn(function showModal(this: HTMLDialogElement) {
  this.setAttribute("open", "");
});

const hint =
  "Crea ~/.config/fuck-toscana-salute/google-client.json (vedi README)";

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

const connected: GoogleStatusBody = { configured: true, connected: true };

function renderDialog(
  options: {
    status?: GoogleStatusBody;
    appt?: AppointmentBody;
    connect?: () => Promise<void>;
    onClose?: () => void;
  } = {},
): void {
  render(
    <GoogleContext
      value={{
        status: options.status ?? connected,
        loading: false,
        error: undefined,
        refresh: vi.fn(),
        connect: options.connect ?? vi.fn(),
        disconnect: vi.fn(),
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

describe("CalendarDialog", () => {
  beforeEach(() => {
    mocks.post.mockReset();
    showModal.mockClear();
    HTMLDialogElement.prototype.showModal = showModal;
    HTMLDialogElement.prototype.close = vi.fn();
  });

  it("opens the modal and shows the summary", () => {
    renderDialog();

    expect(showModal).toHaveBeenCalledTimes(1);
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

    await user.click(screen.getByRole("button", { name: "Aggiungi" }));

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
    await user.click(screen.getByRole("button", { name: "Aggiungi" }));

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

    await user.click(screen.getByRole("button", { name: "Aggiungi" }));

    const body = (mocks.post.mock.calls[0]?.[1] as { body: object }).body;
    expect(body).not.toHaveProperty("bookingId");
    expect(body).toMatchObject({ attachPromemoria: false });
  });

  it("is busy and disabled while the request runs", async () => {
    const user = userEvent.setup();
    mocks.post.mockReturnValue(new Promise(() => undefined));
    renderDialog();

    await user.click(screen.getByRole("button", { name: "Aggiungi" }));

    const add = screen.getByRole("button", { name: "Aggiungi" });
    expect(add).toBeDisabled();
    expect(add).toHaveAttribute("aria-busy", "true");
  });

  it("shows the created message with the link and closes", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    mocks.post.mockResolvedValue({
      data: { created: true, htmlLink: "https://calendar.example.com/e1" },
    });
    renderDialog({ onClose });

    await user.click(screen.getByRole("button", { name: "Aggiungi" }));

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

    await user.click(screen.getByRole("button", { name: "Aggiungi" }));

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

  it("shows the expired link error with a connect button on 401", async () => {
    const user = userEvent.setup();
    const connect = vi.fn().mockResolvedValue(undefined);
    failWith(401, "google_not_connected");
    renderDialog({ connect });

    await user.click(screen.getByRole("button", { name: "Aggiungi" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Collegamento Google scaduto: ricollega.",
    );
    await user.click(
      screen.getByRole("button", { name: "Collega Google Calendar" }),
    );
    expect(connect).toHaveBeenCalled();
  });

  it("shows the missing promemoria error on 404", async () => {
    const user = userEvent.setup();
    failWith(404, "not found");
    renderDialog();

    await user.click(screen.getByRole("button", { name: "Aggiungi" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Promemoria non disponibile: riprova senza allegato.",
    );
  });

  it("shows the hint on 409 google_not_configured", async () => {
    const user = userEvent.setup();
    failWith(409, "google_not_configured");
    renderDialog();

    await user.click(screen.getByRole("button", { name: "Aggiungi" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(hint);
  });

  it("shows the gestione error on any other 409", async () => {
    const user = userEvent.setup();
    failWith(409, "complete the Gestione Prenotazioni login step");
    renderDialog();

    await user.click(screen.getByRole("button", { name: "Aggiungi" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Serve l'accesso a Gestione Prenotazioni: premi Accedi.",
    );
  });

  it("shows the generic error on 502", async () => {
    const user = userEvent.setup();
    failWith(502, "bad gateway");
    renderDialog();

    await user.click(screen.getByRole("button", { name: "Aggiungi" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Google o il portale regionale non rispondono. Riprova tra poco.",
    );
  });

  it("shows the generic error when the request throws", async () => {
    const user = userEvent.setup();
    mocks.post.mockRejectedValue(new Error("network"));
    renderDialog();

    await user.click(screen.getByRole("button", { name: "Aggiungi" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Google o il portale regionale non rispondono. Riprova tra poco.",
    );
  });

  it("offers connect and disables Aggiungi when not connected", async () => {
    const user = userEvent.setup();
    const connect = vi.fn().mockResolvedValue(undefined);
    renderDialog({ status: { configured: true, connected: false }, connect });

    expect(screen.getByRole("button", { name: "Aggiungi" })).toBeDisabled();
    await user.click(
      screen.getByRole("button", { name: "Collega Google Calendar" }),
    );
    expect(connect).toHaveBeenCalled();
  });

  it("shows the hint and disables Aggiungi when not configured", () => {
    renderDialog({ status: { configured: false, connected: false } });

    expect(screen.getByText(hint)).toBeVisible();
    expect(screen.getByRole("button", { name: "Aggiungi" })).toBeDisabled();
  });
});
