import { describe, expect, it } from "vitest";

import type { ReportBody, RowBody } from "../../api/client";
import { linkedReports, reportKey, reportLabel } from "./reportLinks";

function report(abstractId: string, title?: string): ReportBody {
  return {
    kind: "lab",
    kindLabel: "Laboratorio",
    abstractId,
    detailId: "d1",
    repCode: "r1",
    provider: "Ospedale esempio",
    unread: false,
    hidden: false,
    origin: "remote",
    nres: [],
    ...(title === undefined ? {} : { title }),
  };
}

function rowWith(reports: RowBody["reports"]): RowBody {
  return { nre: "0900A0000000001", overridden: false, reports };
}

describe("reportKey", () => {
  it("joins the four identifying fields", () => {
    expect(reportKey(report("a1"))).toBe("lab|a1|d1|r1");
  });
});

describe("linkedReports", () => {
  it("returns only the referenced reports", () => {
    const all = [report("a1"), report("a2"), report("a3")];
    const row = rowWith([
      { kind: "lab", abstractId: "a2", detailId: "d1", repCode: "r1" },
    ]);
    expect(linkedReports(row, all)).toEqual([all[1]]);
  });

  it("returns nothing when the row has no references", () => {
    expect(linkedReports(rowWith(null), [report("a1")])).toEqual([]);
  });

  it("ignores references missing from the loaded reports", () => {
    const row = rowWith([
      { kind: "lab", abstractId: "zz", detailId: "d1", repCode: "r1" },
    ]);
    expect(linkedReports(row, [report("a1")])).toEqual([]);
  });
});

describe("reportLabel", () => {
  it("prefers the title", () => {
    expect(reportLabel(report("a1", "Esami del sangue"))).toBe(
      "Esami del sangue",
    );
  });

  it("falls back to the kind label", () => {
    expect(reportLabel(report("a1"))).toBe("Laboratorio");
  });
});
