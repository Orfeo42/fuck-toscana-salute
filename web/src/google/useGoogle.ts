import { createContext, useContext } from "react";

import type { GoogleStatusBody } from "../api/client";

export interface GoogleValue {
  status: GoogleStatusBody | undefined;
  loading: boolean;
  error: string | undefined;
  refresh: () => void;
  connect: () => Promise<void>;
  disconnect: () => Promise<void>;
}

export const GoogleContext = createContext<GoogleValue | undefined>(undefined);

export const notConfiguredHint =
  "Crea ~/.config/fuck-toscana-salute/google-client.json (vedi README)";

export function useGoogle(): GoogleValue {
  const value = useContext(GoogleContext);
  if (value === undefined) {
    throw new Error("useGoogle must be used inside GoogleProvider");
  }
  return value;
}
