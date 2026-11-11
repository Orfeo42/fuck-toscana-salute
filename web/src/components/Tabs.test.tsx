import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";

import { Tabs, type TabItem } from "./Tabs";

const tabs: readonly TabItem[] = [
  { id: "appuntamenti", label: "Uno", content: <p>contenuto uno</p> },
  { id: "ricette", label: "Due", content: <p>contenuto due</p> },
];

describe("Tabs", () => {
  beforeEach(() => {
    window.location.hash = "";
  });

  it("selects the first tab by default", () => {
    render(<Tabs tabs={tabs} />);
    expect(screen.getByRole("tab", { name: "Uno" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByRole("tabpanel")).toHaveTextContent("contenuto uno");
  });

  it("selects the tab named by the hash", () => {
    window.location.hash = "#/ricette";
    render(<Tabs tabs={tabs} />);
    expect(screen.getByRole("tab", { name: "Due" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("selects the tab of a deep link and keeps the focus in the hash", () => {
    window.location.hash = "#/ricette/0900A0000000001";
    render(<Tabs tabs={tabs} />);
    expect(screen.getByRole("tab", { name: "Due" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(window.location.hash).toBe("#/ricette/0900A0000000001");
  });

  it("clicking a tab sets the plain hash", async () => {
    const user = userEvent.setup();
    window.location.hash = "#/ricette/0900A0000000001";
    render(<Tabs tabs={tabs} />);
    await user.click(screen.getByRole("tab", { name: "Uno" }));
    expect(window.location.hash).toBe("#/appuntamenti");
  });

  it("switches on click and updates the hash", async () => {
    const user = userEvent.setup();
    render(<Tabs tabs={tabs} />);
    await user.click(screen.getByRole("tab", { name: "Due" }));
    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "Due" })).toHaveAttribute(
        "aria-selected",
        "true",
      );
    });
    expect(window.location.hash).toBe("#/ricette");
    expect(screen.getByRole("tabpanel")).toHaveTextContent("contenuto due");
  });

  it("moves with arrow keys, wrapping around", async () => {
    const user = userEvent.setup();
    render(<Tabs tabs={tabs} />);
    screen.getByRole("tab", { name: "Uno" }).focus();

    await user.keyboard("{ArrowRight}");
    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "Due" })).toHaveAttribute(
        "aria-selected",
        "true",
      );
    });
    expect(screen.getByRole("tab", { name: "Due" })).toHaveFocus();
    expect(window.location.hash).toBe("#/ricette");

    await user.keyboard("{ArrowRight}");
    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "Uno" })).toHaveAttribute(
        "aria-selected",
        "true",
      );
    });
    expect(window.location.hash).toBe("#/appuntamenti");

    await user.keyboard("{ArrowLeft}");
    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "Due" })).toHaveAttribute(
        "aria-selected",
        "true",
      );
    });
  });

  it("jumps with Home and End", async () => {
    const user = userEvent.setup();
    render(<Tabs tabs={tabs} />);
    screen.getByRole("tab", { name: "Uno" }).focus();

    await user.keyboard("{End}");
    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "Due" })).toHaveAttribute(
        "aria-selected",
        "true",
      );
    });

    await user.keyboard("{Home}");
    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "Uno" })).toHaveAttribute(
        "aria-selected",
        "true",
      );
    });
  });
});
