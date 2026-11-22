import { DataTable } from "mantine-datatable";
import { describe, expect, it } from "vitest";

import { render } from "../test/render";
import { tableScrollProps } from "./tableScroll";

describe("tableScrollProps", () => {
  it("caps the scroll viewport height", () => {
    const records = Array.from({ length: 50 }, (_, i) => ({
      id: i,
      name: `row ${String(i)}`,
    }));
    const { container } = render(
      <DataTable
        {...tableScrollProps}
        records={records}
        columns={[{ accessor: "name" }]}
      />,
    );
    const viewport = container.querySelector<HTMLElement>(
      ".mantine-ScrollArea-viewport",
    );
    expect(viewport?.style.maxHeight).toBe("calc(-2px + 75vh)");
  });
});
