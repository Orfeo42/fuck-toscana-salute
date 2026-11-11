import { act, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { render } from "../test/render";
import { AppNav } from "./AppNav";

describe("AppNav", () => {
  beforeEach(() => {
    window.location.hash = "";
  });

  it("renders the navigation landmark with both links", () => {
    render(<AppNav onNavigate={vi.fn()} />);
    expect(
      screen.getByRole("navigation", { name: "Sezioni" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Visite ed esami" }),
    ).toHaveAttribute("href", "#/visite");
    expect(screen.getByRole("link", { name: "Referti" })).toHaveAttribute(
      "href",
      "#/referti",
    );
    expect(screen.getByRole("link", { name: "Farmaci" })).toHaveAttribute(
      "href",
      "#/farmaci",
    );
  });

  it("lists Referti between Visite ed esami and Farmaci", () => {
    render(<AppNav onNavigate={vi.fn()} />);
    const names = screen.getAllByRole("link").map((link) => link.textContent);
    expect(names).toEqual(["Visite ed esami", "Referti", "Farmaci"]);
  });

  it("marks the first section active by default", () => {
    render(<AppNav onNavigate={vi.fn()} />);
    expect(
      screen.getByRole("link", { name: "Visite ed esami" }),
    ).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Farmaci" })).not.toHaveAttribute(
      "aria-current",
    );
  });

  it("follows the hash", () => {
    render(<AppNav onNavigate={vi.fn()} />);
    act(() => {
      window.location.hash = "#/farmaci";
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });
    expect(screen.getByRole("link", { name: "Farmaci" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(
      screen.getByRole("link", { name: "Visite ed esami" }),
    ).not.toHaveAttribute("aria-current");
  });

  it("calls onNavigate when a link is clicked", async () => {
    const onNavigate = vi.fn();
    render(<AppNav onNavigate={onNavigate} />);
    await userEvent.click(screen.getByRole("link", { name: "Farmaci" }));
    expect(onNavigate).toHaveBeenCalledTimes(1);
  });
});
