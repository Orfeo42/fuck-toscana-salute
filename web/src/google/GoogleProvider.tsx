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
import {
  createCodeRequester,
  loadGoogleIdentity,
  type CodeRequester,
} from "../lib/googleSignIn";
import { GoogleSettingsModal } from "./GoogleSettingsModal";
import { GoogleContext, type GoogleValue } from "./useGoogle";

interface Props {
  children: ReactNode;
}

function codeFailureMessage(
  status: number,
  detail: string | undefined,
): string {
  if (status === 409 && detail === "google_grant_reset") {
    return "Google non ha concesso l'accesso permanente: riprova";
  }
  if (status === 403) {
    return "Consenti l'accesso a Calendar e Drive per continuare";
  }
  if (status === 409 && detail === "google_not_configured") {
    return "Google non è configurato";
  }
  return "Accesso a Google non riuscito";
}

export function GoogleProvider({ children }: Props): JSX.Element {
  const [status, setStatus] = useState<GoogleStatusBody | undefined>(undefined);
  const [error, setError] = useState<string | undefined>(undefined);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const requesterRef = useRef<CodeRequester | undefined>(undefined);

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

  const signIn = useCallback(async (): Promise<boolean> => {
    const requester = requesterRef.current;
    if (requester === undefined) {
      setError("Accesso Google non ancora pronto, riprova");
      return false;
    }
    let code: string | null;
    try {
      code = await requester();
    } catch (e: unknown) {
      setError(
        e instanceof Error ? e.message : "Accesso a Google non riuscito",
      );
      return false;
    }
    if (code === null) {
      return false;
    }
    try {
      const { error, response } = await api.POST("/api/google/code", {
        params: { header: { "X-Requested-With": "XmlHttpRequest" } },
        body: { code },
      });
      if (error !== undefined) {
        setError(codeFailureMessage(response.status, error.detail));
        return false;
      }
    } catch {
      setError("Accesso a Google non riuscito");
      return false;
    }
    setError(undefined);
    await load();
    return true;
  }, [load]);

  const disconnect = useCallback(async (): Promise<void> => {
    try {
      await api.DELETE("/api/google");
    } catch {
      setError("Impossibile scollegare Google Calendar");
    }
    await load();
  }, [load]);

  const openSettings = useCallback((): void => {
    setSettingsOpen(true);
  }, []);

  const closeSettings = useCallback((): void => {
    setSettingsOpen(false);
  }, []);

  useEffect(() => {
    const first = setTimeout(refresh, 0);
    window.addEventListener("focus", refresh);
    return () => {
      clearTimeout(first);
      window.removeEventListener("focus", refresh);
    };
  }, [refresh]);

  const clientId = status?.configured === true ? status.clientId : "";

  useEffect(() => {
    requesterRef.current = undefined;
    if (clientId === "") {
      return;
    }
    let cancelled = false;
    loadGoogleIdentity()
      .then((oauth2) => {
        if (!cancelled) {
          requesterRef.current = createCodeRequester(oauth2, clientId);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setError("Impossibile caricare l'accesso Google");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [clientId]);

  const loading = status === undefined && error === undefined;

  const value = useMemo<GoogleValue>(
    () => ({
      status,
      loading,
      error,
      refresh,
      signIn,
      disconnect,
      openSettings,
    }),
    [status, loading, error, refresh, signIn, disconnect, openSettings],
  );

  return (
    <GoogleContext value={value}>
      {children}
      <GoogleSettingsModal opened={settingsOpen} onClose={closeSettings} />
    </GoogleContext>
  );
}
