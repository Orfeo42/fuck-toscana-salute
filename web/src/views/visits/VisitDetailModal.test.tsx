import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { JSX } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AppointmentBody, ReportBody, RowBody } from "../../api/client";
import type { UploadReportDialogProps } from "../reports/UploadReportDialog";
import { render } from "../../test/render";
import { effectiveState } from "./filter";
import { VisitDetailModal } from "./VisitDetailModal";

const mocks = vi.hoisted(() => ({
  put: vi.fn(),
  del: vi.fn(),
  upload: vi.fn(),
}));

vi.mock("../../api/client", () => ({
  api: { PUT: mocks.put, DELETE: mocks.del },
  prescriptionPdfUrl: (ref: string) => `/api/documents/prescription?ref=${ref}`,
  bookingPdfUrl: (id: string) => `/api/documents/booking?id=${id}`,
}));

vi.mock("../reports/UploadReportDialog", () => ({
  UploadReportDialog: (props: UploadReportDialogProps): JSX.Element | null => {
    mocks.upload(props);
    return props.opened ? <p>upload aperto</p> : null;
  },
}));

vi.mock("../reports/ReportDetailModal", () => ({
  ReportDetailModal: ({ report }: { report: ReportBody }): JSX.Element => (
    <p>{`dettaglio ${report.abstractId}`}</p>
  ),
}));

const now = new Date("2026-11-10T09:00:00Z");
const nre = "0900A0000000001";

function visit(extra: Partial<RowBody> = {}): RowBody {
  return {
    nre,
    overridden: false,
    reports: [],
    fse: {
      id: "f1",
      nre,
      description: "Visita esempio",
      kind: "visit",
      status: 3,
      date: "2026-11-04T09:00:00Z",
      documentRef: "",
    },
    ...extra,
  };
}

function report(
  abstractId: string,
  origin: ReportBody["origin"] = "remote",
  title?: string,
): ReportBody {
  return {
    kind: origin === "local" ? "local" : "lab",
    kindLabel: "Laboratorio",
    abstractId,
    detailId: "d1",
    repCode: "r1",
    date: "2026-11-02T09:00:00Z",
    provider: "Ospedale esempio",
    unread: false,
    hidden: false,
    origin,
    nres: [],
    ...(title === undefined ? {} : { title }),
  };
}

const booking: AppointmentBody = {
  id: "a1",
  nre,
  status: "PRENOTATO",
  at: "2026-12-01T09:00:00Z",
  unit: {
    name: "Unita esempio",
    description: "",
    address: "Via Esempio 1",
    postalCode: "50100",
    city: "Firenze",
    province: "FI",
  },
};

interface Options {
  row?: RowBody;
  reports?: ReportBody[];
  booked?: boolean;
}

function renderModal(options: Options = {}): { onChanged: () => void } {
  const row = options.row ?? visit();
  const onChanged = vi.fn();
  const bookings = new Map(options.booked === true ? [[nre, booking]] : []);
  const state = effectiveState(row, bookings, new Set(), now);
  render(
    <VisitDetailModal
      row={row}
      state={state}
      booking={options.booked === true ? booking : undefined}
      now={now}
      appointmentCheckedAt={undefined}
      refreshing={false}
      reports={options.reports ?? []}
      onChanged={onChanged}
      onRefresh={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  return { onChanged };
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.put.mockResolvedValue({ response: { ok: true } });
  mocks.del.mockResolvedValue({ response: { ok: true } });
});

describe("VisitDetailModal status", () => {
  it("shows no manual marker and no reset when not overridden", () => {
    renderModal();
    expect(
      screen.queryByRole("img", { name: "Stato impostato a mano" }),
    ).toBeNull();
    expect(
      screen.queryByRole("button", { name: "Usa lo stato di Toscana Salute" }),
    ).toBeNull();
  });

  it("shows the manual marker and the override status when overridden", () => {
    const fse = visit().fse;
    renderModal({
      row: visit({
        overridden: true,
        ...(fse === undefined ? {} : { fse: { ...fse, status: 8 } }),
      }),
    });
    expect(
      screen.getByRole("img", { name: "Stato impostato a mano" }),
    ).toBeVisible();
    expect(
      screen.getByRole("combobox", { name: "Imposta stato a mano" }),
    ).toHaveValue("Erogata");
  });

  it("sets the local status", async () => {
    const user = userEvent.setup();
    const { onChanged } = renderModal();
    await user.click(
      screen.getByRole("combobox", { name: "Imposta stato a mano" }),
    );
    await user.click(await screen.findByRole("option", { name: "Erogata" }));
    await waitFor(() => {
      expect(mocks.put).toHaveBeenCalledWith("/api/visits/status", {
        body: { nre, status: 8 },
      });
    });
    await waitFor(() => {
      expect(onChanged).toHaveBeenCalledTimes(1);
    });
  });

  it("resets to the remote status", async () => {
    const user = userEvent.setup();
    const { onChanged } = renderModal({ row: visit({ overridden: true }) });
    await user.click(
      screen.getByRole("button", { name: "Usa lo stato di Toscana Salute" }),
    );
    await waitFor(() => {
      expect(mocks.del).toHaveBeenCalledWith("/api/visits/status", {
        body: { nre },
      });
    });
    await waitFor(() => {
      expect(onChanged).toHaveBeenCalledTimes(1);
    });
  });

  it("reports a failed save and does not reload", async () => {
    mocks.put.mockResolvedValue({ response: { ok: false } });
    const user = userEvent.setup();
    const { onChanged } = renderModal();
    await user.click(
      screen.getByRole("combobox", { name: "Imposta stato a mano" }),
    );
    await user.click(await screen.findByRole("option", { name: "Erogata" }));
    expect(
      await screen.findByText("Impossibile salvare lo stato"),
    ).toBeVisible();
    expect(onChanged).not.toHaveBeenCalled();
  });
});

async function openReports(): Promise<void> {
  await userEvent.click(screen.getByRole("tab", { name: /Referti/ }));
}

describe("VisitDetailModal reports", () => {
  const linkedRef = {
    kind: "lab",
    abstractId: "a1",
    detailId: "d1",
    repCode: "r1",
  };

  it("lists linked reports and marks local ones", async () => {
    const localRef = {
      kind: "local",
      abstractId: "l1",
      detailId: "d1",
      repCode: "r1",
    };
    renderModal({
      row: visit({ reports: [linkedRef, localRef] }),
      reports: [
        report("a1", "remote", "Esami del sangue"),
        report("l1", "local"),
      ],
    });
    await openReports();
    expect(screen.getByText(/Esami del sangue/)).toBeVisible();
    expect(screen.getAllByRole("img", { name: "Caricato da te" })).toHaveLength(
      1,
    );
  });

  it("opens a linked report", async () => {
    const user = userEvent.setup();
    renderModal({
      row: visit({ reports: [linkedRef] }),
      reports: [report("a1")],
    });
    await openReports();
    await user.click(
      screen.getByRole("button", { name: "Apri referto Laboratorio" }),
    );
    expect(await screen.findByText("dettaglio a1")).toBeVisible();
  });

  it("unlinks a report", async () => {
    const user = userEvent.setup();
    const { onChanged } = renderModal({
      row: visit({ reports: [linkedRef] }),
      reports: [report("a1")],
    });
    await openReports();
    await user.click(
      screen.getByRole("button", { name: "Scollega referto Laboratorio" }),
    );
    await waitFor(() => {
      expect(mocks.del).toHaveBeenCalledWith("/api/links", {
        body: { nre, report: linkedRef },
      });
    });
    await waitFor(() => {
      expect(onChanged).toHaveBeenCalledTimes(1);
    });
  });

  it("links an unlinked report chosen from the searchable list", async () => {
    const user = userEvent.setup();
    const { onChanged } = renderModal({
      row: visit({ reports: [linkedRef] }),
      reports: [report("a1"), report("a2", "remote", "Radiografia")],
    });
    await openReports();
    await user.click(screen.getByRole("combobox", { name: "Collega referto" }));
    expect(screen.queryByRole("option", { name: /Laboratorio/ })).toBeNull();
    await user.click(
      await screen.findByRole("option", { name: /Radiografia/ }),
    );
    await waitFor(() => {
      expect(mocks.put).toHaveBeenCalledWith("/api/links", {
        body: {
          nre,
          report: { ...linkedRef, abstractId: "a2" },
        },
      });
    });
    await waitFor(() => {
      expect(onChanged).toHaveBeenCalledTimes(1);
    });
  });

  it("opens the upload dialog with today's date when not booked", async () => {
    const user = userEvent.setup();
    renderModal();
    await openReports();
    await user.click(screen.getByRole("button", { name: "Carica referto" }));
    expect(await screen.findByText("upload aperto")).toBeVisible();
    expect(mocks.upload).toHaveBeenLastCalledWith(
      expect.objectContaining({
        opened: true,
        nre,
        defaultTitle: "Visita esempio",
        defaultDate: "2026-11-10",
      }),
    );
    expect(mocks.upload.mock.lastCall?.[0]).not.toHaveProperty(
      "defaultProvider",
    );
  });

  it("defaults the upload date and provider from the appointment", async () => {
    const user = userEvent.setup();
    renderModal({ booked: true });
    await openReports();
    await user.click(screen.getByRole("button", { name: "Carica referto" }));
    expect(mocks.upload).toHaveBeenLastCalledWith(
      expect.objectContaining({
        defaultDate: "2026-12-01",
        defaultProvider: "Unita esempio",
      }),
    );
  });
});

describe("VisitDetailModal reports tab", () => {
  const refs = ["a", "b", "c", "d", "e", "f", "g"].map((id) => ({
    kind: "lab",
    abstractId: id,
    detailId: "d1",
    repCode: "r1",
  }));

  it("caps the linked list and shows the count", async () => {
    renderModal({
      row: visit({ reports: refs }),
      reports: refs.map((ref) => report(ref.abstractId)),
    });
    expect(screen.getByRole("tab", { name: /Referti/ })).toHaveTextContent("7");
    await openReports();
    expect(
      screen.getAllByRole("button", { name: /Apri referto/ }),
    ).toHaveLength(5);
    expect(
      screen.getByRole("button", { name: "Mostra altri 2" }),
    ).toBeVisible();
  });

  it("does not mount the upload dialog until opened", async () => {
    const user = userEvent.setup();
    renderModal();
    await openReports();
    expect(mocks.upload).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Carica referto" }));
    expect(mocks.upload).toHaveBeenCalled();
  });
});

describe("VisitDetailModal layout", () => {
  it("shows title, badge and the prescription date when not booked", () => {
    renderModal();
    expect(
      screen.getByRole("dialog", { name: "Visita esempio" }),
    ).toBeVisible();
    expect(screen.getByText("Prescritta il", { exact: false })).toBeVisible();
    expect(screen.getByRole("button", { name: "Aggiorna" })).toBeVisible();
  });

  it("calls onRefresh from the actions", async () => {
    const user = userEvent.setup();
    const onRefresh = vi.fn();
    const row = visit();
    render(
      <VisitDetailModal
        row={row}
        state={effectiveState(row, new Map(), new Set(), now)}
        booking={undefined}
        now={now}
        appointmentCheckedAt={undefined}
        refreshing={false}
        reports={[]}
        onChanged={vi.fn()}
        onRefresh={onRefresh}
        onClose={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Aggiorna" }));
    expect(onRefresh).toHaveBeenCalledTimes(1);
  });

  it("shows the booked summary with map links", () => {
    renderModal({ booked: true });
    expect(screen.getByText(/· tra \d+ giorni|· domani|· oggi/)).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Indicazioni stradali" }),
    ).toHaveAttribute("href", expect.stringContaining("google"));
    expect(
      screen.getByRole("link", { name: "Apri in Google Maps" }),
    ).toHaveAttribute("target", "_blank");
    expect(screen.getAllByText("Unita esempio").length).toBeGreaterThan(0);
  });

  it("shows the NRE copy button on the prescription tab", async () => {
    const user = userEvent.setup();
    renderModal();
    await user.click(screen.getByRole("tab", { name: "Prescrizione" }));
    expect(
      screen.getByRole("button", { name: `Copia NRE ${nre}` }),
    ).toBeVisible();
  });
});
