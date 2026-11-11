import { Alert } from "@mantine/core";
import { Info } from "lucide-react";
import type { JSX } from "react";

import { useSession } from "../session/useSession";

export function SessionNotice(): JSX.Element | null {
  const { session, error } = useSession();

  if (error !== undefined) {
    return (
      <Alert color="red" role="alert">
        {error}
      </Alert>
    );
  }
  if (session === undefined) {
    return null;
  }
  if (!(session.portals ?? []).some((portal) => portal.state === "active")) {
    return (
      <Alert
        color="blue"
        variant="light"
        icon={<Info size={18} />}
        title="Accedi per vedere i tuoi dati"
      >
        Accedi apre una finestra: SPID, poi Gestione Prenotazioni (codice
        fiscale, tessera, captcha). La sessione resta attiva anche dopo un
        riavvio.
      </Alert>
    );
  }
  if (!session.gestioneReady) {
    return (
      <Alert color="yellow" variant="light">
        Date e orari degli appuntamenti non disponibili: usa Account → Accedi di
        nuovo.
      </Alert>
    );
  }
  return null;
}
