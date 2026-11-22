export const googleScopes =
  "https://www.googleapis.com/auth/calendar.events https://www.googleapis.com/auth/drive.file";

export type CodeRequester = () => Promise<string | null>;

const scriptUrl = "https://accounts.google.com/gsi/client";
const loadFailedMessage = "Impossibile caricare l'accesso Google";
const popupBlockedMessage =
  "Il browser ha bloccato la finestra di Google: consenti i popup";
const signInFailedMessage = "Accesso a Google non riuscito";

let identity: Promise<GoogleOAuth2Namespace> | undefined;

function injectScript(): Promise<void> {
  return new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.src = scriptUrl;
    script.async = true;
    script.addEventListener("load", () => {
      resolve();
    });
    script.addEventListener("error", () => {
      reject(new Error(loadFailedMessage));
    });
    document.head.appendChild(script);
  });
}

async function loadScript(): Promise<GoogleOAuth2Namespace> {
  await injectScript();
  const oauth2 = window.google?.accounts?.oauth2;
  if (oauth2 === undefined) {
    throw new Error(loadFailedMessage);
  }
  return oauth2;
}

export async function loadGoogleIdentity(): Promise<GoogleOAuth2Namespace> {
  const present = window.google?.accounts?.oauth2;
  if (present !== undefined) {
    return present;
  }
  identity ??= loadScript();
  try {
    return await identity;
  } catch (error: unknown) {
    identity = undefined;
    throw error;
  }
}

interface Pending {
  resolve: (code: string | null) => void;
  reject: (error: Error) => void;
}

export function createCodeRequester(
  oauth2: GoogleOAuth2Namespace,
  clientId: string,
): CodeRequester {
  let pending: Pending | undefined;
  const client = oauth2.initCodeClient({
    client_id: clientId,
    scope: googleScopes,
    ux_mode: "popup",
    callback: (response) => {
      const current = pending;
      pending = undefined;
      if (response.code === undefined || response.error !== undefined) {
        current?.resolve(null);
        return;
      }
      current?.resolve(response.code);
    },
    error_callback: (error) => {
      const current = pending;
      pending = undefined;
      if (error.type === "popup_closed") {
        current?.resolve(null);
        return;
      }
      if (error.type === "popup_failed_to_open") {
        current?.reject(new Error(popupBlockedMessage));
        return;
      }
      current?.reject(new Error(signInFailedMessage));
    },
  });
  return () =>
    new Promise<string | null>((resolve, reject) => {
      pending?.resolve(null);
      pending = { resolve, reject };
      client.requestCode();
    });
}
