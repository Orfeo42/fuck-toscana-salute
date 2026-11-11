import { describe, expect, it } from "vitest";

import type { RowBody } from "../../api/client";
import {
  countDrugs,
  drugRows,
  drugStatusLabel,
  drugStatusTone,
  filterDrugs,
  matchesSearch,
  sortDrugs,
  type DrugRowData,
} from "./filter";

function drug(
  nre: string,
  description: string,
  status: number,
  date?: string,
): DrugRowData {
  return {
    nre,
    fse: {
      id: nre,
      nre,
      description,
      kind: "drug",
      status,
      documentRef: "r",
      ...(date === undefined ? {} : { date }),
    },
  };
}

const a = drug("0900A0000000001", "Farmaco alfa", 3, "2026-11-01T09:00:00Z");
const b = drug("0900A0000000002", "Farmaco beta", 8, "2026-11-03T09:00:00Z");
const c = drug("0900A0000000003", "Farmaco gamma", 3);
const d = drug("0900A0000000004", "Farmaco delta", 3, "2026-11-01T09:00:00Z");

function nres(rows: readonly DrugRowData[]): string[] {
  return rows.map((row) => row.nre);
}

describe("drugStatusLabel", () => {
  it("labels status 3 as Da ritirare", () => {
    expect(drugStatusLabel(3)).toBe("Da ritirare");
  });

  it("falls back to the shared prescription label", () => {
    expect(drugStatusLabel(8)).toBe("Erogata");
    expect(drugStatusLabel(99)).toBe("Stato 99");
  });
});

describe("drugStatusTone", () => {
  it("maps known statuses", () => {
    expect(drugStatusTone(3)).toBe("tobook");
    expect(drugStatusTone(8)).toBe("done");
    expect(drugStatusTone(9)).toBe("done");
    expect(drugStatusTone(7)).toBe("pending");
    expect(drugStatusTone(4)).toBe("closed");
  });

  it("maps unknown statuses to closed", () => {
    expect(drugStatusTone(99)).toBe("closed");
  });
});

describe("drugRows", () => {
  it("keeps only drug rows with fse", () => {
    const rows: RowBody[] = [
      { nre: a.nre, fse: a.fse },
      { nre: "v", fse: { ...a.fse, kind: "visit" } },
      { nre: "c" },
    ];

    expect(nres(drugRows(rows))).toEqual([a.nre]);
  });
});

describe("matchesSearch", () => {
  it("matches description case-insensitively", () => {
    expect(matchesSearch(a, "ALFA")).toBe(true);
  });

  it("matches NRE", () => {
    expect(matchesSearch(a, "0000001")).toBe(true);
  });

  it("matches everything on blank search", () => {
    expect(matchesSearch(a, "  ")).toBe(true);
  });

  it("rejects a non matching term", () => {
    expect(matchesSearch(a, "zzz")).toBe(false);
  });
});

describe("filterDrugs", () => {
  it("keeps status 3 for pickup", () => {
    expect(nres(filterDrugs([a, b, c], "pickup", ""))).toEqual([a.nre, c.nre]);
  });

  it("keeps all for all", () => {
    expect(filterDrugs([a, b, c], "all", "")).toHaveLength(3);
  });

  it("applies search", () => {
    expect(nres(filterDrugs([a, b, c], "all", "beta"))).toEqual([b.nre]);
  });
});

describe("countDrugs", () => {
  it("counts per filter honoring search", () => {
    expect(countDrugs([a, b, c], "")).toEqual({ pickup: 2, all: 3 });
    expect(countDrugs([a, b, c], "alfa")).toEqual({ pickup: 1, all: 1 });
  });
});

describe("sortDrugs", () => {
  it("sorts by date descending with undefined last and NRE tie-break", () => {
    expect(nres(sortDrugs([c, a, b, d], "data", "desc"))).toEqual([
      b.nre,
      a.nre,
      d.nre,
      c.nre,
    ]);
  });

  it("sorts by date ascending", () => {
    expect(nres(sortDrugs([b, a], "data", "asc"))).toEqual([a.nre, b.nre]);
  });

  it("sorts by farmaco", () => {
    expect(nres(sortDrugs([c, a, b], "farmaco", "asc"))).toEqual([
      a.nre,
      b.nre,
      c.nre,
    ]);
  });

  it("sorts by stato label", () => {
    expect(nres(sortDrugs([a, b], "stato", "asc"))).toEqual([a.nre, b.nre]);
  });

  it("sorts by nre descending", () => {
    expect(nres(sortDrugs([a, b, c], "nre", "desc"))).toEqual([
      c.nre,
      b.nre,
      a.nre,
    ]);
  });

  it("does not mutate the input", () => {
    const input = [b, a];
    sortDrugs(input, "nre", "asc");
    expect(nres(input)).toEqual([b.nre, a.nre]);
  });
});
