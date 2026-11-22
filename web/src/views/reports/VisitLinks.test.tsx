import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { LinkedVisitBody, ReportRefBody } from "../../api/client";
import { render } from "../../test/render";
import { VisitLinks } from "./VisitLinks";

const mocks = vi.hoisted(() => ({
  open: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
}));

vi.mock("../../api/client", () => ({
  api: { POST: mocks.post, PUT: mocks.put, DELETE: mocks.del },
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

const linked: LinkedVisitBody = { nre: "0123A", title: "Ecografia addome" };
const free: LinkedVisitBody = { nre: "0456B", title: "Visita cardiologica" };

function renderLinks(visits: readonly LinkedVisitBody[]): {
  onChanged: () => void;
} {
  const onChanged = vi.fn();
  render(
    <VisitLinks reportRef={reportRef} visits={visits} onChanged={onChanged} />,
  );
  return { onChanged };
}

describe("VisitLinks", () => {
  beforeEach(() => {
    mocks.open.mockReset();
    mocks.post.mockReset();
    mocks.put.mockReset();
    mocks.del.mockReset();
    mocks.post.mockResolvedValue({
      data: { options: [free] },
      response: { ok: true },
    });
  });

  it("shows the linked visits from the row", () => {
    renderLinks([linked]);

    expect(
      screen.getByRole("button", { name: "Ecografia addome" }),
    ).toBeVisible();
    expect(screen.getByText("0123A")).toBeVisible();
  });

  it("says when there are no linked visits", () => {
    renderLinks([]);

    expect(screen.getByText("Nessuna visita collegata.")).toBeVisible();
  });

  it("opens the visit from the title and from the icon", async () => {
    renderLinks([linked]);

    await userEvent.click(
      screen.getByRole("button", { name: "Ecografia addome" }),
    );
    await userEvent.click(screen.getByRole("button", { name: "Apri visita" }));

    expect(mocks.open).toHaveBeenNthCalledWith(1, "0123A");
    expect(mocks.open).toHaveBeenNthCalledWith(2, "0123A");
  });

  it("never puts the NRE in a link", () => {
    renderLinks([linked]);

    for (const link of screen.queryAllByRole("link")) {
      expect(link.getAttribute("href") ?? "").not.toContain("0123A");
    }
  });

  it("loads the options from the backend with the report ref", async () => {
    renderLinks([]);

    await waitFor(() => {
      expect(mocks.post).toHaveBeenCalledWith(
        "/api/reports/visit-options",
        expect.objectContaining({ body: { ref: reportRef } }),
      );
    });
  });

  it("links an offered visit and notifies the parent", async () => {
    mocks.put.mockResolvedValue({ response: { ok: true } });
    const { onChanged } = renderLinks([]);

    await userEvent.click(
      screen.getByRole("combobox", { name: "Collega a visita" }),
    );
    await userEvent.click(
      await screen.findByRole("option", {
        name: "Visita cardiologica · 0456B",
      }),
    );

    await waitFor(() => {
      expect(onChanged).toHaveBeenCalledTimes(1);
    });
    expect(mocks.put).toHaveBeenCalledWith("/api/links", {
      body: { nre: "0456B", report: reportRef },
    });
  });

  it("unlinks a visit and notifies the parent", async () => {
    mocks.del.mockResolvedValue({ response: { ok: true } });
    const { onChanged } = renderLinks([linked]);

    await userEvent.click(
      screen.getByRole("button", { name: "Scollega 0123A" }),
    );

    await waitFor(() => {
      expect(onChanged).toHaveBeenCalledTimes(1);
    });
    expect(mocks.del).toHaveBeenCalledWith("/api/links", {
      body: { nre: "0123A", report: reportRef },
    });
  });

  it("shows an error and does not notify when unlinking fails", async () => {
    mocks.del.mockResolvedValue({ response: { ok: false } });
    const { onChanged } = renderLinks([linked]);

    await userEvent.click(
      screen.getByRole("button", { name: "Scollega 0123A" }),
    );

    expect(await screen.findByText("Operazione non riuscita")).toBeVisible();
    expect(onChanged).not.toHaveBeenCalled();
  });

  it("shows an error and does not notify when linking fails", async () => {
    mocks.put.mockResolvedValue({ response: { ok: false } });
    const { onChanged } = renderLinks([]);

    await userEvent.click(
      screen.getByRole("combobox", { name: "Collega a visita" }),
    );
    await userEvent.click(
      await screen.findByRole("option", {
        name: "Visita cardiologica · 0456B",
      }),
    );

    expect(await screen.findByText("Operazione non riuscita")).toBeVisible();
    expect(onChanged).not.toHaveBeenCalled();
  });
});
