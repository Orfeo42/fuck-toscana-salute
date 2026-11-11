import { Modal, Stack, Text } from "@mantine/core";
import { useMediaQuery } from "@mantine/hooks";
import { useState, type JSX } from "react";

import { useSession } from "../session/useSession";

const viewerSrc =
  "/viewer/vnc.html?autoconnect=true&resize=remote&reconnect=true&path=viewer/websockify";
const helpText =
  "Accedi con SPID/CIE nella finestra qui sotto. Per il CUP completa anche Gestione prenotazioni (captcha e tessera). Si chiude da sola alla fine.";
const closeButtonProps = { "aria-label": "Chiudi" };
const frameStyle = {
  width: "100%",
  height: "calc(100dvh - 160px)",
  border: 0,
};

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
        <iframe title="Browser di accesso" src={viewerSrc} style={frameStyle} />
      </Stack>
    </Modal>
  );
}
