import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { render } from "../test/render";
import { DetailSection } from "./DetailSection";

describe("DetailSection", () => {
  it("renders the title and children without actions", () => {
    render(
      <DetailSection title="Stato">
        <p>contenuto</p>
      </DetailSection>,
    );
    expect(screen.getByRole("heading", { name: "Stato" })).toBeVisible();
    expect(screen.getByText("contenuto")).toBeVisible();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("renders the actions slot when given", () => {
    render(
      <DetailSection title="Stato" actions={<button type="button">Vai</button>}>
        <p>contenuto</p>
      </DetailSection>,
    );
    expect(screen.getByRole("button", { name: "Vai" })).toBeVisible();
  });
});
