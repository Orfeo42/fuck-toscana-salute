import { createContext, useContext } from "react";

import type { SessionBody } from "../api/client";

export interface SessionValue {
  session: SessionBody | undefined;
  error: string | undefined;
  refresh: () => void;
  loginAll: () => Promise<void>;
  logout: () => Promise<void>;
}

export const SessionContext = createContext<SessionValue | undefined>(
  undefined,
);

export function useSession(): SessionValue {
  const value = useContext(SessionContext);
  if (value === undefined) {
    throw new Error("useSession must be used inside SessionProvider");
  }
  return value;
}
