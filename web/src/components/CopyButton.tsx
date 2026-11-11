import { Text, Tooltip, UnstyledButton } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { Check, Copy, X, type LucideIcon } from "lucide-react";
import { useEffect, useRef, useState, type JSX } from "react";

type CopyState = "idle" | "copied" | "failed";

const resetMs = 2000;

const icons: Readonly<Record<CopyState, LucideIcon>> = {
  idle: Copy,
  copied: Check,
  failed: X,
};

interface Props {
  value: string;
  label: string;
}

export function CopyButton({ value, label }: Props): JSX.Element {
  const [state, setState] = useState<CopyState>("idle");
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => {
    return () => {
      clearTimeout(timer.current);
    };
  }, []);

  const show = (next: CopyState): void => {
    setState(next);
    clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      setState("idle");
    }, resetMs);
  };

  const copy = async (): Promise<void> => {
    try {
      await navigator.clipboard.writeText(value);
      show("copied");
      notifications.show({
        message: "NRE copiato",
        color: "teal",
        autoClose: resetMs,
        withCloseButton: false,
      });
    } catch {
      show("failed");
      notifications.show({
        message: "Copia non riuscita",
        color: "red",
        autoClose: resetMs,
        withCloseButton: false,
      });
    }
  };

  const Icon = icons[state];

  return (
    <Tooltip label={label} withArrow openDelay={150}>
      <UnstyledButton
        aria-label={`${label} ${value}`}
        onClick={() => {
          void copy();
        }}
        display="inline-flex"
        style={buttonStyle}
      >
        <Text ff="monospace" span miw="15ch">
          {value}
        </Text>
        <Icon aria-hidden="true" size={14} />
      </UnstyledButton>
    </Tooltip>
  );
}

const buttonStyle = { alignItems: "center", gap: 6 };
