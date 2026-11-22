import { api, type StartRefreshRequest } from "../api/client";

export type StartRefreshScope = Pick<
  StartRefreshRequest,
  "all" | "lists" | "reports" | "nres"
>;

export async function startRefresh(scope: StartRefreshScope): Promise<boolean> {
  const { response } = await api.POST("/api/refresh/start", { body: scope });
  return response.ok;
}
