import {
  Anchor,
  Badge,
  Button,
  Card,
  CopyButton,
  Group,
  List,
  Stack,
  Text,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { Check, Copy, ExternalLink } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type JSX } from "react";

import { api, type PortalBody } from "../api/client";
import {
  bookmarkletCode,
  parseTokensMessage,
  portalForOrigin,
  portalUrls,
  type PortalId,
} from "../lib/bookmarklet";
import { useSession } from "../session/useSession";

const portalNames: Readonly<Record<PortalId, string>> = {
  cup: "CUP",
  fse: "FSE",
};

const bookmarkletName = "Invia a Toscana Salute";
const readyMessage = { type: "fts-ready" };
const importFailed = "Impossibile collegare il portale";

type ImportState =
  | { kind: "idle" }
  | { kind: "done"; portal: PortalId }
  | { kind: "error"; message: string };

function portalState(
  portals: PortalBody[] | null | undefined,
  id: PortalId,
): PortalBody["state"] {
  return portals?.find((portal) => portal.portal === id)?.state ?? "logged_out";
}

export function LinkView(): JSX.Element {
  const { session, refresh } = useSession();
  const [result, setResult] = useState<ImportState>({ kind: "idle" });
  const code = useMemo(() => bookmarkletCode(window.location.origin), []);

  const attachHref = useCallback(
    (element: HTMLAnchorElement | null): void => {
      element?.setAttribute("href", code);
    },
    [code],
  );

  useEffect(() => {
    const onMessage = (event: MessageEvent<unknown>): void => {
      const portal = portalForOrigin(event.origin);
      const message = parseTokensMessage(event.data);
      if (portal === undefined || message === undefined) {
        return;
      }
      const base = {
        portal,
        access: message.access,
        refresh: message.refresh,
        cf: message.cf,
      };
      const body =
        message.tessera === "" ? base : { ...base, tessera: message.tessera };
      void (async (): Promise<void> => {
        try {
          const { error } = await api.POST("/api/login/import", { body });
          if (error !== undefined) {
            setResult({ kind: "error", message: error.detail ?? importFailed });
            return;
          }
        } catch {
          setResult({ kind: "error", message: importFailed });
          return;
        }
        setResult({ kind: "done", portal });
        notifications.show({
          message: `${portalNames[portal]} collegato`,
          color: "teal",
        });
        refresh();
        window.close();
      })();
    };

    window.addEventListener("message", onMessage);
    const opener = window.opener as WindowProxy | null;
    opener?.postMessage(readyMessage, "*");
    return () => {
      window.removeEventListener("message", onMessage);
    };
  }, [refresh]);

  return (
    <Stack gap="lg" maw={720}>
      <Title order={2}>Collega i portali</Title>
      <List type="ordered" spacing="sm">
        <List.Item>
          Aggiungi il segnalibro qui sotto (una volta sola).
        </List.Item>
        <List.Item>Apri il portale in una nuova scheda.</List.Item>
        <List.Item>
          Accedi con SPID/CIE. Per il CUP completa anche «Gestione prenotazioni»
          (captcha e tessera sanitaria) prima del passo 4.
        </List.Item>
        <List.Item>
          Sulla pagina del portale tocca il segnalibro «{bookmarkletName}».
        </List.Item>
      </List>

      <Group>
        <Button
          component="a"
          href={portalUrls.cup}
          target="_blank"
          rel="noopener"
          variant="default"
          leftSection={<ExternalLink size={16} aria-hidden="true" />}
        >
          Apri CUP
        </Button>
        <Button
          component="a"
          href={portalUrls.fse}
          target="_blank"
          rel="noopener"
          variant="default"
          leftSection={<ExternalLink size={16} aria-hidden="true" />}
        >
          Apri FSE
        </Button>
      </Group>

      <Card withBorder>
        <Stack gap="sm">
          <Title order={3} size="h4">
            Segnalibro
          </Title>
          <Text size="sm">
            Trascina questo link nella barra dei segnalibri:{" "}
            <Anchor ref={attachHref} fw={600} draggable>
              {bookmarkletName}
            </Anchor>
          </Text>
          <Text size="sm" c="dimmed">
            Da telefono: salva un segnalibro qualsiasi, modificalo e incolla il
            codice come indirizzo; poi, sulla pagina del portale, scrivi il nome
            del segnalibro nella barra degli indirizzi e toccalo.
          </Text>
          <CopyButton value={code}>
            {({ copied, copy }) => (
              <Button
                variant="light"
                w="fit-content"
                leftSection={
                  copied ? (
                    <Check size={16} aria-hidden="true" />
                  ) : (
                    <Copy size={16} aria-hidden="true" />
                  )
                }
                onClick={copy}
              >
                {copied ? "Codice copiato" : "Copia il codice"}
              </Button>
            )}
          </CopyButton>
        </Stack>
      </Card>

      <Card withBorder>
        <Stack gap="xs">
          <Title order={3} size="h4">
            Stato
          </Title>
          {(["cup", "fse"] as const).map((id) => {
            const connected = portalState(session?.portals, id) === "active";
            return (
              <Group key={id} justify="space-between">
                <Text size="sm" fw={600}>
                  {portalNames[id]}
                </Text>
                <Badge variant="light" color={connected ? "teal" : "gray"}>
                  {connected ? "collegato" : "non collegato"}
                </Badge>
              </Group>
            );
          })}
          <Group justify="space-between">
            <Text size="sm" fw={600}>
              Date appuntamenti
            </Text>
            <Badge
              variant="light"
              color={session?.gestioneReady === true ? "teal" : "gray"}
            >
              {session?.gestioneReady === true
                ? "disponibili"
                : "non disponibili"}
            </Badge>
          </Group>
        </Stack>
      </Card>

      {result.kind === "done" && (
        <Text role="status" fw={600}>
          {portalNames[result.portal]} collegato. Puoi chiudere questa scheda.
        </Text>
      )}
      {result.kind === "error" && (
        <Text role="alert" c="red" fw={600}>
          {result.message}
        </Text>
      )}
    </Stack>
  );
}
