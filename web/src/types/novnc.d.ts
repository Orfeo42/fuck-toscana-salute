declare module "@novnc/novnc" {
  export interface RFBOptions {
    shared?: boolean;
    credentials?: { username?: string; password?: string; target?: string };
  }

  export interface RFBDisconnectDetail {
    clean: boolean;
  }

  export interface RFBClipboardDetail {
    text: string;
  }

  export default class RFB {
    constructor(target: HTMLElement, url: string, options?: RFBOptions);
    scaleViewport: boolean;
    resizeSession: boolean;
    clipViewport: boolean;
    focusOnClick: boolean;
    viewOnly: boolean;
    clipboardPasteFrom(text: string): void;
    sendKey(keysym: number, code: string | null, down?: boolean): void;
    focus(): void;
    disconnect(): void;
    addEventListener(
      type: "connect",
      listener: (event: CustomEvent) => void,
    ): void;
    addEventListener(
      type: "disconnect",
      listener: (event: CustomEvent<RFBDisconnectDetail>) => void,
    ): void;
    addEventListener(
      type: "clipboard",
      listener: (event: CustomEvent<RFBClipboardDetail>) => void,
    ): void;
    removeEventListener(
      type: "connect",
      listener: (event: CustomEvent) => void,
    ): void;
    removeEventListener(
      type: "disconnect",
      listener: (event: CustomEvent<RFBDisconnectDetail>) => void,
    ): void;
    removeEventListener(
      type: "clipboard",
      listener: (event: CustomEvent<RFBClipboardDetail>) => void,
    ): void;
  }
}
