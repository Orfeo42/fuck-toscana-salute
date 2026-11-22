import { DataTable } from "mantine-datatable";
import { describe, expect, it } from "vitest";

import { render } from "../test/render";
import { tableScrollProps } from "./tableScroll";

describe("tableScrollProps", () => {
  it("lets the scroll viewport shrink above the pagination", () => {
    const records = Array.from({ length: 50 }, (_, i) => ({
      id: i,
      name: `row ${String(i)}`,
    }));
    const { container } = render(
      <DataTable
        {...tableScrollProps}
        records={records}
        columns={[{ accessor: "name" }]}
        totalRecords={100}
        recordsPerPage={50}
        page={1}
        onPageChange={() => undefined}
      />,
    );
    const scrollArea = container.querySelector<HTMLElement>(
      ".mantine-datatable-scroll-area",
    );
    const viewport = container.querySelector<HTMLElement>(
      ".mantine-ScrollArea-viewport",
    );
    expect(scrollArea?.style.display).toBe("flex");
    expect(scrollArea?.style.flexDirection).toBe("column");
    expect(viewport?.style.flex).toBe("1 1 auto");
    expect(viewport?.style.minHeight).toBe("0px");
    expect(viewport?.style.maxHeight).toBe("");
  });
});
