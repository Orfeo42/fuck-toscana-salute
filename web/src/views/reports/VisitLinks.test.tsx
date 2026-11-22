import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ReportRefBody, RowBody } from "../../api/client";
import { render } from "../../test/render";
import { VisitLinks } from "./VisitLinks";

const mocks = vi.hoisted(() => ({
  open: vi.fn(),
  reload: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
}));

vi.mock("../../data/useHealthData", () => ({
  useHealthData: () => ({
    prescriptions: {
      data: { prescriptions: rows, updated: null },
      reload: mocks.reload,
    },
    appointments: { data: undefined, reload: vi.fn() },
  }),
}));

vi.mock("../../api/client", () => ({
  api: { PUT: mocks.put, DELETE: mocks.del },
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
    <VisitLinks
      reportRef={reportRef}
      nres={nres}
      onLinked={() => undefined}
      onUnlinked={() => undefined}
    />,
  );
}

describe("VisitLinks", () => {
  beforeEach(() => {
    mocks.open.mockReset();
    mocks.reload.mockReset();
    mocks.put.mockReset();
    mocks.del.mockReset();
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

  it("reloads prescriptions after a successful unlink", async () => {
    mocks.del.mockResolvedValue({
      response: new Response(null, { status: 200 }),
    });
    renderLinks(["0123A"]);
    await userEvent.click(
      screen.getByRole("button", { name: "Scollega 0123A" }),
    );
    expect(mocks.reload).toHaveBeenCalledTimes(1);
  });

  it("does not reload prescriptions after a failed unlink", async () => {
    mocks.del.mockResolvedValue({
      response: new Response(null, { status: 500 }),
    });
    renderLinks(["0123A"]);
    await userEvent.click(
      screen.getByRole("button", { name: "Scollega 0123A" }),
    );
    expect(mocks.reload).not.toHaveBeenCalled();
  });

  it("reloads prescriptions after a successful link", async () => {
    mocks.put.mockResolvedValue({
      response: new Response(null, { status: 200 }),
    });
    renderLinks([]);
    await userEvent.click(
      screen.getByRole("combobox", { name: "Collega a visita" }),
    );
    await userEvent.click(await screen.findByText(/0123A/));
    expect(mocks.reload).toHaveBeenCalledTimes(1);
  });

  it("does not reload prescriptions after a failed link", async () => {
    mocks.put.mockResolvedValue({
      response: new Response(null, { status: 500 }),
    });
    renderLinks([]);
    await userEvent.click(
      screen.getByRole("combobox", { name: "Collega a visita" }),
    );
    await userEvent.click(await screen.findByText(/0123A/));
    expect(mocks.reload).not.toHaveBeenCalled();
  });
});
