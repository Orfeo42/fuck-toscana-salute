import { Box, Button, Group, Stack, Text, TextInput } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import RFB from "@novnc/novnc";
import { ClipboardPaste } from "lucide-react";
import {
  useEffect,
  useRef,
  useState,
  type ClipboardEvent,
  type SyntheticEvent,
  type JSX,
} from "react";

type Status = "connecting" | "connected" | "disconnected";

const KEYSYM_CONTROL_L = 0xffe3;
const KEYSYM_V = 0x0076;
const statusLabels: Record<Status, string> = {
  connecting: "Connessione…",
  connected: "Connesso",
  disconnected: "Disconnesso",
};
const targetStyle = {
  width: "100%",
  height: "calc(100dvh - 300px)",
  minHeight: 240,
  touchAction: "none",
  background: "#000",
};

function viewerUrl(): string {
  const scheme = window.location.protocol === "https:" ? "wss" : "ws";
  return `${scheme}://${window.location.host}/viewer/websockify`;
}

export function RemoteViewer(): JSX.Element {
  const targetRef = useRef<HTMLDivElement>(null);
  const rfbRef = useRef<RFB | null>(null);
  const textRef = useRef<HTMLInputElement>(null);
  const [status, setStatus] = useState<Status>("connecting");
  const [attempt, setAttempt] = useState(0);
  const [text, setText] = useState("");

  useEffect(() => {
    const target = targetRef.current;
    if (!target) {
      return undefined;
    }
    const rfb = new RFB(target, viewerUrl());
    rfb.scaleViewport = false;
    rfb.resizeSession = true;
    rfb.focusOnClick = true;
    rfbRef.current = rfb;

    const onConnect = (): void => {
      setStatus("connected");
    };
    const onDisconnect = (): void => {
      setStatus("disconnected");
    };
    const onClipboard = (event: CustomEvent<{ text: string }>): void => {
      navigator.clipboard
        .writeText(event.detail.text)
        .then(() => {
          notifications.show({ message: "Copiato dal browser remoto" });
        })
        .catch(() => undefined);
    };
    rfb.addEventListener("connect", onConnect);
    rfb.addEventListener("disconnect", onDisconnect);
    rfb.addEventListener("clipboard", onClipboard);

    return () => {
      rfb.removeEventListener("connect", onConnect);
      rfb.removeEventListener("disconnect", onDisconnect);
      rfb.removeEventListener("clipboard", onClipboard);
      rfb.disconnect();
      rfbRef.current = null;
    };
  }, [attempt]);

  function sendToRemote(value: string): void {
    const rfb = rfbRef.current;
    if (!rfb || value === "") {
      return;
    }
    rfb.clipboardPasteFrom(value);
    rfb.sendKey(KEYSYM_CONTROL_L, "ControlLeft", true);
    rfb.sendKey(KEYSYM_V, "KeyV", true);
    rfb.sendKey(KEYSYM_V, "KeyV", false);
    rfb.sendKey(KEYSYM_CONTROL_L, "ControlLeft", false);
    rfb.focus();
  }

  async function pasteFromDevice(): Promise<void> {
    try {
      sendToRemote(await navigator.clipboard.readText());
    } catch {
      textRef.current?.focus();
    }
  }

  function onPaste(event: ClipboardEvent<HTMLElement>): void {
    if (event.target === textRef.current) {
      return;
    }
    event.preventDefault();
    sendToRemote(event.clipboardData.getData("text"));
  }

  function onSubmit(event: SyntheticEvent<HTMLFormElement>): void {
    event.preventDefault();
    sendToRemote(text);
    setText("");
  }

  function reconnect(): void {
    setStatus("connecting");
    setAttempt((value) => value + 1);
  }

  return (
    <Stack gap="xs" onPaste={onPaste}>
      <Group gap="xs" justify="space-between">
        <Text size="sm" role="status">
          {statusLabels[status]}
        </Text>
        <Group gap="xs">
          {status === "disconnected" && (
            <Button size="xs" variant="default" onClick={reconnect}>
              Riprova
            </Button>
          )}
          <Button
            size="xs"
            variant="default"
            leftSection={<ClipboardPaste size={14} />}
            onClick={() => {
              void pasteFromDevice();
            }}
          >
            Incolla
          </Button>
        </Group>
      </Group>
      <Box ref={targetRef} data-testid="remote-target" style={targetStyle} />
      <form onSubmit={onSubmit}>
        <Group gap="xs" align="flex-end" wrap="nowrap">
          <TextInput
            ref={textRef}
            label="Testo da inviare al browser"
            value={text}
            onChange={(event) => {
              setText(event.currentTarget.value);
            }}
            style={{ flex: 1 }}
          />
          <Button type="submit">Invia</Button>
        </Group>
      </form>
    </Stack>
  );
}
