import { useState, type JSX } from "react";

type CopyState = "idle" | "copied" | "failed";

const messages: Readonly<Record<CopyState, string>> = {
  idle: "",
  copied: "Copiato",
  failed: "Copia non riuscita",
};

interface Props {
  value: string;
  label: string;
}

export function CopyButton({ value, label }: Props): JSX.Element {
  const [state, setState] = useState<CopyState>("idle");

  const copy = async (): Promise<void> => {
    try {
      await navigator.clipboard.writeText(value);
      setState("copied");
    } catch {
      setState("failed");
    }
  };

  return (
    <span className="copy-button">
      <button
        type="button"
        aria-label={label}
        onClick={() => {
          void copy();
        }}
      >
        Copia
      </button>
      <span role="status" className="copy-status">
        {messages[state]}
      </span>
    </span>
  );
}
