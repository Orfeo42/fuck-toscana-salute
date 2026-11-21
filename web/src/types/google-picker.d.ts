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

interface Window {
  gapi?: { load: (name: string, callback: () => void) => void };
  google?: { picker?: GooglePickerNamespace };
}
