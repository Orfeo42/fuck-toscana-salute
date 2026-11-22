import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { JSX } from "react";
import { describe, expect, it, vi } from "vitest";

import type { AppointmentBody } from "../../api/client";
import { render } from "../../test/render";
import { CalendarAction } from "./CalendarAction";

vi.mock("./CalendarDialog", () => ({
  CalendarDialog: ({ onClose }: { onClose: () => void }): JSX.Element => (
    <button type="button" onClick={onClose}>
      chiudi dialogo
    </button>
  ),
}));

const appointment: AppointmentBody = {
  id: "a1",
  nre: "0900A0000000001",
  status: "PRENOTATO",
  at: "2026-12-01T09:00:00Z",
  unit: {
    name: "Unita esempio",
    description: "",
    address: "Via Esempio 1",
    postalCode: "50100",
    city: "Firenze",
    province: "FI",
  },
};

describe("CalendarAction", () => {
  it("mounts the dialog only while open", async () => {
    const user = userEvent.setup();
    render(<CalendarAction appointment={appointment} title="Visita esempio" />);
    expect(screen.queryByText("chiudi dialogo")).toBeNull();
    await user.click(
      screen.getByRole("button", { name: "Aggiungi a Google Calendar" }),
    );
    await user.click(screen.getByText("chiudi dialogo"));
    expect(screen.queryByText("chiudi dialogo")).toBeNull();
  });
});
