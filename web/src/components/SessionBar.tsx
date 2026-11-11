import { Badge, Button, Group, Paper, Stack, Text } from "@mantine/core";
import type { JSX } from "react";

import type { PortalBody } from "../api/client";
import { GoogleConnect } from "../google/GoogleConnect";
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

const stateColors: Record<PortalBody["state"], string> = {
  active: "teal",
  expired: "orange",
  logged_out: "gray",
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
    <Paper
      component="section"
      aria-label="Sessione"
      withBorder
      p="sm"
      radius="md"
    >
      <Stack gap="sm">
        <Group justify="space-between" wrap="wrap">
          <Group gap="md" wrap="wrap">
            {portals.map((portal) => (
              <Group key={portal.portal} gap="xs">
                <Text fw={600}>{portalNames[portal.portal]}</Text>
                <Badge color={stateColors[portal.state]} variant="light">
                  {statusText(portal)}
                </Badge>
                {portal.refreshExpiresAt !== undefined && (
                  <Text size="sm" c="dimmed">
                    scade {formatDateTime(portal.refreshExpiresAt)}
                  </Text>
                )}
              </Group>
            ))}
            {session !== undefined && (
              <Group gap="xs">
                <Text fw={600}>Date appuntamenti</Text>
                <Badge
                  color={session.gestioneReady ? "teal" : "gray"}
                  variant="light"
                >
                  {session.gestioneReady ? "disponibile" : "non disponibile"}
                </Badge>
              </Group>
            )}
          </Group>
          <Group gap="xs">
            <Button variant="filled" onClick={handleLogin} disabled={waiting}>
              Accedi
            </Button>
            <Button variant="default" onClick={handleLogout}>
              Esci
            </Button>
          </Group>
        </Group>
        {error !== undefined && (
          <Text c="red" role="alert">
            {error}
          </Text>
        )}
        <GoogleConnect />
        <Text size="sm" c="dimmed">
          Accedi apre una finestra: SPID, poi Gestione Prenotazioni (codice
          fiscale, tessera, captcha). La sessione resta attiva anche dopo un
          riavvio.
        </Text>
      </Stack>
    </Paper>
  );
}
