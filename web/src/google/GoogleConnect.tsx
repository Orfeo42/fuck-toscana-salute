import type { JSX } from "react";

import { notConfiguredHint, useGoogle } from "./useGoogle";

export function GoogleConnect(): JSX.Element | null {
  const { status, error, connect, disconnect } = useGoogle();

  const handleConnect = (): void => {
    void connect();
  };
  const handleDisconnect = (): void => {
    void disconnect();
  };

  if (status === undefined) {
    return error === undefined ? null : (
      <p className="session-error" role="alert">
        {error}
      </p>
    );
  }
  if (!status.configured) {
    return (
      <p className="google-connect muted" title={notConfiguredHint}>
        Google Calendar non configurato
      </p>
    );
  }
  if (!status.connected) {
    return (
      <div className="google-connect">
        <button type="button" onClick={handleConnect}>
          Collega Google Calendar
        </button>
        {error === undefined ? null : <span role="alert">{error}</span>}
      </div>
    );
  }
  return (
    <div className="google-connect">
      <span>Google Calendar collegato</span>
      <button
        type="button"
        className="secondary small"
        onClick={handleDisconnect}
      >
        Scollega
      </button>
    </div>
  );
}
