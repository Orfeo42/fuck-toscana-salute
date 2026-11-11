import { describe, expect, it } from "vitest";

import type { RowBody } from "../../api/client";
import {
  countByStatus,
  filterRows,
  rowDate,
  rowTitle,
  sortRows,
  type Criteria,
} from "./filter";

const booked: RowBody = {
  nre: "0900A0000000001",
  fse: {
    id: "f1",
    nre: "0900A0000000001",
    description: "Visita esempio",
    kind: "visit",
    status: 30,
    date: "2026-11-04T09:00:00Z",
    documentRef: "ref-1",
  },
};

const closedDrug: RowBody = {
  nre: "0900A0000000002",
  fse: {
    id: "f2",
    nre: "0900A0000000002",
    description: "Farmaco esempio",
    kind: "drug",
    status: 8,
    date: "2026-10-01T09:00:00Z",
    documentRef: "ref-2",
  },
};

const cupOnly: RowBody = {
  nre: "0900A0000000003",
  cup: {
    nre: "0900A0000000003",
    kind: "visit",
    status: 3,
    items: ["Ecografia esempio", "Esame sangue"],
    prescribedOn: "2026-11-05T09:00:00Z",
  },
};

const nullItems: RowBody = {
  nre: "0900A0000000004",
  cup: { nre: "0900A0000000004", kind: "visit", status: 3, items: null },
};

const rows = [booked, closedDrug, cupOnly, nullItems];

const base: Criteria = { status: "all", kind: "all", search: "" };

describe("filterRows", () => {
  it("bookable keeps rows with cup", () => {
    const got = filterRows(rows, { ...base, status: "bookable" });
    expect(got).toEqual([cupOnly, nullItems]);
  });

  it("open keeps non closed fse and cup only rows", () => {
    const got = filterRows(rows, { ...base, status: "open" });
    expect(got).toEqual([booked, cupOnly, nullItems]);
  });

  it("all keeps everything", () => {
    expect(filterRows(rows, base)).toEqual(rows);
  });

  it("searches the description case-insensitively", () => {
    const got = filterRows(rows, { ...base, search: "FARMACO" });
    expect(got).toEqual([closedDrug]);
  });

  it("searches the cup items", () => {
    const got = filterRows(rows, { ...base, search: "ecografia" });
    expect(got).toEqual([cupOnly]);
  });

  it("searches the NRE", () => {
    const got = filterRows(rows, { ...base, search: "0900a0000000001" });
    expect(got).toEqual([booked]);
  });

  it("ignores a blank search", () => {
    expect(filterRows(rows, { ...base, search: "   " })).toEqual(rows);
  });

  it("kind visit keeps visits and cup rows", () => {
    const got = filterRows(rows, { ...base, kind: "visit" });
    expect(got).toEqual([booked, cupOnly, nullItems]);
  });

  it("kind drug keeps drugs only", () => {
    const got = filterRows(rows, { ...base, kind: "drug" });
    expect(got).toEqual([closedDrug]);
  });

  it("handles null cup items", () => {
    const got = filterRows([nullItems], { ...base, search: "x" });
    expect(got).toEqual([]);
  });
});

describe("countByStatus", () => {
  it("counts each status filter", () => {
    expect(countByStatus(rows, "all", "")).toEqual({
      bookable: 2,
      open: 3,
      all: 4,
    });
  });

  it("applies kind and search before counting", () => {
    expect(countByStatus(rows, "visit", "esempio")).toEqual({
      bookable: 1,
      open: 2,
      all: 2,
    });
  });
});

describe("sortRows", () => {
  it("sorts by date descending using the cup date as fallback", () => {
    expect(sortRows([closedDrug, booked, cupOnly])).toEqual([
      cupOnly,
      booked,
      closedDrug,
    ]);
  });

  it("puts undated rows last", () => {
    expect(sortRows([nullItems, closedDrug, cupOnly])).toEqual([
      cupOnly,
      closedDrug,
      nullItems,
    ]);
  });

  it("does not mutate the input", () => {
    const input = [closedDrug, booked];
    sortRows(input);
    expect(input).toEqual([closedDrug, booked]);
  });
});

describe("rowDate", () => {
  it("prefers the fse date over the cup date", () => {
    const row: RowBody = {
      ...booked,
      cup: {
        nre: booked.nre,
        kind: "visit",
        status: 3,
        items: null,
        prescribedOn: "2026-12-01T09:00:00Z",
      },
    };
    expect(rowDate(row)).toBe("2026-11-04T09:00:00Z");
  });

  it("falls back to the cup date", () => {
    expect(rowDate(cupOnly)).toBe("2026-11-05T09:00:00Z");
  });
});

describe("rowTitle", () => {
  it("uses the fse description", () => {
    expect(rowTitle(booked)).toBe("Visita esempio");
  });

  it("joins the cup items", () => {
    expect(rowTitle(cupOnly)).toBe("Ecografia esempio, Esame sangue");
  });

  it("falls back to a generic title for null items", () => {
    expect(rowTitle(nullItems)).toBe("Prescrizione");
  });
});
