import type { JSX } from "react";

import type { PortalBody } from "../api/client";
import { formatDateTime } from "../lib/format";
import { useSession } from "../session/useSession";

const portalNames: Record<PortalBody["portal"], string> = {
  cup: "CUP",
  fse: "FSE",
};

const stateLabels: Record<PortalBody["state"], string> = {
  active: "attivo",
  expired: "scaduto",
  logged_out: "non connesso",
};

const failureLabels: Record<string, string> = {
  timeout: "tempo scaduto",
  window_closed: "finestra chiusa",
  interaction_required: "serve un nuovo accesso",
  error: "errore",
};

function statusText(portal: PortalBody): string {
  if (portal.login.phase === "waiting") {
    return "accesso in corso…";
  }
  if (portal.login.phase === "failed") {
    const reason = failureLabels[portal.login.error ?? "error"] ?? "errore";
    return `${stateLabels[portal.state]} (${reason})`;
  }
  return stateLabels[portal.state];
}

export function SessionBar(): JSX.Element {
  const { session, error, loginAll, logout } = useSession();
  const portals = session?.portals ?? [];
  const waiting = portals.some((portal) => portal.login.phase === "waiting");

  const handleLogin = (): void => {
    void loginAll();
  };
  const handleLogout = (): void => {
    void logout();
  };

  return (
    <section className="session-bar" aria-label="Sessione">
      <ul className="session-portals">
        {portals.map((portal) => (
          <li key={portal.portal} className={`portal portal-${portal.state}`}>
            <span className="portal-name">{portalNames[portal.portal]}</span>
            <span className="portal-state">{statusText(portal)}</span>
            {portal.refreshExpiresAt !== undefined && (
              <span className="portal-expiry">
                scade {formatDateTime(portal.refreshExpiresAt)}
              </span>
            )}
          </li>
        ))}
        {session !== undefined && (
          <li className="portal">
            <span className="portal-name">Date appuntamenti</span>
            <span className="portal-state">
              {session.gestioneReady ? "disponibile" : "non disponibile"}
            </span>
          </li>
        )}
      </ul>
      {error !== undefined && (
        <p className="session-error" role="alert">
          {error}
        </p>
      )}
      <div className="session-actions">
        <button type="button" onClick={handleLogin} disabled={waiting}>
          Accedi
        </button>
        <button type="button" className="secondary" onClick={handleLogout}>
          Esci
        </button>
      </div>
      <p className="session-help">
        Accedi apre una finestra: SPID, poi Gestione Prenotazioni (codice
        fiscale, tessera, captcha). La sessione resta attiva anche dopo un
        riavvio.
      </p>
    </section>
  );
}
