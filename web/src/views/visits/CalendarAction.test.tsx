import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { JSX } from "react";
import { describe, expect, it, vi } from "vitest";

import type { VisitAppointmentBody } from "../../api/client";
import { render } from "../../test/render";
import { CalendarAction } from "./CalendarAction";

vi.mock("./CalendarDialog", () => ({
  CalendarDialog: ({ onClose }: { onClose: () => void }): JSX.Element => (
    <button type="button" onClick={onClose}>
      chiudi dialogo
    </button>
  ),
}));

const appointment: VisitAppointmentBody = {
  id: "a1",
  status: "PRENOTATO",
  at: "2026-12-01T09:00:00Z",
  upcoming: true,
  calendar: {
    title: "Visita esempio",
    location: "Unita esempio, Via Esempio 1, Firenze, FI",
    details: "Prenotazione CUP Regione Toscana",
  },
  unit: {
    name: "Unita esempio",
    description: "",
    displayName: "Unita esempio",
    address: "Via Esempio 1",
    postalCode: "50100",
    city: "Firenze",
    province: "FI",
    fullAddress: "Via Esempio 1, 50100 Firenze, FI",
    mapsSearchUrl: "https://maps.example.com/search",
    mapsDirectionsUrl: "https://maps.example.com/dir",
  },
};

describe("CalendarAction", () => {
  it("mounts the dialog only while open", async () => {
    const user = userEvent.setup();
    render(<CalendarAction appointment={appointment} />);
    expect(screen.queryByText("chiudi dialogo")).toBeNull();
    await user.click(
      screen.getByRole("button", { name: "Aggiungi a Google Calendar" }),
    );
    await user.click(screen.getByText("chiudi dialogo"));
    expect(screen.queryByText("chiudi dialogo")).toBeNull();
  });
});
