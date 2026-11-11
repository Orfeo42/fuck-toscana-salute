import { describe, expect, it, vi, type Mock } from "vitest";

import {
  bookmarkletCode,
  parseTokensMessage,
  portalForOrigin,
} from "./bookmarklet";

const appOrigin = "https://fts.example.test";

interface Entry {
  name: string;
  responseStatus?: number;
}

interface Harness {
  storage: Record<string, string>;
  entries: Entry[];
}

function newWindow(): Window {
  const frame = document.createElement("iframe");
  document.body.append(frame);
  const win = frame.contentWindow;
  if (win === null) {
    throw new Error("iframe has no window");
  }
  return win;
}

function fakePopup(): { popup: Window; postMessage: Mock } {
  const popup = newWindow();
  const postMessage = vi.fn();
  Object.defineProperty(popup, "postMessage", { value: postMessage });
  return { popup, postMessage };
}

function run(harness: Harness, popup: Window | null) {
  const win = newWindow();
  const alertFn = vi.fn();
  const open = vi.fn(() => popup);
  const storage = {
    getItem: (key: string): string | null => harness.storage[key] ?? null,
  };
  const perf = { getEntriesByType: () => harness.entries };
  Object.defineProperties(win, {
    open: { value: open },
    alert: { value: alertFn },
    sessionStorage: { value: storage },
    performance: { value: perf },
  });
  const script = win.document.createElement("script");
  script.textContent = bookmarkletCode(appOrigin);
  win.document.body.append(script);

  const send = (origin: string, source: Window | null, data: unknown): void => {
    win.dispatchEvent(new MessageEvent("message", { origin, source, data }));
  };
  return { alertFn, open, send };
}

const readyMessage = { type: "fts-ready" };
const tokens = { access_token: "acc", refresh_token: "ref" };

describe("bookmarkletCode", () => {
  it("is a single-line javascript url", () => {
    const code = bookmarkletCode(appOrigin);

    expect(code.startsWith("javascript:")).toBe(true);
    expect(code).not.toMatch(/[\r\n]/);
    expect(code).not.toContain("%");
  });

  it("alerts and stops when there is no access token", () => {
    const { alertFn, open } = run({ storage: {}, entries: [] }, null);

    expect(alertFn).toHaveBeenCalledWith(
      "Accedi prima al portale, poi tocca di nuovo il segnalibro.",
    );
    expect(open).not.toHaveBeenCalled();
  });

  it("opens the app and answers fts-ready once with the tokens", () => {
    const { popup, postMessage } = fakePopup();
    const { open, send } = run(
      {
        storage: { ...tokens, fcode: "RSSMRA80A01H501U" },
        entries: [],
      },
      popup,
    );

    expect(open).toHaveBeenCalledWith(`${appOrigin}/#/collega`, "fts-collega");

    send(appOrigin, popup, readyMessage);
    send(appOrigin, popup, readyMessage);

    expect(postMessage).toHaveBeenCalledTimes(1);
    expect(postMessage).toHaveBeenCalledWith(
      {
        type: "fts-tokens",
        access: "acc",
        refresh: "ref",
        cf: "RSSMRA80A01H501U",
        tessera: "",
      },
      appOrigin,
    );
  });

  it("falls back to fseCf", () => {
    const { popup, postMessage } = fakePopup();
    const { send } = run(
      { storage: { ...tokens, fseCf: "RSSMRA80A01H501U" }, entries: [] },
      popup,
    );

    send(appOrigin, popup, readyMessage);

    expect(postMessage).toHaveBeenCalledWith(
      expect.objectContaining({ cf: "RSSMRA80A01H501U" }),
      appOrigin,
    );
  });

  it("ignores messages from another origin, source or type", () => {
    const { popup, postMessage } = fakePopup();
    const { send } = run({ storage: tokens, entries: [] }, popup);

    send("https://evil.example", popup, readyMessage);
    send(appOrigin, newWindow(), readyMessage);
    send(appOrigin, popup, null);
    send(appOrigin, popup, { type: "other" });

    expect(postMessage).not.toHaveBeenCalled();
  });

  it("takes the tessera from the last successful verificaTs entry", () => {
    const { popup, postMessage } = fakePopup();
    const base = "https://prenota.sanita.toscana.it/x/verificaTs";
    const { send } = run(
      {
        storage: tokens,
        entries: [
          {
            name: `${base}?numbersTeam=80380000000000000001`,
            responseStatus: 200,
          },
          { name: `${base}?numbersTeam=80380000000000000000` },
          {
            name: `${base}?numbersTeam=80380000000000000002`,
            responseStatus: 500,
          },
          { name: "https://prenota.sanita.toscana.it/other" },
        ],
      },
      popup,
    );

    send(appOrigin, popup, readyMessage);

    expect(postMessage).toHaveBeenCalledWith(
      expect.objectContaining({ tessera: "80380000000000000000" }),
      appOrigin,
    );
  });

  it("alerts when the popup is blocked", () => {
    const { alertFn, send } = run({ storage: tokens, entries: [] }, null);

    send(appOrigin, null, readyMessage);

    expect(alertFn).toHaveBeenCalled();
  });
});

describe("portalForOrigin", () => {
  it("maps the portal origins", () => {
    expect(portalForOrigin("https://prenota.sanita.toscana.it")).toBe("cup");
    expect(portalForOrigin("https://fse.sanita.toscana.it")).toBe("fse");
  });

  it("rejects anything else", () => {
    expect(portalForOrigin("https://evil.example")).toBeUndefined();
    expect(portalForOrigin("http://prenota.sanita.toscana.it")).toBeUndefined();
    expect(
      portalForOrigin("https://prenota.sanita.toscana.it.evil.example"),
    ).toBeUndefined();
  });

  it("does not resolve inherited object keys", () => {
    expect(portalForOrigin("constructor")).toBeUndefined();
    expect(portalForOrigin("__proto__")).toBeUndefined();
  });
});

describe("parseTokensMessage", () => {
  const valid = {
    type: "fts-tokens",
    access: "a",
    refresh: "r",
    cf: "RSSMRA80A01H501U",
    tessera: "",
  };

  it("accepts a well-formed message", () => {
    expect(parseTokensMessage(valid)).toEqual({
      access: "a",
      refresh: "r",
      cf: "RSSMRA80A01H501U",
      tessera: "",
    });
  });

  it("rejects other types and shapes", () => {
    expect(parseTokensMessage(null)).toBeUndefined();
    expect(parseTokensMessage("fts-tokens")).toBeUndefined();
    expect(parseTokensMessage({ ...valid, type: "fts-ready" })).toBeUndefined();
    expect(parseTokensMessage({ ...valid, access: 1 })).toBeUndefined();
    expect(
      parseTokensMessage({ ...valid, tessera: undefined }),
    ).toBeUndefined();
  });
});
