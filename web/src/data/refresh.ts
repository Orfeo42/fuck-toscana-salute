import { api, type RefreshRequest } from "../api/client";

export type RefreshScope = Pick<RefreshRequest, "all" | "nres">;

export async function refreshPortals(scope: RefreshScope): Promise<boolean> {
  const { response } = await api.POST("/api/refresh", { body: scope });
  return response.ok;
}
