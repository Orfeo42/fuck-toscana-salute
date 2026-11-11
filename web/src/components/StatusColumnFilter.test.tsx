import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { render } from "../test/render";
import { StatusColumnFilter } from "./StatusColumnFilter";

const options = [
  { value: "a", label: "Prenotata", count: 2 },
  { value: "b", label: "Scaduta", count: 1 },
];

describe("StatusColumnFilter", () => {
  it("lists one checkbox per option with its count", () => {
    render(
      <StatusColumnFilter
        options={options}
        selected={["a"]}
        onChange={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("checkbox", { name: "Prenotata (2)" }),
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Scaduta (1)" }),
    ).not.toBeChecked();
  });

  it("reports the new selection when ticking a box", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <StatusColumnFilter
        options={options}
        selected={["a"]}
        onChange={onChange}
      />,
    );

    await user.click(screen.getByRole("checkbox", { name: "Scaduta (1)" }));

    expect(onChange).toHaveBeenCalledWith(["a", "b"]);
  });

  it("selects every option with Tutti", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <StatusColumnFilter
        options={options}
        selected={[]}
        onChange={onChange}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Tutti" }));

    expect(onChange).toHaveBeenCalledWith(["a", "b"]);
  });

  it("clears the selection with Nessuno", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <StatusColumnFilter
        options={options}
        selected={["a", "b"]}
        onChange={onChange}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Nessuno" }));

    expect(onChange).toHaveBeenCalledWith([]);
  });
});
