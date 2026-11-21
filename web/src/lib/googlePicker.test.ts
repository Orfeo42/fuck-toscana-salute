import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

async function freshPicker(): Promise<
  typeof import("./googlePicker").pickDriveFile
> {
  vi.resetModules();
  return (await import("./googlePicker")).pickDriveFile;
}

const credentials = { accessToken: "tok", apiKey: "key", appId: "123" };

interface Fake {
  callback: ((response: GooglePickerResponse) => void) | undefined;
  calls: string[];
}

function installFakes(fake: Fake): void {
  class DocsView {
    setMimeTypes(types: string): this {
      fake.calls.push(`mime:${types}`);
      return this;
    }
    setMode(mode: string): this {
      fake.calls.push(`mode:${mode}`);
      return this;
    }
  }
  class PickerBuilder {
    addView(): this {
      fake.calls.push("view");
      return this;
    }
    setOAuthToken(token: string): this {
      fake.calls.push(`token:${token}`);
      return this;
    }
    setDeveloperKey(key: string): this {
      fake.calls.push(`key:${key}`);
      return this;
    }
    setAppId(appId: string): this {
      fake.calls.push(`app:${appId}`);
      return this;
    }
    setCallback(callback: (response: GooglePickerResponse) => void): this {
      fake.callback = callback;
      return this;
    }
    build(): GooglePickerInstance {
      return {
        setVisible: (visible) => {
          fake.calls.push(`visible:${String(visible)}`);
        },
      };
    }
  }
  window.gapi = {
    load: (_name, callback) => {
      window.google = {
        picker: {
          PickerBuilder,
          DocsView,
          DocsViewMode: { LIST: "list" },
          Action: { PICKED: "picked", CANCEL: "cancel" },
        },
      };
      callback();
    },
  };
}

function callback(fake: Fake): (response: GooglePickerResponse) => void {
  if (fake.callback === undefined) {
    throw new Error("picker not built");
  }
  return fake.callback;
}

describe("pickDriveFile", () => {
  let fake: Fake;

  beforeEach(() => {
    fake = { callback: undefined, calls: [] };
    installFakes(fake);
  });

  afterEach(() => {
    delete window.gapi;
    delete window.google;
  });

  it("configures the picker and resolves with the picked file", async () => {
    const pickDriveFile = await freshPicker();
    const result = pickDriveFile(credentials);
    await vi.waitFor(() => {
      expect(fake.callback).toBeDefined();
    });

    callback(fake)({ action: "loaded" });
    callback(fake)({
      action: "picked",
      docs: [{ id: "f1", name: "referto.pdf" }],
    });

    await expect(result).resolves.toEqual({ id: "f1", name: "referto.pdf" });
    expect(fake.calls).toEqual([
      "mime:application/pdf,image/jpeg,image/png",
      "mode:list",
      "view",
      "token:tok",
      "key:key",
      "app:123",
      "visible:true",
    ]);
  });

  it("resolves null when the user cancels", async () => {
    const pickDriveFile = await freshPicker();
    const result = pickDriveFile(credentials);
    await vi.waitFor(() => {
      expect(fake.callback).toBeDefined();
    });

    callback(fake)({ action: "cancel" });

    await expect(result).resolves.toBeNull();
  });
});

describe("pickDriveFile script loading", () => {
  afterEach(() => {
    delete window.gapi;
    delete window.google;
    document.head.querySelector("script")?.remove();
  });

  it("injects the api script and rejects when it fails, then retries", async () => {
    const pickDriveFile = await freshPicker();
    const first = pickDriveFile(credentials);
    const script = document.head.querySelector<HTMLScriptElement>(
      'script[src="https://apis.google.com/js/api.js"]',
    );
    expect(script).not.toBeNull();

    script?.dispatchEvent(new Event("error"));

    await expect(first).rejects.toThrow("Impossibile caricare Google Picker");

    const fake: Fake = { callback: undefined, calls: [] };
    installFakes(fake);
    const second = pickDriveFile(credentials);
    await vi.waitFor(() => {
      expect(fake.callback).toBeDefined();
    });
    callback(fake)({ action: "cancel" });
    await expect(second).resolves.toBeNull();
  });
});
