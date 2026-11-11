import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StatusBadge } from "./StatusBadge";
import { render } from "../test/render";

describe("StatusBadge", () => {
  it("renders the text", () => {
    render(<StatusBadge tone="booked">Prenotato</StatusBadge>);
    expect(screen.getByText("Prenotato")).toBeVisible();
  });

  it("renders the text for every tone", () => {
    render(
      <>
        <StatusBadge tone="done">Eseguito</StatusBadge>
        <StatusBadge tone="tobook">Da prenotare</StatusBadge>
        <StatusBadge tone="pending">In attesa</StatusBadge>
        <StatusBadge tone="closed">Chiuso</StatusBadge>
        <StatusBadge tone="error">Errore</StatusBadge>
      </>,
    );
    expect(screen.getByText("Eseguito")).toBeVisible();
    expect(screen.getByText("Da prenotare")).toBeVisible();
    expect(screen.getByText("In attesa")).toBeVisible();
    expect(screen.getByText("Chiuso")).toBeVisible();
    expect(screen.getByText("Errore")).toBeVisible();
  });
});
