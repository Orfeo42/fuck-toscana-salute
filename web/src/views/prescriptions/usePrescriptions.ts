import { useCallback, useEffect, useState } from "react";

import { api } from "../../api/client";
import type { RowBody } from "../../api/client";
import { useSession } from "../../session/useSession";

export type PrescriptionsError = "unauthorized" | "unavailable";

export interface PrescriptionsState {
  data: RowBody[];
  error: PrescriptionsError | undefined;
  loading: boolean;
  reload: () => void;
}

interface Result {
  key: string;
  data: RowBody[];
  error: PrescriptionsError | undefined;
}

export function usePrescriptions(): PrescriptionsState {
  const { session } = useSession();
  const portalKey =
    session?.portals?.map((p) => `${p.portal}:${p.state}`).join("|") ?? "";
  const [reloads, setReloads] = useState(0);
  const [result, setResult] = useState<Result | undefined>(undefined);
  const requestKey = `${portalKey}#${String(reloads)}`;

  useEffect(() => {
    const controller = new AbortController();
    const run = async (): Promise<void> => {
      try {
        const { data, response } = await api.GET("/api/prescriptions", {
          signal: controller.signal,
        });
        if (controller.signal.aborted) {
          return;
        }
        if (data !== undefined) {
          setResult({
            key: requestKey,
            data: data.prescriptions ?? [],
            error: undefined,
          });
          return;
        }
        setResult({
          key: requestKey,
          data: [],
          error: response.status === 401 ? "unauthorized" : "unavailable",
        });
      } catch {
        if (controller.signal.aborted) {
          return;
        }
        setResult({ key: requestKey, data: [], error: "unavailable" });
      }
    };
    void run();
    return () => {
      controller.abort();
    };
  }, [requestKey]);

  const reload = useCallback(() => {
    setReloads((n) => n + 1);
  }, []);

  const loading = result?.key !== requestKey;
  return {
    data: loading ? [] : result.data,
    error: loading ? undefined : result.error,
    loading,
    reload,
  };
}
