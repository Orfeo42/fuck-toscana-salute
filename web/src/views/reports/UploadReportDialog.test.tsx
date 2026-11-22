import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ReportBody } from "../../api/client";
import { GoogleContext } from "../../google/useGoogle";
import { render } from "../../test/render";
import { UploadReportDialog } from "./UploadReportDialog";

const mocks = vi.hoisted(() => ({
  post: vi.fn(),
  pick: vi.fn(),
}));

vi.mock("../../api/client", () => ({
  api: { POST: mocks.post },
}));

vi.mock("../../lib/googlePicker", () => ({
  pickDriveFile: mocks.pick,
}));

const created: ReportBody = {
  kind: "local",
  kindLabel: "Caricato",
  abstractId: "L1",
  detailId: "",
  repCode: "",
  provider: "Clinica Prova",
  title: "Eco",
  unread: false,
  hidden: false,
  origin: "local",
  nres: null,
};

interface Options {
  connected?: boolean;
  drive?: boolean;
  signIn?: () => Promise<boolean>;
  refresh?: () => void;
  nre?: string;
  defaults?: boolean;
}

function renderDialog(options: Options = {}): {
  onClose: () => void;
  onUploaded: (report: ReportBody) => void;
} {
  const onClose = vi.fn();
  const onUploaded = vi.fn();
  render(
    <GoogleContext
      value={{
        status: {
          configured: true,
          connected: options.connected ?? false,
          clientId: "123456789012-abc.apps.googleusercontent.com",
          drive: options.drive ?? true,
        },
        loading: false,
        error: undefined,
        refresh: options.refresh ?? vi.fn(),
        signIn: options.signIn ?? vi.fn().mockResolvedValue(true),
        disconnect: vi.fn(),
        openSettings: vi.fn(),
      }}
    >
      <UploadReportDialog
        opened
        onClose={onClose}
        onUploaded={onUploaded}
        {...(options.nre === undefined ? {} : { nre: options.nre })}
        {...(options.defaults === true
          ? {
              defaultTitle: "Eco addome",
              defaultDate: "2026-10-01",
              defaultProvider: "Ospedale Esempio",
            }
          : {})}
      />
    </GoogleContext>,
  );
  return { onClose, onUploaded };
}

function pdf(name = "eco.pdf", size = 10): File {
  const file = new File(["x"], name, { type: "application/pdf" });
  Object.defineProperty(file, "size", { value: size });
  return file;
}

function sentForm(): FormData {
  const options: unknown = mocks.post.mock.calls[0]?.[1];
  if (
    typeof options === "object" &&
    options !== null &&
    "bodySerializer" in options &&
    typeof options.bodySerializer === "function"
  ) {
    const form: unknown = Reflect.apply(options.bodySerializer, undefined, []);
    if (form instanceof FormData) {
      return form;
    }
  }
  throw new Error("no form sent");
}

async function chooseFile(file: File): Promise<void> {
  const input = document.querySelector<HTMLInputElement>('input[type="file"]');
  if (input === null) {
    throw new Error("missing file input");
  }
  await userEvent.upload(input, file, { applyAccept: false });
}

describe("UploadReportDialog", () => {
  beforeEach(() => {
    mocks.post.mockReset();
    mocks.pick.mockReset();
  });

  it("disables Carica until a file is chosen", () => {
    renderDialog();

    expect(screen.getByRole("button", { name: "Carica" })).toBeDisabled();
  });

  it("prefills the defaults", () => {
    renderDialog({ defaults: true });

    expect(screen.getByLabelText("Titolo")).toHaveValue("Eco addome");
    expect(screen.getByLabelText("Data")).toHaveValue("2026-10-01");
    expect(screen.getByLabelText("Struttura")).toHaveValue("Ospedale Esempio");
  });

  it("uploads the file with the metadata and the nre as multipart", async () => {
    mocks.post.mockResolvedValue({ data: created, response: { status: 201 } });
    const { onClose, onUploaded } = renderDialog({ nre: "0123A" });

    await chooseFile(pdf());
    await userEvent.type(screen.getByLabelText("Titolo"), "Eco");
    await userEvent.type(screen.getByLabelText("Data"), "2026-10-02");
    await userEvent.type(screen.getByLabelText("Struttura"), "Clinica Prova");
    await userEvent.click(screen.getByRole("button", { name: "Carica" }));

    await waitFor(() => {
      expect(onUploaded).toHaveBeenCalledWith(created);
    });
    expect(onClose).toHaveBeenCalled();
    expect(mocks.post.mock.calls[0]?.[0]).toBe("/api/reports/local");
    const form = sentForm();
    const sentFile = form.get("file");
    expect(sentFile instanceof File ? sentFile.name : "").toBe("eco.pdf");
    expect(form.get("title")).toBe("Eco");
    expect(form.get("date")).toBe("2026-10-02");
    expect(form.get("provider")).toBe("Clinica Prova");
    expect(form.get("nre")).toBe("0123A");
  });

  it("omits empty metadata and the nre when not given", async () => {
    mocks.post.mockResolvedValue({ data: created, response: { status: 201 } });
    renderDialog();

    await chooseFile(pdf());
    await userEvent.click(screen.getByRole("button", { name: "Carica" }));

    await waitFor(() => {
      expect(mocks.post).toHaveBeenCalled();
    });
    const form = sentForm();
    expect([...form.keys()]).toEqual(["file"]);
  });

  it("rejects a file over 20 MB without calling the server", async () => {
    renderDialog();

    await chooseFile(pdf("grande.pdf", 21 * 1024 * 1024));

    expect(await screen.findByText("Il file supera i 20 MB")).toBeVisible();
    expect(screen.getByRole("button", { name: "Carica" })).toBeDisabled();
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("shows the 413 message", async () => {
    mocks.post.mockResolvedValue({
      data: undefined,
      response: { status: 413 },
    });
    renderDialog();

    await chooseFile(pdf());
    await userEvent.click(screen.getByRole("button", { name: "Carica" }));

    expect(await screen.findByText("Il file supera i 20 MB")).toBeVisible();
  });

  it("shows the 422 message", async () => {
    mocks.post.mockResolvedValue({
      data: undefined,
      response: { status: 422 },
    });
    renderDialog();

    await chooseFile(pdf());
    await userEvent.click(screen.getByRole("button", { name: "Carica" }));

    expect(
      await screen.findByText("Formato non supportato o file non valido"),
    ).toBeVisible();
  });

  it("shows a generic message when the upload throws", async () => {
    mocks.post.mockRejectedValue(new Error("down"));
    renderDialog();

    await chooseFile(pdf());
    await userEvent.click(screen.getByRole("button", { name: "Carica" }));

    expect(await screen.findByText("Caricamento non riuscito")).toBeVisible();
  });

  it("shows the Drive button when configured with drive and disconnected", () => {
    renderDialog({ connected: false, drive: true });

    expect(
      screen.getByRole("button", { name: "Scegli da Google Drive" }),
    ).toBeVisible();
  });

  it("signs in before the picker request when disconnected", async () => {
    const order: string[] = [];
    const signIn = vi.fn().mockImplementation(() => {
      order.push("signIn");
      return Promise.resolve(true);
    });
    mocks.post.mockImplementation((path: string) => {
      order.push(path);
      return Promise.resolve({ data: undefined, response: { status: 409 } });
    });
    renderDialog({ connected: false, signIn });

    await userEvent.click(
      screen.getByRole("button", { name: "Scegli da Google Drive" }),
    );

    await waitFor(() => {
      expect(order).toEqual(["signIn", "/api/google/picker"]);
    });
  });

  it("does not request the picker when the sign-in fails", async () => {
    const signIn = vi.fn().mockResolvedValue(false);
    renderDialog({ connected: false, signIn });

    await userEvent.click(
      screen.getByRole("button", { name: "Scegli da Google Drive" }),
    );

    await waitFor(() => {
      expect(signIn).toHaveBeenCalled();
    });
    expect(mocks.post).not.toHaveBeenCalled();
  });

  it("hides the Drive button when the Picker API key is missing", () => {
    renderDialog({ connected: false, drive: false });

    expect(
      screen.queryByRole("button", { name: "Scegli da Google Drive" }),
    ).toBeNull();
  });

  it("imports the picked Drive file with the metadata", async () => {
    const credentials = { accessToken: "t", apiKey: "k", appId: "1" };
    mocks.post.mockImplementation((path: string) =>
      Promise.resolve(
        path === "/api/google/picker"
          ? { data: credentials, response: { status: 200 } }
          : { data: created, response: { status: 201 } },
      ),
    );
    mocks.pick.mockResolvedValue({ id: "drive1", name: "eco.pdf" });
    const { onUploaded, onClose } = renderDialog({
      connected: true,
      nre: "0123A",
    });

    await userEvent.type(screen.getByLabelText("Titolo"), "Eco");
    await userEvent.click(
      screen.getByRole("button", { name: "Scegli da Google Drive" }),
    );

    await waitFor(() => {
      expect(onUploaded).toHaveBeenCalledWith(created);
    });
    expect(onClose).toHaveBeenCalled();
    expect(mocks.pick).toHaveBeenCalledWith(credentials);
    expect(mocks.post).toHaveBeenCalledWith("/api/reports/local/drive", {
      body: { fileId: "drive1", title: "Eco", nre: "0123A" },
    });
  });

  it("does nothing when the picker is cancelled", async () => {
    mocks.post.mockResolvedValue({
      data: { accessToken: "t", apiKey: "k", appId: "1" },
      response: { status: 200 },
    });
    mocks.pick.mockResolvedValue(null);
    const { onUploaded } = renderDialog({ connected: true });

    await userEvent.click(
      screen.getByRole("button", { name: "Scegli da Google Drive" }),
    );

    await waitFor(() => {
      expect(mocks.pick).toHaveBeenCalled();
    });
    expect(mocks.post).toHaveBeenCalledTimes(1);
    expect(onUploaded).not.toHaveBeenCalled();
  });

  it("explains a picker that is not configured", async () => {
    mocks.post.mockResolvedValue({
      data: undefined,
      response: { status: 409 },
    });
    renderDialog({ connected: true });

    await userEvent.click(
      screen.getByRole("button", { name: "Scegli da Google Drive" }),
    );

    expect(
      await screen.findByText("Aggiungi la API key in Impostazioni Google"),
    ).toBeVisible();
    expect(mocks.pick).not.toHaveBeenCalled();
  });

  it("refreshes and asks to retry on 401", async () => {
    const refresh = vi.fn();
    mocks.post.mockResolvedValue({
      data: undefined,
      response: { status: 401 },
    });
    renderDialog({ connected: true, refresh });

    await userEvent.click(
      screen.getByRole("button", { name: "Scegli da Google Drive" }),
    );

    expect(
      await screen.findByText(
        "Accesso a Google scaduto: premi di nuovo Scegli da Google Drive",
      ),
    ).toBeVisible();
    expect(refresh).toHaveBeenCalled();
  });

  it("shows the import error when the Drive import fails", async () => {
    mocks.post.mockImplementation((path: string) =>
      Promise.resolve(
        path === "/api/google/picker"
          ? {
              data: { accessToken: "t", apiKey: "k", appId: "1" },
              response: { status: 200 },
            }
          : { data: undefined, response: { status: 422 } },
      ),
    );
    mocks.pick.mockResolvedValue({ id: "drive1", name: "x" });
    renderDialog({ connected: true });

    await userEvent.click(
      screen.getByRole("button", { name: "Scegli da Google Drive" }),
    );

    expect(
      await screen.findByText("Formato non supportato o file non valido"),
    ).toBeVisible();
  });
});
