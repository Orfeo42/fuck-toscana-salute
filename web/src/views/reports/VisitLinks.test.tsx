import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ReportRefBody, RowBody } from "../../api/client";
import { render } from "../../test/render";
import { WithHealth } from "../../test/WithHealth";
import { VisitLinks } from "./VisitLinks";

const mocks = vi.hoisted(() => ({
  open: vi.fn(),
}));

vi.mock("../../api/client", () => ({
  api: { PUT: vi.fn(), DELETE: vi.fn() },
}));

vi.mock("../../lib/openVisit", () => ({
  requestOpenVisit: mocks.open,
}));

const reportRef: ReportRefBody = {
  kind: "lab",
  abstractId: "a1",
  detailId: "d1",
  repCode: "r1",
};

const rows: RowBody[] = [
  {
    nre: "0123A",
    overridden: false,
    reports: null,
    fse: {
      id: "p1",
      nre: "0123A",
      description: "Ecografia addome",
      kind: "visit",
      status: 3,
      documentRef: "",
    },
  },
];

function renderLinks(nres: readonly string[]): void {
  render(
    <WithHealth rows={rows}>
      <VisitLinks
        reportRef={reportRef}
        nres={nres}
        onLinked={() => undefined}
        onUnlinked={() => undefined}
      />
    </WithHealth>,
  );
}

describe("VisitLinks", () => {
  beforeEach(() => {
    mocks.open.mockReset();
  });

  it("opens the visit from the title", async () => {
    renderLinks(["0123A"]);
    await userEvent.click(
      screen.getByRole("button", { name: "Ecografia addome" }),
    );
    expect(mocks.open).toHaveBeenCalledWith("0123A");
  });

  it("opens the visit from the icon", async () => {
    renderLinks(["0123A"]);
    await userEvent.click(screen.getByRole("button", { name: "Apri visita" }));
    expect(mocks.open).toHaveBeenCalledWith("0123A");
  });

  it("opens a visit that is not loaded", async () => {
    renderLinks(["0999Z"]);
    await userEvent.click(screen.getByRole("button", { name: "0999Z" }));
    expect(mocks.open).toHaveBeenCalledWith("0999Z");
  });

  it("never puts the NRE in a link", () => {
    renderLinks(["0123A"]);
    for (const link of screen.queryAllByRole("link")) {
      expect(link.getAttribute("href") ?? "").not.toContain("0123A");
    }
  });
});
