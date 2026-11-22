import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { JSX } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  LinkedReportBody,
  ReportOptionBody,
  ReportRefBody,
  VisitRowBody,
  VisitRowResponseBody,
} from "../../api/client";
import { render } from "../../test/render";
import {
  appointment,
  bookedRow,
  nre1,
  nre2,
  rowResponse,
  unit,
  visitRow,
} from "../../test/visits";
import { WithSession } from "../../test/WithSession";
import type { UploadReportDialogProps } from "../reports/UploadReportDialog";
import { VisitDetailModal } from "./VisitDetailModal";

const mocks = vi.hoisted(() => ({
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
  upload: vi.fn(),
  startRefresh: vi.fn(),
}));

vi.mock("../../api/client", () => ({
  api: { POST: mocks.post, PUT: mocks.put, DELETE: mocks.del },
  prescriptionPdfUrl: (ref: string) => `/api/documents/prescription?ref=${ref}`,
  bookingPdfUrl: (id: string) => `/api/documents/booking?id=${id}`,
}));

vi.mock("../../data/refresh", () => ({ startRefresh: mocks.startRefresh }));

vi.mock("../../lib/pdf", () => ({
  loadPdf: () => ({
    promise: new Promise<never>(() => undefined),
    destroy: () => Promise.resolve(),
  }),
}));

vi.mock("../reports/UploadReportDialog", () => ({
  UploadReportDialog: (props: UploadReportDialogProps): JSX.Element | null => {
    mocks.upload(props);
    return props.opened ? <p>upload aperto</p> : null;
  },
}));

vi.mock("../reports/ReportDetailModal", () => ({
  ReportDetailModal: ({
    reportRef,
  }: {
    reportRef: ReportRefBody;
  }): JSX.Element => <p>{`dettaglio ${reportRef.abstractId}`}</p>,
}));

const refA: ReportRefBody = {
  kind: "lab",
  abstractId: "A1",
  detailId: "D1",
  repCode: "R1",
};
const refB: ReportRefBody = {
  kind: "lab",
  abstractId: "B2",
  detailId: "D2",
  repCode: "R2",
};

function linked(
  ref: ReportRefBody,
  extra: Partial<LinkedReportBody> = {},
): LinkedReportBody {
  return {
    ref,
    label: `Esame ${ref.abstractId}`,
    date: "2026-11-05T09:00:00Z",
    provider: "Laboratorio Esempio",
    origin: "remote",
    available: true,
    ...extra,
  };
}

const option: ReportOptionBody = {
  ref: refB,
  label: "Referto libero",
  date: "2026-11-06T09:00:00Z",
  provider: "Studio Esempio",
  origin: "remote",
};

let rowData: VisitRowResponseBody | undefined;
let rowStatus = 200;
let options: ReportOptionBody[] = [];

function show(row: VisitRowBody, extra: Partial<VisitRowResponseBody> = {}) {
  rowData = rowResponse(row, extra);
}

function renderModal(
  onClose: () => void = vi.fn(),
  onChanged: () => void = vi.fn(),
  visitNre: string = nre1,
): void {
  render(
    <WithSession>
      <VisitDetailModal
        nre={visitNre}
        onClose={onClose}
        onChanged={onChanged}
      />
    </WithSession>,
  );
}

function rowCalls(): number {
  return mocks.post.mock.calls.filter((call) => call[0] === "/api/visits/row")
    .length;
}

describe("VisitDetailModal", () => {
  beforeEach(() => {
    mocks.post.mockReset();
    mocks.put.mockReset();
    mocks.del.mockReset();
    mocks.upload.mockReset();
    mocks.startRefresh.mockReset();
    mocks.startRefresh.mockResolvedValue(true);
    mocks.put.mockResolvedValue({ response: { ok: true } });
    mocks.del.mockResolvedValue({ response: { ok: true } });
    rowStatus = 200;
    options = [];
    show(visitRow());
    mocks.post.mockImplementation((path: string) => {
      if (path === "/api/visits/report-options") {
        return Promise.resolve({
          data: { options },
          response: { status: 200 },
        });
      }
      return Promise.resolve({
        data: rowStatus === 200 ? rowData : undefined,
        response: { status: rowStatus },
      });
    });
  });

  it("posts the NRE to the row endpoint and shows a loader first", async () => {
    renderModal();

    expect(screen.getByLabelText("Carico la visita")).toBeInTheDocument();
    expect(
      await screen.findByRole("dialog", { name: "Visita esempio" }),
    ).toBeInTheDocument();
    expect(mocks.post).toHaveBeenCalledWith(
      "/api/visits/row",
      expect.objectContaining({ body: { nre: nre1 } }),
    );
  });

  it("shows a not found message", async () => {
    rowStatus = 404;
    renderModal();

    expect(await screen.findByText("Visita non trovata.")).toBeInTheDocument();
  });

  it("shows a failure message", async () => {
    rowStatus = 500;
    renderModal();

    expect(
      await screen.findByText("Impossibile caricare la visita."),
    ).toBeInTheDocument();
  });

  it("shows title, badge, prescription date and the status details", async () => {
    show(
      visitRow({
        status: {
          ...visitRow().status,
          details: ["Scade il 2 gen 2027"],
        },
      }),
    );
    renderModal();

    const dialog = await screen.findByRole("dialog", {
      name: "Visita esempio",
    });
    expect(within(dialog).getAllByText("Da prenotare")).not.toHaveLength(0);
    expect(within(dialog).getByText(/^Prescritta il/)).toBeInTheDocument();
    expect(within(dialog).getByText("Scade il 2 gen 2027")).toBeInTheDocument();
    expect(
      within(dialog).getByRole("link", { name: "Prenota sul CUP" }),
    ).toBeInTheDocument();
    expect(within(dialog).queryByText("Appuntamento")).not.toBeInTheDocument();
  });

  it("hides the booking action when the status cannot be booked", async () => {
    show(visitRow({ status: { ...visitRow().status, canBook: false } }));
    renderModal();

    await screen.findByRole("dialog", { name: "Visita esempio" });

    expect(
      screen.queryByRole("link", { name: "Prenota sul CUP" }),
    ).not.toBeInTheDocument();
  });

  it("renders the appointment, relative day, unit and map links", async () => {
    show(bookedRow());
    renderModal(vi.fn(), vi.fn(), nre2);

    const dialog = await screen.findByRole("dialog", {
      name: "Controllo esempio",
    });
    expect(within(dialog).getAllByText(/tra 3 giorni/)).not.toHaveLength(0);
    expect(within(dialog).getAllByText("Ospedale Esempio")).not.toHaveLength(0);
    expect(within(dialog).getByText("Piano 2")).toBeInTheDocument();
    expect(
      within(dialog).getByText("Via Finta 1, 50100 Firenze, FI"),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("link", { name: "Indicazioni stradali" }),
    ).toHaveAttribute("href", "https://maps.example.com/dir?q=finta");
    expect(
      within(dialog).getByRole("link", { name: "Apri in Google Maps" }),
    ).toHaveAttribute("href", "https://maps.example.com/search?q=finta");
    expect(
      within(dialog).getByRole("button", {
        name: "Apri promemoria prenotazione (PDF)",
      }),
    ).toBeInTheDocument();
  });

  it("hides the map links when the unit has no address", async () => {
    show(
      bookedRow({ appointment: appointment({ unit: unit({ address: "" }) }) }),
    );
    renderModal(vi.fn(), vi.fn(), nre2);

    await screen.findByRole("dialog", { name: "Controllo esempio" });

    expect(
      screen.queryByRole("link", { name: "Indicazioni stradali" }),
    ).not.toBeInTheDocument();
  });

  it("offers the calendar only for an upcoming appointment", async () => {
    show(bookedRow({ appointment: appointment({ upcoming: false }) }));
    renderModal(vi.fn(), vi.fn(), nre2);

    await screen.findByRole("dialog", { name: "Controllo esempio" });

    expect(
      screen.queryByRole("button", { name: "Aggiungi a Google Calendar" }),
    ).not.toBeInTheDocument();
  });

  it("shows the calendar action for an upcoming appointment", async () => {
    show(bookedRow());
    renderModal(vi.fn(), vi.fn(), nre2);

    expect(
      await screen.findByRole("button", { name: "Aggiungi a Google Calendar" }),
    ).toBeInTheDocument();
  });

  it("shows the appointment with a manual status", async () => {
    show(
      bookedRow({
        status: { ...bookedRow().status, manual: true, label: "Erogata" },
      }),
      {
        manualStatus: { current: 8, options: [{ code: 8, label: "Erogata" }] },
      },
    );
    renderModal(vi.fn(), vi.fn(), nre2);

    const dialog = await screen.findByRole("dialog", {
      name: "Controllo esempio",
    });

    expect(
      within(dialog).getByRole("img", { name: "Stato impostato a mano" }),
    ).toBeInTheDocument();
    expect(within(dialog).getByText("Appuntamento")).toBeInTheDocument();
    expect(within(dialog).getAllByText("Ospedale Esempio")).not.toHaveLength(0);
    expect(
      within(dialog).getByRole("button", {
        name: "Usa lo stato di Toscana Salute",
      }),
    ).toBeInTheDocument();
  });

  it("shows no reset when there is no manual status", async () => {
    renderModal();

    await screen.findByRole("dialog", { name: "Visita esempio" });

    expect(
      screen.queryByRole("button", { name: "Usa lo stato di Toscana Salute" }),
    ).not.toBeInTheDocument();
  });

  it("sets the manual status, reloads the row and notifies the view", async () => {
    const user = userEvent.setup();
    const onChanged = vi.fn();
    renderModal(vi.fn(), onChanged);
    await screen.findByRole("dialog", { name: "Visita esempio" });
    const before = rowCalls();

    await user.click(
      screen.getByRole("combobox", { name: "Imposta stato a mano" }),
    );
    await user.click(await screen.findByRole("option", { name: "Erogata" }));

    expect(mocks.put).toHaveBeenCalledWith("/api/visits/status", {
      body: { nre: nre1, status: 8 },
    });
    await waitFor(() => {
      expect(rowCalls()).toBeGreaterThan(before);
    });
    expect(onChanged).toHaveBeenCalledTimes(1);
  });

  it("resets the manual status", async () => {
    const user = userEvent.setup();
    show(visitRow({ status: { ...visitRow().status, manual: true } }), {
      manualStatus: { current: 8, options: [{ code: 8, label: "Erogata" }] },
    });
    renderModal();
    await screen.findByRole("dialog", { name: "Visita esempio" });

    await user.click(
      screen.getByRole("button", { name: "Usa lo stato di Toscana Salute" }),
    );

    expect(mocks.del).toHaveBeenCalledWith("/api/visits/status", {
      body: { nre: nre1 },
    });
  });

  it("reports a failed save and does not notify the view", async () => {
    const user = userEvent.setup();
    const onChanged = vi.fn();
    mocks.put.mockResolvedValue({ response: { ok: false } });
    renderModal(vi.fn(), onChanged);
    await screen.findByRole("dialog", { name: "Visita esempio" });

    await user.click(
      screen.getByRole("combobox", { name: "Imposta stato a mano" }),
    );
    await user.click(await screen.findByRole("option", { name: "Erogata" }));

    expect(
      await screen.findByText("Impossibile salvare lo stato"),
    ).toBeInTheDocument();
    expect(onChanged).not.toHaveBeenCalled();
  });

  it("counts, lists and marks the linked reports", async () => {
    show(
      visitRow({
        reports: [linked(refA), linked(refB, { origin: "local" })],
      }),
    );
    const user = userEvent.setup();
    renderModal();
    const dialog = await screen.findByRole("dialog", {
      name: "Visita esempio",
    });

    expect(
      within(dialog).getByRole("tab", { name: /Referti/ }),
    ).toHaveTextContent("2");
    await user.click(within(dialog).getByRole("tab", { name: /Referti/ }));

    expect(
      within(dialog).getByText(/Esame A1 · .* · Laboratorio Esempio/),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("img", { name: "Caricato da te" }),
    ).toBeInTheDocument();
  });

  it("opens a linked report", async () => {
    show(visitRow({ reports: [linked(refA)] }));
    const user = userEvent.setup();
    renderModal();
    await screen.findByRole("dialog", { name: "Visita esempio" });
    await user.click(screen.getByRole("tab", { name: /Referti/ }));

    await user.click(
      screen.getByRole("button", { name: "Apri referto Esame A1" }),
    );

    expect(screen.getByText("dettaglio A1")).toBeInTheDocument();
  });

  it("disables opening an unavailable report", async () => {
    show(visitRow({ reports: [linked(refA, { available: false })] }));
    const user = userEvent.setup();
    renderModal();
    await screen.findByRole("dialog", { name: "Visita esempio" });
    await user.click(screen.getByRole("tab", { name: /Referti/ }));

    expect(
      screen.getByRole("button", { name: "Apri referto Esame A1" }),
    ).toBeDisabled();
  });

  it("unlinks a report with its ref and reloads", async () => {
    show(visitRow({ reports: [linked(refA)] }));
    const user = userEvent.setup();
    const onChanged = vi.fn();
    renderModal(vi.fn(), onChanged);
    await screen.findByRole("dialog", { name: "Visita esempio" });
    await user.click(screen.getByRole("tab", { name: /Referti/ }));

    await user.click(
      screen.getByRole("button", { name: "Scollega referto Esame A1" }),
    );

    expect(mocks.del).toHaveBeenCalledWith("/api/links", {
      body: { nre: nre1, report: refA },
    });
    await waitFor(() => {
      expect(onChanged).toHaveBeenCalledTimes(1);
    });
  });

  it("links a report chosen among the options from the backend", async () => {
    options = [option];
    const user = userEvent.setup();
    const onChanged = vi.fn();
    renderModal(vi.fn(), onChanged);
    await screen.findByRole("dialog", { name: "Visita esempio" });
    await user.click(screen.getByRole("tab", { name: /Referti/ }));

    await user.click(screen.getByRole("combobox", { name: "Collega referto" }));
    await user.click(
      await screen.findByRole("option", { name: /Referto libero/ }),
    );

    expect(mocks.post).toHaveBeenCalledWith(
      "/api/visits/report-options",
      expect.objectContaining({ body: { nre: nre1 } }),
    );
    expect(mocks.put).toHaveBeenCalledWith("/api/links", {
      body: { nre: nre1, report: refB },
    });
    await waitFor(() => {
      expect(onChanged).toHaveBeenCalledTimes(1);
    });
  });

  it("opens the upload dialog with the defaults of the row", async () => {
    show(
      visitRow({
        upload: {
          title: "Visita esempio",
          date: "2026-12-01",
          provider: "Ospedale Esempio",
        },
      }),
    );
    const user = userEvent.setup();
    renderModal();
    await screen.findByRole("dialog", { name: "Visita esempio" });
    await user.click(screen.getByRole("tab", { name: /Referti/ }));
    expect(mocks.upload).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Carica referto" }));

    expect(screen.getByText("upload aperto")).toBeInTheDocument();
    expect(mocks.upload).toHaveBeenLastCalledWith(
      expect.objectContaining({
        nre: nre1,
        defaultTitle: "Visita esempio",
        defaultDate: "2026-12-01",
        defaultProvider: "Ospedale Esempio",
      }),
    );
  });

  it("starts a refresh for this NRE and reloads the row and the view", async () => {
    const user = userEvent.setup();
    const onChanged = vi.fn();
    renderModal(vi.fn(), onChanged);
    const dialog = await screen.findByRole("dialog", {
      name: "Visita esempio",
    });
    const before = rowCalls();

    await user.click(within(dialog).getByRole("button", { name: "Aggiorna" }));

    expect(mocks.startRefresh).toHaveBeenCalledWith({ nres: [nre1] });
    await waitFor(() => {
      expect(rowCalls()).toBeGreaterThan(before);
    });
    expect(onChanged).toHaveBeenCalledTimes(1);
  });

  it("notifies a failed refresh", async () => {
    const user = userEvent.setup();
    mocks.startRefresh.mockResolvedValue(false);
    renderModal();
    const dialog = await screen.findByRole("dialog", {
      name: "Visita esempio",
    });

    await user.click(within(dialog).getByRole("button", { name: "Aggiorna" }));

    expect(
      await screen.findByText("Impossibile avviare l'aggiornamento"),
    ).toBeInTheDocument();
  });

  it("shows the ricetta action and the NRE on the prescription tab", async () => {
    const user = userEvent.setup();
    renderModal();
    const dialog = await screen.findByRole("dialog", {
      name: "Visita esempio",
    });

    await user.click(within(dialog).getByRole("tab", { name: "Prescrizione" }));

    expect(
      within(dialog).getByRole("button", { name: "Apri ricetta (PDF)" }),
    ).toBeInTheDocument();
    expect(within(dialog).getByText(nre1)).toBeInTheDocument();
  });

  it("shows the check times in the footer", async () => {
    show(
      visitRow({
        checkedAt: {
          appointments: "2026-11-10T08:30:00Z",
          verdict: "2026-11-09T07:15:00Z",
        },
      }),
    );
    renderModal();

    const dialog = await screen.findByRole("dialog", {
      name: "Visita esempio",
    });

    expect(
      within(dialog).getByText(/Appuntamenti: 10\/11 09:30/),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText(/Verifica CUP: 09\/11 08:15/),
    ).toBeInTheDocument();
  });

  it("omits the footer when nothing was checked", async () => {
    renderModal();

    await screen.findByRole("dialog", { name: "Visita esempio" });

    expect(screen.queryByText(/Aggiornamenti —/)).not.toBeInTheDocument();
  });
});
