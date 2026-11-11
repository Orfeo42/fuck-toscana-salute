import { Modal, Stack, Text } from "@mantine/core";
import { useMediaQuery } from "@mantine/hooks";
import { useState, type JSX } from "react";

import { useSession } from "../session/useSession";
import { RemoteViewer } from "./viewer/RemoteViewer";

const helpText =
  "Accedi con SPID/CIE nella finestra qui sotto. Per il CUP completa anche Gestione prenotazioni (captcha e tessera). Si chiude da sola alla fine.";
const closeButtonProps = { "aria-label": "Chiudi" };

export function LoginViewer(): JSX.Element | null {
  const { session } = useSession();
  const small = useMediaQuery("(max-width: 48em)");
  const [dismissed, setDismissed] = useState(false);

  const waiting =
    session?.viewer === true &&
    (session.portals ?? []).some((portal) => portal.login.phase === "waiting");

  if (dismissed && !waiting) {
    setDismissed(false);
  }

  if (!waiting) {
    return null;
  }

  return (
    <Modal
      opened={!dismissed}
      onClose={() => {
        setDismissed(true);
      }}
      title="Accesso ai portali"
      closeButtonProps={closeButtonProps}
      fullScreen={small}
      size="100%"
    >
      <Stack gap="xs">
        <Text size="sm">{helpText}</Text>
        <RemoteViewer />
      </Stack>
    </Modal>
  );
}
