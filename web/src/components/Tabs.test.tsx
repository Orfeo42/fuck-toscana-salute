import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";

import { Tabs, type TabItem } from "./Tabs";

const tabs: readonly TabItem[] = [
  { id: "uno", label: "Uno", content: <p>contenuto uno</p> },
  { id: "due", label: "Due", content: <p>contenuto due</p> },
  { id: "tre", label: "Tre", content: <p>contenuto tre</p> },
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
    window.location.hash = "#/due";
    render(<Tabs tabs={tabs} />);
    expect(screen.getByRole("tab", { name: "Due" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
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
    expect(window.location.hash).toBe("#/due");
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
    expect(window.location.hash).toBe("#/due");

    await user.keyboard("{ArrowLeft}{ArrowLeft}");
    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "Tre" })).toHaveAttribute(
        "aria-selected",
        "true",
      );
    });
    expect(window.location.hash).toBe("#/tre");
  });

  it("jumps with Home and End", async () => {
    const user = userEvent.setup();
    render(<Tabs tabs={tabs} />);
    screen.getByRole("tab", { name: "Uno" }).focus();

    await user.keyboard("{End}");
    await waitFor(() => {
      expect(screen.getByRole("tab", { name: "Tre" })).toHaveAttribute(
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
