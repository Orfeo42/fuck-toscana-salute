import { Menu } from "@mantine/core";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { GoogleStatusBody } from "../api/client";
import { render } from "../test/render";
import { GoogleConnect } from "./GoogleConnect";
import { GoogleContext } from "./useGoogle";

function status(configured: boolean, connected: boolean): GoogleStatusBody {
  return {
    configured,
    connected,
    clientId: "123456789012-abc.apps.googleusercontent.com",
    drive: false,
  };
}

function renderConnect(
  current: GoogleStatusBody | undefined,
  options: {
    error?: string;
    signIn?: () => Promise<boolean>;
    disconnect?: () => Promise<void>;
    openSettings?: () => void;
  } = {},
): void {
  render(
    <GoogleContext
      value={{
        status: current,
        loading: false,
        error: options.error,
        refresh: vi.fn(),
        signIn: options.signIn ?? vi.fn(),
        disconnect: options.disconnect ?? vi.fn(),
        openSettings: options.openSettings ?? vi.fn(),
      }}
    >
      <Menu opened>
        <Menu.Target>
          <button type="button">t</button>
        </Menu.Target>
        <Menu.Dropdown>
          <GoogleConnect />
        </Menu.Dropdown>
      </Menu>
    </GoogleContext>,
  );
}

describe("GoogleConnect", () => {
  it("renders nothing while the status is unknown", () => {
    renderConnect(undefined);

    expect(screen.queryByRole("menuitem")).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows the error when the status is unknown", () => {
    renderConnect(undefined, { error: "boom" });

    expect(screen.getByRole("alert")).toHaveTextContent("boom");
  });

  it("shows the not configured text with a configure button", async () => {
    const user = userEvent.setup();
    const openSettings = vi.fn();
    renderConnect(status(false, false), { openSettings });

    expect(screen.getByText("Google non è configurato")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Configura" }));

    expect(openSettings).toHaveBeenCalled();
  });

  it("signs in from the Accedi con Google item", async () => {
    const user = userEvent.setup();
    const signIn = vi.fn().mockResolvedValue(true);
    renderConnect(status(true, false), { signIn });

    await user.click(
      screen.getByRole("menuitem", { name: "Accedi con Google" }),
    );

    expect(signIn).toHaveBeenCalled();
  });

  it("shows the error next to the connect button", () => {
    renderConnect(status(true, false), { error: "boom" });

    expect(screen.getByRole("alert")).toHaveTextContent("boom");
  });

  it("disconnects when connected", async () => {
    const user = userEvent.setup();
    const disconnect = vi.fn().mockResolvedValue(undefined);
    renderConnect(status(true, true), { disconnect });

    expect(screen.getByText("Collegato")).toBeVisible();
    await user.click(screen.getByRole("menuitem", { name: "Scollega Google" }));

    expect(disconnect).toHaveBeenCalled();
  });
});
