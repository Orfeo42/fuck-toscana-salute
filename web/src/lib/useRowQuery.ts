import {
  usePolledQuery,
  type PolledQuery,
  type QueryRequest,
} from "./usePolledQuery";

export type RowPath = "/api/visits/row" | "/api/reports/row" | "/api/drugs/row";

export function useRowQuery<P extends RowPath>(
  path: P,
  request: QueryRequest<P>,
): PolledQuery<P> {
  return usePolledQuery(path, request, 0);
}
