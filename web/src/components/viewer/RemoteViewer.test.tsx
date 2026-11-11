import { act, fireEvent, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { render } from "../../test/render";
import { RemoteViewer } from "./RemoteViewer";

interface FakeRfb {
  target: HTMLElement;
  url: string;
  scaleViewport: boolean;
  resizeSession: boolean;
  focusOnClick: boolean;
  clipboardPasteFrom: ReturnType<typeof vi.fn>;
  sendKey: ReturnType<typeof vi.fn>;
  focus: ReturnType<typeof vi.fn>;
  disconnect: ReturnType<typeof vi.fn>;
  emit: (type: string, detail?: object) => void;
}

const mocks = vi.hoisted(() => {
  const instances: FakeRfb[] = [];

  class Rfb implements FakeRfb {
    scaleViewport = true;
    resizeSession = false;
    focusOnClick = false;
    clipboardPasteFrom = vi.fn();
    sendKey = vi.fn();
    focus = vi.fn();
    disconnect = vi.fn();
    listeners = new Map<string, Set<(event: CustomEvent) => void>>();

    constructor(
      public target: HTMLElement,
      public url: string,
    ) {
      instances.push(this);
    }

    addEventListener(
      type: string,
      listener: (event: CustomEvent) => void,
    ): void {
      const set = this.listeners.get(type) ?? new Set();
      set.add(listener);
      this.listeners.set(type, set);
    }

    removeEventListener(
      type: string,
      listener: (event: CustomEvent) => void,
    ): void {
      this.listeners.get(type)?.delete(listener);
    }

    emit(type: string, detail?: object): void {
      this.listeners.get(type)?.forEach((listener) => {
        listener(new CustomEvent(type, { detail }));
      });
    }
  }

  return { instances, Rfb };
});

vi.mock("@novnc/novnc", () => ({ default: mocks.Rfb }));

function lastRfb(): FakeRfb {
  const rfb = mocks.instances.at(-1);
  if (!rfb) {
    throw new Error("no RFB instance");
  }
  return rfb;
}

function expectPasted(rfb: FakeRfb, text: string): void {
  expect(rfb.clipboardPasteFrom).toHaveBeenCalledWith(text);
  expect(rfb.sendKey.mock.calls).toEqual([
    [0xffe3, "ControlLeft", true],
    [0x76, "KeyV", true],
    [0x76, "KeyV", false],
    [0xffe3, "ControlLeft", false],
  ]);
  expect(rfb.focus).toHaveBeenCalled();
}

describe("RemoteViewer", () => {
  beforeEach(() => {
    mocks.instances.length = 0;
  });

  it("connects to the same-origin websockify with resizing options", () => {
    render(<RemoteViewer />);

    const rfb = lastRfb();
    expect(rfb.target).toBe(screen.getByTestId("remote-target"));
    expect(rfb.url).toBe(`ws://${window.location.host}/viewer/websockify`);
    expect(rfb.scaleViewport).toBe(false);
    expect(rfb.resizeSession).toBe(true);
    expect(rfb.focusOnClick).toBe(true);
    expect(screen.getByText("Connessione…")).toBeInTheDocument();
  });

  it("shows connected after the connect event", () => {
    render(<RemoteViewer />);

    act(() => {
      lastRfb().emit("connect");
    });

    expect(screen.getByText("Connesso")).toBeInTheDocument();
  });

  it("disconnects on unmount", () => {
    const { unmount } = render(<RemoteViewer />);
    const rfb = lastRfb();

    unmount();

    expect(rfb.disconnect).toHaveBeenCalled();
  });

  it("copies the remote clipboard to the device and notifies", async () => {
    userEvent.setup();
    const writeText = vi.spyOn(navigator.clipboard, "writeText");
    render(<RemoteViewer />);

    act(() => {
      lastRfb().emit("clipboard", { text: "codice 123" });
    });

    expect(await screen.findByText("Copiato dal browser remoto")).toBeVisible();
    expect(writeText).toHaveBeenCalledWith("codice 123");
  });

  it("sends the device clipboard with the Incolla button", async () => {
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, "readText").mockResolvedValue("da incollare");
    render(<RemoteViewer />);

    await user.click(screen.getByRole("button", { name: "Incolla" }));

    expectPasted(lastRfb(), "da incollare");
  });

  it("focuses the text field when the clipboard cannot be read", async () => {
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, "readText").mockRejectedValue(
      new Error("denied"),
    );
    render(<RemoteViewer />);

    await user.click(screen.getByRole("button", { name: "Incolla" }));

    expect(screen.getByLabelText("Testo da inviare al browser")).toHaveFocus();
    expect(lastRfb().clipboardPasteFrom).not.toHaveBeenCalled();
  });

  it("sends the pasted text on a paste event", () => {
    render(<RemoteViewer />);

    fireEvent.paste(screen.getByTestId("remote-target"), {
      clipboardData: { getData: () => "incollato" },
    });

    expectPasted(lastRfb(), "incollato");
  });

  it("sends the typed text and clears the field", async () => {
    const user = userEvent.setup();
    render(<RemoteViewer />);
    const field = screen.getByLabelText("Testo da inviare al browser");

    await user.type(field, "ciao");
    await user.click(screen.getByRole("button", { name: "Invia" }));

    expectPasted(lastRfb(), "ciao");
    expect(field).toHaveValue("");
  });

  it("offers a reconnect after a disconnect", async () => {
    const user = userEvent.setup();
    render(<RemoteViewer />);
    const first = lastRfb();

    act(() => {
      first.emit("disconnect", { clean: false });
    });
    expect(screen.getByText("Disconnesso")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Riprova" }));

    expect(mocks.instances).toHaveLength(2);
    expect(first.disconnect).toHaveBeenCalled();
    expect(screen.getByText("Connessione…")).toBeInTheDocument();
  });
});
