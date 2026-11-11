import { Check, Copy, X, type LucideIcon } from "lucide-react";
import { useEffect, useRef, useState, type JSX } from "react";

type CopyState = "idle" | "copied" | "failed";

const resetMs = 2000;

const messages: Readonly<Record<CopyState, string>> = {
  idle: "",
  copied: "Copiato",
  failed: "Copia non riuscita",
};

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
    } catch {
      show("failed");
    }
  };

  const Icon = icons[state];

  return (
    <span className="copy-button">
      <button
        type="button"
        className="nre-copy"
        aria-label={`${label} ${value}`}
        onClick={() => {
          void copy();
        }}
      >
        <span className="nre-text">{value}</span>
        <Icon aria-hidden="true" size={14} />
        <span className="icon-tip" aria-hidden="true">
          {state === "idle" ? label : messages[state]}
        </span>
      </button>
      <span role="status" className="copy-status">
        {messages[state]}
      </span>
    </span>
  );
}
