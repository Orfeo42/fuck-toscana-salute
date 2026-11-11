import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type JSX,
  type ReactNode,
} from "react";

import { api, type GoogleStatusBody } from "../api/client";
import { GoogleContext, type GoogleValue } from "./useGoogle";

interface Props {
  children: ReactNode;
}

const connectPollMs = 3000;
const connectPollMaxMs = 2 * 60 * 1000;

export function GoogleProvider({ children }: Props): JSX.Element {
  const [status, setStatus] = useState<GoogleStatusBody | undefined>(undefined);
  const [error, setError] = useState<string | undefined>(undefined);
  const pollRef = useRef<number | undefined>(undefined);
  const deadlineRef = useRef<number | undefined>(undefined);

  const load = useCallback(async (): Promise<GoogleStatusBody | undefined> => {
    try {
      const { data } = await api.GET("/api/google");
      if (data === undefined) {
        setError("Impossibile leggere lo stato di Google Calendar");
        return undefined;
      }
      setStatus(data);
      setError(undefined);
      return data;
    } catch {
      setError("Impossibile contattare il server");
      return undefined;
    }
  }, []);

  const refresh = useCallback((): void => {
    void load();
  }, [load]);

  const stopPolling = useCallback((): void => {
    window.clearInterval(pollRef.current);
    window.clearTimeout(deadlineRef.current);
    pollRef.current = undefined;
    deadlineRef.current = undefined;
  }, []);

  const startPolling = useCallback((): void => {
    stopPolling();
    pollRef.current = window.setInterval(() => {
      void load().then((next) => {
        if (next?.connected === true) {
          stopPolling();
        }
      });
    }, connectPollMs);
    deadlineRef.current = window.setTimeout(stopPolling, connectPollMaxMs);
  }, [load, stopPolling]);

  const connect = useCallback(async (): Promise<void> => {
    try {
      const { data } = await api.POST("/api/google/connect");
      if (data === undefined) {
        setError("Impossibile avviare il collegamento a Google Calendar");
        return;
      }
      setError(undefined);
      window.open(data.authUrl, "_blank", "noopener");
      startPolling();
    } catch {
      setError("Impossibile avviare il collegamento a Google Calendar");
    }
  }, [startPolling]);

  const disconnect = useCallback(async (): Promise<void> => {
    try {
      await api.DELETE("/api/google");
    } catch {
      setError("Impossibile scollegare Google Calendar");
    }
    await load();
  }, [load]);

  useEffect(() => {
    const first = setTimeout(refresh, 0);
    window.addEventListener("focus", refresh);
    return () => {
      clearTimeout(first);
      window.removeEventListener("focus", refresh);
      stopPolling();
    };
  }, [refresh, stopPolling]);

  const loading = status === undefined && error === undefined;

  const value = useMemo<GoogleValue>(
    () => ({ status, loading, error, refresh, connect, disconnect }),
    [status, loading, error, refresh, connect, disconnect],
  );

  return <GoogleContext value={value}>{children}</GoogleContext>;
}
