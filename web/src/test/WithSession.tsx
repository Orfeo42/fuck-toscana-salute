import { vi } from "vitest";
import type { JSX, ReactNode } from "react";

import type { PortalBody } from "../api/client";
import { SessionContext } from "../session/useSession";

interface Props {
  cupState?: PortalBody["state"];
  children: ReactNode;
}

export function WithSession({
  cupState = "active",
  children,
}: Props): JSX.Element {
  return (
    <SessionContext.Provider
      value={{
        session: {
          gestioneReady: true,
          loginMode: "window",
          viewer: false,
          portals: [
            { portal: "cup", state: cupState, login: { phase: "idle" } },
            { portal: "fse", state: "active", login: { phase: "idle" } },
          ],
        },
        error: undefined,
        refresh: vi.fn(),
        loginAll: vi.fn(),
        logout: vi.fn(),
      }}
    >
      {children}
    </SessionContext.Provider>
  );
}
