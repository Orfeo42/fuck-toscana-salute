import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ReportRowBody } from "../../api/client";
import { render } from "../../test/render";
import { LocalReportEditor } from "./LocalReportEditor";

const mocks = vi.hoisted(() => ({ put: vi.fn(), del: vi.fn() }));

vi.mock("../../api/client", () => ({
  api: { PUT: mocks.put, DELETE: mocks.del },
}));

const row: ReportRowBody = {
  id: "local|L1",
  kind: "local",
  kindLabel: "Caricato",
  title: "Ecografia addome",
  customTitle: "Eco addome personale",
  ref: { kind: "local", abstractId: "L1", detailId: "", repCode: "" },
  date: "2026-10-04T22:00:00Z",
  editDate: "2026-10-05",
  provider: "Studio Prova",
  unread: false,
  hidden: false,
  origin: "local",
  visits: [],
  linkedLabel: "",
};

function renderEditor(): { onChanged: () => void } {
  const onChanged = vi.fn();
  render(<LocalReportEditor row={row} onChanged={onChanged} />);
  return { onChanged };
}

describe("LocalReportEditor", () => {
  beforeEach(() => {
    mocks.put.mockReset();
    mocks.del.mockReset();
    mocks.put.mockResolvedValue({ response: { ok: true } });
    mocks.del.mockResolvedValue({ response: { ok: true } });
  });

  it("prefills title, date and provider from the row", () => {
    renderEditor();

    expect(screen.getByLabelText("Titolo")).toHaveValue("Eco addome personale");
    expect(screen.getByLabelText("Data")).toHaveValue("2026-10-05");
    expect(screen.getByRole("combobox", { name: "Struttura" })).toHaveValue(
      "Studio Prova",
    );
  });

  it("saves with the row's own id", async () => {
    const { onChanged } = renderEditor();

    await userEvent.click(
      screen.getByRole("button", { name: "Salva modifiche" }),
    );

    await waitFor(() => {
      expect(onChanged).toHaveBeenCalledTimes(1);
    });
    expect(mocks.put).toHaveBeenCalledWith("/api/reports/local", {
      body: {
        id: "L1",
        title: "Eco addome personale",
        date: "2026-10-05",
        provider: "Studio Prova",
      },
    });
  });

  it("requires a title", async () => {
    renderEditor();

    await userEvent.clear(screen.getByLabelText("Titolo"));
    await userEvent.click(
      screen.getByRole("button", { name: "Salva modifiche" }),
    );

    expect(await screen.findByText("Il titolo è obbligatorio")).toBeVisible();
    expect(mocks.put).not.toHaveBeenCalled();
  });

  it("shows an error when saving fails", async () => {
    mocks.put.mockResolvedValue({ response: { ok: false } });
    const { onChanged } = renderEditor();

    await userEvent.click(
      screen.getByRole("button", { name: "Salva modifiche" }),
    );

    expect(await screen.findByText("Salvataggio non riuscito")).toBeVisible();
    expect(onChanged).not.toHaveBeenCalled();
  });

  it("deletes only after the in-app confirmation", async () => {
    const { onChanged } = renderEditor();

    await userEvent.click(
      screen.getByRole("button", { name: "Elimina referto" }),
    );
    expect(await screen.findByText("Eliminare il referto?")).toBeVisible();
    expect(mocks.del).not.toHaveBeenCalled();

    await userEvent.click(screen.getByRole("button", { name: "Elimina" }));

    await waitFor(() => {
      expect(onChanged).toHaveBeenCalledTimes(1);
    });
    expect(mocks.del).toHaveBeenCalledWith("/api/reports/local", {
      body: { id: "L1" },
    });
  });

  it("shows an error when deleting fails", async () => {
    mocks.del.mockResolvedValue({ response: { ok: false } });
    const { onChanged } = renderEditor();

    await userEvent.click(
      screen.getByRole("button", { name: "Elimina referto" }),
    );
    await userEvent.click(
      await screen.findByRole("button", { name: "Elimina" }),
    );

    expect(await screen.findByText("Eliminazione non riuscita")).toBeVisible();
    expect(onChanged).not.toHaveBeenCalled();
  });
});
