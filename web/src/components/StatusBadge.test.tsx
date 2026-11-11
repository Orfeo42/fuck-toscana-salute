import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StatusBadge } from "./StatusBadge";

describe("StatusBadge", () => {
  it("renders the text with the tone class", () => {
    render(<StatusBadge tone="booked">Prenotato</StatusBadge>);
    const badge = screen.getByText("Prenotato");
    expect(badge).toHaveClass("status-badge", "status-booked");
  });
});
