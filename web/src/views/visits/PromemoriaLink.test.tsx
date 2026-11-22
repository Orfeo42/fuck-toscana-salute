import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { render } from "../../test/render";
import { PromemoriaLink } from "./PromemoriaLink";

vi.mock("../../lib/pdf", () => ({
  loadPdf: () => ({
    promise: new Promise<never>(() => undefined),
    destroy: () => Promise.resolve(),
  }),
}));

const openLabel = "Apri promemoria prenotazione (PDF)";

describe("PromemoriaLink", () => {
  it("mounts the viewer dialog only while open", async () => {
    const user = userEvent.setup();
    render(<PromemoriaLink bookingId="b1" />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: openLabel }));
    expect(
      await screen.findByRole("dialog", { name: "Promemoria" }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Chiudi" }));
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  });

  it("renders nothing without a booking id", () => {
    render(<PromemoriaLink bookingId={undefined} />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
