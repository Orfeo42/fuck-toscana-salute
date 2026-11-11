import {
  Badge,
  Button,
  Group,
  Indicator,
  Loader,
  Menu,
  Text,
} from "@mantine/core";
import { ChevronDown, LogIn, LogOut, RefreshCw, UserRound } from "lucide-react";
import type { JSX } from "react";

import type { PortalBody } from "../api/client";
import { GoogleConnect } from "../google/GoogleConnect";
import { formatDateTime } from "../lib/format";
import { linkHref } from "../lib/route";
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

export function SessionMenu(): JSX.Element | null {
  const { session, loginAll, logout } = useSession();

  if (session === undefined) {
    return <Loader size="sm" color="white" aria-label="Caricamento sessione" />;
  }

  const handleLogin = (): void => {
    if (session.loginMode === "bookmarklet") {
      window.location.hash = linkHref;
      return;
    }
    void loginAll();
  };
  const handleLogout = (): void => {
    void logout();
  };

  const portals = session.portals ?? [];
  const loggedIn = portals.some((portal) => portal.state === "active");
  const waiting = portals.some((portal) => portal.login.phase === "waiting");

  if (!loggedIn) {
    return (
      <Button
        variant="white"
        color="navy"
        leftSection={<LogIn size={16} />}
        loading={waiting}
        onClick={handleLogin}
      >
        Accedi
      </Button>
    );
  }

  const degraded =
    portals.some((portal) => portal.state !== "active") ||
    !session.gestioneReady;

  return (
    <Menu position="bottom-end" width={300} withArrow shadow="md">
      <Menu.Target>
        <Indicator color="orange" disabled={!degraded} offset={4}>
          <Button
            variant="outline"
            color="navy.0"
            aria-label="Account"
            leftSection={<UserRound size={16} />}
            rightSection={<ChevronDown size={16} />}
          >
            Account
          </Button>
        </Indicator>
      </Menu.Target>
      <Menu.Dropdown>
        <Menu.Label>Portali</Menu.Label>
        {portals.map((portal) => (
          <Group
            key={portal.portal}
            justify="space-between"
            px="sm"
            py={4}
            wrap="nowrap"
          >
            <Text size="sm" fw={600}>
              {portalNames[portal.portal]}
            </Text>
            <Badge variant="light" color={stateColors[portal.state]}>
              {statusText(portal)}
            </Badge>
            {portal.refreshExpiresAt !== undefined && (
              <Text size="xs" c="dimmed">
                scade {formatDateTime(portal.refreshExpiresAt)}
              </Text>
            )}
          </Group>
        ))}
        <Group justify="space-between" px="sm" py={4} wrap="nowrap">
          <Text size="sm" fw={600}>
            Date appuntamenti
          </Text>
          <Badge
            variant="light"
            color={session.gestioneReady ? "teal" : "gray"}
          >
            {session.gestioneReady ? "disponibile" : "non disponibile"}
          </Badge>
        </Group>
        <Menu.Divider />
        <Menu.Label>Google Calendar</Menu.Label>
        <GoogleConnect />
        <Menu.Divider />
        {degraded && (
          <Menu.Item
            leftSection={<RefreshCw size={16} />}
            disabled={waiting}
            onClick={handleLogin}
          >
            Accedi di nuovo
          </Menu.Item>
        )}
        <Menu.Item
          color="red"
          leftSection={<LogOut size={16} />}
          onClick={handleLogout}
        >
          Esci
        </Menu.Item>
      </Menu.Dropdown>
    </Menu>
  );
}
