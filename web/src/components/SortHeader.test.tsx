import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { SortState } from "../lib/useSort";
import { SortHeader } from "./SortHeader";

type Key = "date" | "name";

function makeSort(key: Key, dir: "asc" | "desc"): SortState<Key> {
  return {
    key,
    dir,
    toggle: vi.fn(),
    ariaSort: (k) => {
      if (k !== key) return "none";
      return dir === "asc" ? "ascending" : "descending";
    },
  };
}

function renderHeader(sort: SortState<Key>, sortKey: Key): void {
  render(
    <table>
      <thead>
        <tr>
          <SortHeader label="Data" sortKey={sortKey} sort={sort} />
        </tr>
      </thead>
    </table>,
  );
}

describe("SortHeader", () => {
  it("renders a column header with a button", () => {
    renderHeader(makeSort("name", "asc"), "date");
    expect(screen.getByRole("columnheader", { name: "Data" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Data" })).toBeVisible();
  });

  it("calls toggle with its key on click", async () => {
    const sort = makeSort("name", "asc");
    renderHeader(sort, "date");
    await userEvent.click(screen.getByRole("button", { name: "Data" }));
    expect(sort.toggle).toHaveBeenCalledWith("date");
  });

  it("reflects aria-sort", () => {
    renderHeader(makeSort("date", "desc"), "date");
    expect(screen.getByRole("columnheader")).toHaveAttribute(
      "aria-sort",
      "descending",
    );
  });

  it("shows the indicator only on the active column", () => {
    const { container } = render(
      <table>
        <thead>
          <tr>
            <SortHeader
              label="Data"
              sortKey="date"
              sort={makeSort("date", "asc")}
            />
            <SortHeader
              label="Nome"
              sortKey="name"
              sort={makeSort("date", "asc")}
            />
          </tr>
        </thead>
      </table>,
    );
    const glyphs = container.querySelectorAll('[aria-hidden="true"]');
    expect(glyphs).toHaveLength(1);
    expect(glyphs[0]).toHaveTextContent("▲");
  });
});
