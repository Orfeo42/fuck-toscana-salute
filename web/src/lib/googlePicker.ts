import type { PickerBody } from "../api/client";

export interface PickedFile {
  id: string;
  name: string;
}

const apiUrl = "https://apis.google.com/js/api.js";
const mimeTypes = "application/pdf,image/jpeg,image/png";
const apiLoadFailed = "Impossibile caricare Google Picker";

let pickerApi: Promise<GooglePickerNamespace> | undefined;

function injectScript(): Promise<void> {
  if (window.gapi !== undefined) {
    return Promise.resolve();
  }
  return new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.src = apiUrl;
    script.async = true;
    script.addEventListener("load", () => {
      resolve();
    });
    script.addEventListener("error", () => {
      reject(new Error(apiLoadFailed));
    });
    document.head.appendChild(script);
  });
}

async function loadPicker(): Promise<GooglePickerNamespace> {
  await injectScript();
  const gapi = window.gapi;
  if (gapi === undefined) {
    throw new Error(apiLoadFailed);
  }
  await new Promise<void>((resolve) => {
    gapi.load("picker", resolve);
  });
  const picker = window.google?.picker;
  if (picker === undefined) {
    throw new Error(apiLoadFailed);
  }
  return picker;
}

export async function pickDriveFile(
  credentials: PickerBody,
): Promise<PickedFile | null> {
  pickerApi ??= loadPicker();
  let picker: GooglePickerNamespace;
  try {
    picker = await pickerApi;
  } catch (error: unknown) {
    pickerApi = undefined;
    throw error;
  }
  return new Promise((resolve) => {
    const view = new picker.DocsView()
      .setMimeTypes(mimeTypes)
      .setMode(picker.DocsViewMode.LIST);
    new picker.PickerBuilder()
      .addView(view)
      .setOAuthToken(credentials.accessToken)
      .setDeveloperKey(credentials.apiKey)
      .setAppId(credentials.appId)
      .setCallback((response) => {
        if (response.action === picker.Action.CANCEL) {
          resolve(null);
          return;
        }
        const doc = response.docs?.[0];
        if (response.action === picker.Action.PICKED && doc !== undefined) {
          resolve({ id: doc.id, name: doc.name });
        }
      })
      .build()
      .setVisible(true);
  });
}
