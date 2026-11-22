import {
  usePolledQuery,
  type PolledQuery,
  type QueryRequest,
} from "./usePolledQuery";

export type ViewPath =
  "/api/visits/query" | "/api/reports/query" | "/api/drugs/query";

const debounceMs = 250;

export function useViewQuery<P extends ViewPath>(
  path: P,
  request: QueryRequest<P>,
): PolledQuery<P> {
  return usePolledQuery(path, request, debounceMs);
}
