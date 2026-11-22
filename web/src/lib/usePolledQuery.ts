import { useCallback, useEffect, useState } from "react";

import {
  api,
  type DrugRowResponseBody,
  type DrugsQueryBody,
  type DrugsQueryRequest,
  type ReportRowRequest,
  type ReportRowResponseBody,
  type ReportsQueryBody,
  type ReportsQueryRequest,
  type RowRequest,
  type VisitRowResponseBody,
  type VisitsQueryBody,
  type VisitsQueryRequest,
} from "../api/client";
import { useSession } from "../session/useSession";

export type QueryError = "unauthorized" | "needs_gestione" | "unavailable";

interface QueryTypes {
  "/api/visits/query": {
    request: VisitsQueryRequest;
    response: VisitsQueryBody;
  };
  "/api/reports/query": {
    request: ReportsQueryRequest;
    response: ReportsQueryBody;
  };
  "/api/drugs/query": {
    request: DrugsQueryRequest;
    response: DrugsQueryBody;
  };
  "/api/visits/row": { request: RowRequest; response: VisitRowResponseBody };
  "/api/reports/row": {
    request: ReportRowRequest;
    response: ReportRowResponseBody;
  };
  "/api/drugs/row": { request: RowRequest; response: DrugRowResponseBody };
}

export type QueryPath = keyof QueryTypes;
export type QueryRequest<P extends QueryPath> = QueryTypes[P]["request"];
export type QueryResponse<P extends QueryPath> = QueryTypes[P]["response"];

export interface PolledQuery<P extends QueryPath> {
  data: QueryResponse<P> | undefined;
  error: QueryError | undefined;
  notFound: boolean;
  loading: boolean;
  reload: () => void;
}

interface Fetched<Res> {
  data: Res | undefined;
  status: number;
}

type Fetchers = {
  [P in QueryPath]: (
    body: QueryRequest<P>,
    signal: AbortSignal,
  ) => Promise<Fetched<QueryResponse<P>>>;
};

const fetchers: Fetchers = {
  "/api/visits/query": async (body, signal) => {
    const { data, response } = await api.POST("/api/visits/query", {
      body,
      signal,
    });
    return { data, status: response.status };
  },
  "/api/reports/query": async (body, signal) => {
    const { data, response } = await api.POST("/api/reports/query", {
      body,
      signal,
    });
    return { data, status: response.status };
  },
  "/api/drugs/query": async (body, signal) => {
    const { data, response } = await api.POST("/api/drugs/query", {
      body,
      signal,
    });
    return { data, status: response.status };
  },
  "/api/visits/row": async (body, signal) => {
    const { data, response } = await api.POST("/api/visits/row", {
      body,
      signal,
    });
    return { data, status: response.status };
  },
  "/api/reports/row": async (body, signal) => {
    const { data, response } = await api.POST("/api/reports/row", {
      body,
      signal,
    });
    return { data, status: response.status };
  },
  "/api/drugs/row": async (body, signal) => {
    const { data, response } = await api.POST("/api/drugs/row", {
      body,
      signal,
    });
    return { data, status: response.status };
  },
};

const pollIntervalMs = 3000;

function queryError(status: number): QueryError {
  if (status === 401) {
    return "unauthorized";
  }
  if (status === 409) {
    return "needs_gestione";
  }
  return "unavailable";
}

interface Settled<P extends QueryPath> {
  key: string;
  request: QueryRequest<P>;
}

interface Result<P extends QueryPath> {
  fetchId: string;
  data: QueryResponse<P> | undefined;
  error: QueryError | undefined;
  notFound: boolean;
}

export function usePolledQuery<P extends QueryPath>(
  path: P,
  request: QueryRequest<P>,
  debounceMs: number,
): PolledQuery<P> {
  const { session } = useSession();
  const portalKey =
    session?.portals?.map((p) => `${p.portal}:${p.state}`).join("|") ?? "";
  const requestKey = JSON.stringify(request);
  const [settled, setSettled] = useState<Settled<P>>({
    key: requestKey,
    request,
  });
  const [reloads, setReloads] = useState(0);
  const [result, setResult] = useState<Result<P> | undefined>(undefined);

  useEffect(() => {
    const timer = setTimeout(() => {
      setSettled((prev) =>
        prev.key === requestKey ? prev : { key: requestKey, request },
      );
    }, debounceMs);
    return () => {
      clearTimeout(timer);
    };
  }, [requestKey, request, debounceMs]);

  const fetchId = `${path}|${settled.key}|${portalKey}|${String(reloads)}`;

  useEffect(() => {
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;

    const run = async (): Promise<void> => {
      try {
        const { data, status } = await fetchers[path](
          settled.request,
          controller.signal,
        );
        if (controller.signal.aborted) {
          return;
        }
        if (data === undefined) {
          setResult({
            fetchId,
            data: undefined,
            error: status === 404 ? undefined : queryError(status),
            notFound: status === 404,
          });
          return;
        }
        setResult({ fetchId, data, error: undefined, notFound: false });
        if (data.refreshing) {
          schedule();
        }
      } catch {
        if (controller.signal.aborted) {
          return;
        }
        setResult({
          fetchId,
          data: undefined,
          error: "unavailable",
          notFound: false,
        });
      }
    };

    const schedule = (): void => {
      timer = setTimeout(() => {
        if (document.visibilityState === "visible") {
          void run();
          return;
        }
        schedule();
      }, pollIntervalMs);
    };

    void run();
    return () => {
      controller.abort();
      clearTimeout(timer);
    };
  }, [path, settled, fetchId]);

  const reload = useCallback((): void => {
    setReloads((count) => count + 1);
  }, []);

  const current = result?.fetchId === fetchId ? result : undefined;

  return {
    data: current === undefined ? result?.data : current.data,
    error: current?.error,
    notFound: current?.notFound ?? false,
    loading: current === undefined,
    reload,
  };
}
