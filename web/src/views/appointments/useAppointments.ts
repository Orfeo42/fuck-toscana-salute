import { useCallback, useEffect, useState } from "react";

import { api, type AppointmentsBody } from "../../api/client";
import { useSession } from "../../session/useSession";

export type AppointmentsError =
  "unauthorized" | "needs_gestione" | "unavailable";

export interface AppointmentsState {
  data: AppointmentsBody | undefined;
  error: AppointmentsError | undefined;
  loading: boolean;
  reload: () => void;
}

interface Result {
  requestId: string;
  data: AppointmentsBody | undefined;
  error: AppointmentsError | undefined;
}

function errorForStatus(status: number): AppointmentsError {
  if (status === 401) {
    return "unauthorized";
  }
  if (status === 409) {
    return "needs_gestione";
  }
  return "unavailable";
}

export function useAppointments(): AppointmentsState {
  const { session } = useSession();
  const [reloads, setReloads] = useState(0);
  const [result, setResult] = useState<Result | undefined>(undefined);

  const cupState =
    session?.portals?.find((portal) => portal.portal === "cup")?.state ?? "";
  const gestioneReady = session?.gestioneReady ?? false;
  const requestId = `${cupState}|${String(gestioneReady)}|${String(reloads)}`;

  useEffect(() => {
    const controller = new AbortController();
    const run = async (): Promise<void> => {
      try {
        const { data, response } = await api.GET("/api/appointments", {
          signal: controller.signal,
        });
        if (controller.signal.aborted) {
          return;
        }
        if (data !== undefined) {
          setResult({ requestId, data, error: undefined });
          return;
        }
        setResult({
          requestId,
          data: undefined,
          error: errorForStatus(response.status),
        });
      } catch {
        if (controller.signal.aborted) {
          return;
        }
        setResult({ requestId, data: undefined, error: "unavailable" });
      }
    };
    void run();
    return () => {
      controller.abort();
    };
  }, [requestId]);

  const reload = useCallback((): void => {
    setReloads((count) => count + 1);
  }, []);

  const current = result?.requestId === requestId ? result : undefined;
  return {
    data: current?.data,
    error: current?.error,
    loading: current === undefined,
    reload,
  };
}
