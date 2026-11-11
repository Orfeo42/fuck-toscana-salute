import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type JSX,
  type ReactNode,
} from "react";

import { api, type SessionBody } from "../api/client";
import { SessionContext, type SessionValue } from "./useSession";

interface Props {
  children: ReactNode;
}

const waitingPollMs = 2000;
const idlePollMs = 15000;

export function SessionProvider({ children }: Props): JSX.Element {
  const [session, setSession] = useState<SessionBody | undefined>(undefined);
  const [error, setError] = useState<string | undefined>(undefined);

  const load = useCallback(async (): Promise<void> => {
    try {
      const { data } = await api.GET("/api/session");
      if (data === undefined) {
        setError("Impossibile leggere lo stato della sessione");
        return;
      }
      setSession(data);
      setError(undefined);
    } catch {
      setError("Impossibile contattare il server");
    }
  }, []);

  const refresh = useCallback((): void => {
    void load();
  }, [load]);

  const loginAll = useCallback(async (): Promise<void> => {
    try {
      await api.POST("/api/login");
    } catch {
      setError("Impossibile avviare l'accesso");
    }
    await load();
  }, [load]);

  const logout = useCallback(async (): Promise<void> => {
    try {
      await api.POST("/api/logout");
    } catch {
      setError("Impossibile terminare la sessione");
    }
    await load();
  }, [load]);

  const waiting =
    session?.portals?.some((portal) => portal.login.phase === "waiting") ??
    false;

  useEffect(() => {
    const first = setTimeout(refresh, 0);
    const id = setInterval(refresh, waiting ? waitingPollMs : idlePollMs);
    return () => {
      clearTimeout(first);
      clearInterval(id);
    };
  }, [refresh, waiting]);

  const value = useMemo<SessionValue>(
    () => ({ session, error, refresh, loginAll, logout }),
    [session, error, refresh, loginAll, logout],
  );

  return <SessionContext value={value}>{children}</SessionContext>;
}
