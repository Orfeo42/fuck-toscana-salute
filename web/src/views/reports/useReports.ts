import { useCallback, useEffect, useState } from "react";

import { api, type ReportsBody } from "../../api/client";

export type ReportsError = "unauthorized" | "unavailable";

export interface ReportsState {
  data: ReportsBody | undefined;
  error: ReportsError | undefined;
  loading: boolean;
  reload: () => void;
}

interface Result {
  requestId: number;
  data: ReportsBody | undefined;
  error: ReportsError | undefined;
}

export function useReports(): ReportsState {
  const [requestId, setRequestId] = useState(0);
  const [result, setResult] = useState<Result | undefined>(undefined);

  useEffect(() => {
    const controller = new AbortController();
    const run = async (): Promise<void> => {
      try {
        const { data, response } = await api.GET("/api/reports", {
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
          error: response.status === 401 ? "unauthorized" : "unavailable",
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
    setRequestId((id) => id + 1);
  }, []);

  const current = result?.requestId === requestId ? result : undefined;

  return {
    data: current?.data,
    error: current?.error,
    loading: current === undefined,
    reload,
  };
}
