import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";

async function freshModule(): Promise<typeof import("./googleSignIn")> {
  vi.resetModules();
  return import("./googleSignIn");
}

const clientId = "123456789012-abc.apps.googleusercontent.com";

interface Fake {
  config: GoogleCodeClientConfig | undefined;
  requestCode: Mock<() => void>;
  oauth2: GoogleOAuth2Namespace;
}

function fakeOAuth2(): Fake {
  const fake: Fake = {
    config: undefined,
    requestCode: vi.fn<() => void>(),
    oauth2: {
      initCodeClient: (config) => {
        fake.config = config;
        return { requestCode: fake.requestCode };
      },
    },
  };
  return fake;
}

function config(fake: Fake): GoogleCodeClientConfig {
  if (fake.config === undefined) {
    throw new Error("code client not created");
  }
  return fake.config;
}

describe("loadGoogleIdentity", () => {
  beforeEach(() => {
    delete window.google;
    document.head.querySelectorAll("script").forEach((node) => {
      node.remove();
    });
  });

  it("resolves immediately when the namespace is already present", async () => {
    const fake = fakeOAuth2();
    window.google = { accounts: { oauth2: fake.oauth2 } };
    const { loadGoogleIdentity } = await freshModule();

    await expect(loadGoogleIdentity()).resolves.toBe(fake.oauth2);
    expect(document.head.querySelectorAll("script")).toHaveLength(0);
  });

  it("injects the script once and resolves after it loads", async () => {
    const fake = fakeOAuth2();
    const { loadGoogleIdentity } = await freshModule();

    const first = loadGoogleIdentity();
    const second = loadGoogleIdentity();
    const scripts = document.head.querySelectorAll("script");
    expect(scripts).toHaveLength(1);
    expect(scripts[0]?.src).toBe("https://accounts.google.com/gsi/client");
    window.google = { accounts: { oauth2: fake.oauth2 } };
    scripts[0]?.dispatchEvent(new Event("load"));

    await expect(first).resolves.toBe(fake.oauth2);
    await expect(second).resolves.toBe(fake.oauth2);
  });

  it("rejects and allows a retry when the script fails", async () => {
    const { loadGoogleIdentity } = await freshModule();

    const first = loadGoogleIdentity();
    document.head.querySelector("script")?.dispatchEvent(new Event("error"));
    await expect(first).rejects.toThrow(
      "Impossibile caricare l'accesso Google",
    );

    void loadGoogleIdentity();
    expect(document.head.querySelectorAll("script")).toHaveLength(2);
  });
});

describe("createCodeRequester", () => {
  let fake: Fake;

  afterEach(() => {
    delete window.google;
  });

  beforeEach(() => {
    fake = fakeOAuth2();
  });

  it("creates the code client once with the popup config", async () => {
    const { createCodeRequester, googleScopes } = await freshModule();

    const request = createCodeRequester(fake.oauth2, clientId);
    void request();
    void request();

    expect(config(fake).client_id).toBe(clientId);
    expect(config(fake).scope).toBe(googleScopes);
    expect(config(fake).ux_mode).toBe("popup");
  });

  it("calls requestCode synchronously", async () => {
    const { createCodeRequester } = await freshModule();
    const request = createCodeRequester(fake.oauth2, clientId);

    const promise = request();

    expect(fake.requestCode).toHaveBeenCalledTimes(1);
    config(fake).callback({ code: "fake-code" });
    await expect(promise).resolves.toBe("fake-code");
  });

  it("resolves null when the popup is closed", async () => {
    const { createCodeRequester } = await freshModule();
    const promise = createCodeRequester(fake.oauth2, clientId)();

    config(fake).error_callback({ type: "popup_closed" });

    await expect(promise).resolves.toBeNull();
  });

  it("rejects when the popup cannot open", async () => {
    const { createCodeRequester } = await freshModule();
    const promise = createCodeRequester(fake.oauth2, clientId)();

    config(fake).error_callback({ type: "popup_failed_to_open" });

    await expect(promise).rejects.toThrow(
      "Il browser ha bloccato la finestra di Google: consenti i popup",
    );
  });

  it("rejects on an unknown error", async () => {
    const { createCodeRequester } = await freshModule();
    const promise = createCodeRequester(fake.oauth2, clientId)();

    config(fake).error_callback({ type: "unknown" });

    await expect(promise).rejects.toThrow("Accesso a Google non riuscito");
  });

  it("resolves null on an error response", async () => {
    const { createCodeRequester } = await freshModule();
    const promise = createCodeRequester(fake.oauth2, clientId)();

    config(fake).callback({ error: "access_denied" });

    await expect(promise).resolves.toBeNull();
  });

  it("settles a previous pending request with null", async () => {
    const { createCodeRequester } = await freshModule();
    const request = createCodeRequester(fake.oauth2, clientId);

    const first = request();
    const second = request();
    config(fake).callback({ code: "fake-code" });

    await expect(first).resolves.toBeNull();
    await expect(second).resolves.toBe("fake-code");
  });
});
