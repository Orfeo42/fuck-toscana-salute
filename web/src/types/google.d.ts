interface GooglePickerDoc {
  id: string;
  name: string;
}

interface GooglePickerResponse {
  action: string;
  docs?: GooglePickerDoc[];
}

interface GooglePickerDocsView {
  setMimeTypes: (mimeTypes: string) => GooglePickerDocsView;
  setMode: (mode: string) => GooglePickerDocsView;
}

interface GooglePickerInstance {
  setVisible: (visible: boolean) => void;
}

interface GooglePickerBuilder {
  addView: (view: GooglePickerDocsView) => GooglePickerBuilder;
  setOAuthToken: (token: string) => GooglePickerBuilder;
  setDeveloperKey: (key: string) => GooglePickerBuilder;
  setAppId: (appId: string) => GooglePickerBuilder;
  setCallback: (
    callback: (response: GooglePickerResponse) => void,
  ) => GooglePickerBuilder;
  build: () => GooglePickerInstance;
}

interface GooglePickerNamespace {
  PickerBuilder: new () => GooglePickerBuilder;
  DocsView: new () => GooglePickerDocsView;
  DocsViewMode: { LIST: string };
  Action: { PICKED: string; CANCEL: string };
}

interface GoogleCodeResponse {
  code?: string;
  scope?: string;
  state?: string;
  error?: string;
  error_description?: string;
}

interface GoogleCodeError {
  type: "popup_failed_to_open" | "popup_closed" | "unknown";
}

interface GoogleCodeClientConfig {
  client_id: string;
  scope: string;
  ux_mode: "popup";
  callback: (response: GoogleCodeResponse) => void;
  error_callback: (error: GoogleCodeError) => void;
}

interface GoogleCodeClient {
  requestCode: () => void;
}

interface GoogleOAuth2Namespace {
  initCodeClient: (config: GoogleCodeClientConfig) => GoogleCodeClient;
}

interface Window {
  gapi?: { load: (name: string, callback: () => void) => void };
  google?: {
    picker?: GooglePickerNamespace;
    accounts?: { oauth2: GoogleOAuth2Namespace };
  };
}
