import { describe, expect, it } from "vitest";

import type { ReportBody } from "../../api/client";
import {
  filterByKind,
  kindLabels,
  reportId,
  searchReports,
  sortReports,
} from "./filter";

function report(
  kindLabel: string,
  provider: string,
  date?: string,
): ReportBody {
  return {
    kind: kindLabel.toLowerCase(),
    kindLabel,
    abstractId: "a",
    detailId: "d",
    repCode: "r",
    provider,
    unread: false,
    hidden: false,
    ...(date === undefined ? {} : { date }),
  };
}

const lab = report("Laboratorio", "Ospedale Esempio", "2026-10-01T09:00:00Z");
const radio = report("Radiologia", "Clinica Prova", "2026-11-01T09:00:00Z");
const undated = report("Laboratorio", "Clinica Prova");

describe("searchReports", () => {
  it("returns everything for a blank query", () => {
    expect(searchReports([lab, radio], "  ")).toEqual([lab, radio]);
  });

  it("matches the provider case-insensitively", () => {
    expect(searchReports([lab, radio], "ospedale")).toEqual([lab]);
  });

  it("matches the kind label", () => {
    expect(searchReports([lab, radio], "radio")).toEqual([radio]);
  });
});

describe("kindLabels", () => {
  it("returns distinct sorted labels", () => {
    expect(kindLabels([radio, lab, undated])).toEqual([
      "Laboratorio",
      "Radiologia",
    ]);
  });
});

describe("filterByKind", () => {
  it("keeps everything for an empty kind", () => {
    expect(filterByKind([lab, radio], "")).toEqual([lab, radio]);
  });

  it("keeps only the chosen kind", () => {
    expect(filterByKind([lab, radio], "Radiologia")).toEqual([radio]);
  });
});

describe("sortReports", () => {
  it("sorts by date descending with undated last", () => {
    expect(sortReports([undated, lab, radio], "date", "desc")).toEqual([
      radio,
      lab,
      undated,
    ]);
  });

  it("sorts by provider ascending", () => {
    expect(sortReports([lab, radio], "provider", "asc")).toEqual([radio, lab]);
  });
});

describe("reportId", () => {
  it("joins the identifying fields", () => {
    expect(reportId(lab)).toBe("laboratorio|a|d|r");
  });
});
