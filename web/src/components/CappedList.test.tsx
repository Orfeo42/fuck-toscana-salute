import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { CappedList } from "./CappedList";
import { render } from "../test/render";

const items = ["a", "b", "c", "d", "e", "f", "g"];

function renderList(list: readonly string[]): void {
  render(
    <CappedList
      items={list}
      getKey={(item) => item}
      renderItem={(item) => <span>{`voce ${item}`}</span>}
    />,
  );
}

describe("CappedList", () => {
  it("shows every item up to the limit without a toggle", () => {
    renderList(items.slice(0, 5));

    expect(screen.getAllByText(/^voce /)).toHaveLength(5);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("caps longer lists and expands and collapses", async () => {
    const user = userEvent.setup();
    renderList(items);

    expect(screen.getAllByText(/^voce /)).toHaveLength(5);
    await user.click(screen.getByRole("button", { name: "Mostra altri 2" }));
    expect(screen.getAllByText(/^voce /)).toHaveLength(7);
    await user.click(screen.getByRole("button", { name: "Mostra meno" }));
    expect(screen.getAllByText(/^voce /)).toHaveLength(5);
  });
});
