import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { DetailDrawer } from "./DetailDrawer";
import { render } from "../test/render";

const tabs = [
  { value: "one", label: "Primo", content: <p>Contenuto primo</p> },
  {
    value: "two",
    label: "Secondo",
    count: 3,
    content: <p>Contenuto secondo</p>,
  },
];

describe("DetailDrawer", () => {
  it("shows title, summary, actions and the first tab", () => {
    render(
      <DetailDrawer
        title="Visita di prova"
        summary={<span>Riepilogo</span>}
        actions={<button type="button">Azione</button>}
        tabs={tabs}
        onClose={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("dialog", { name: "Visita di prova" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Riepilogo")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Azione" })).toBeInTheDocument();
    expect(screen.getByText("Contenuto primo")).toBeInTheDocument();
    expect(screen.getByText("Contenuto secondo")).not.toBeVisible();
  });

  it("switches tab and shows the count", async () => {
    const user = userEvent.setup();
    render(<DetailDrawer title="Prova" tabs={tabs} onClose={vi.fn()} />);

    const second = screen.getByRole("tab", { name: /Secondo/ });
    expect(second).toHaveTextContent("3");
    await user.click(second);

    expect(screen.getByText("Contenuto secondo")).toBeVisible();
    expect(screen.getByText("Contenuto primo")).not.toBeVisible();
  });

  it("hides the tab list with a single tab", () => {
    render(
      <DetailDrawer title="Prova" tabs={tabs.slice(0, 1)} onClose={vi.fn()} />,
    );

    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
    expect(screen.getByText("Contenuto primo")).toBeInTheDocument();
  });

  it("calls onClose from the close button", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<DetailDrawer title="Prova" tabs={tabs} onClose={onClose} />);

    await user.click(screen.getByRole("button", { name: "Chiudi" }));

    expect(onClose).toHaveBeenCalled();
  });
});
