import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { GoogleStatusBody } from "../api/client";
import { render } from "../test/render";
import { GoogleConnect } from "./GoogleConnect";
import { GoogleContext } from "./useGoogle";

function renderConnect(
  status: GoogleStatusBody | undefined,
  options: {
    error?: string;
    connect?: () => Promise<void>;
    disconnect?: () => Promise<void>;
  } = {},
): void {
  render(
    <GoogleContext
      value={{
        status,
        loading: false,
        error: options.error,
        refresh: vi.fn(),
        connect: options.connect ?? vi.fn(),
        disconnect: options.disconnect ?? vi.fn(),
      }}
    >
      <GoogleConnect />
    </GoogleContext>,
  );
}

describe("GoogleConnect", () => {
  it("renders nothing while the status is unknown", () => {
    renderConnect(undefined);

    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows the error when the status is unknown", () => {
    renderConnect(undefined, { error: "boom" });

    expect(screen.getByRole("alert")).toHaveTextContent("boom");
  });

  it("shows the not configured text", () => {
    renderConnect({ configured: false, connected: false });

    expect(screen.getByText("Google Calendar non configurato")).toBeVisible();
  });

  it("connects from the connect button", async () => {
    const user = userEvent.setup();
    const connect = vi.fn().mockResolvedValue(undefined);
    renderConnect({ configured: true, connected: false }, { connect });

    await user.click(
      screen.getByRole("button", { name: "Collega Google Calendar" }),
    );

    expect(connect).toHaveBeenCalled();
  });

  it("shows the error next to the connect button", () => {
    renderConnect({ configured: true, connected: false }, { error: "boom" });

    expect(screen.getByRole("alert")).toHaveTextContent("boom");
  });

  it("disconnects when connected", async () => {
    const user = userEvent.setup();
    const disconnect = vi.fn().mockResolvedValue(undefined);
    renderConnect({ configured: true, connected: true }, { disconnect });

    expect(screen.getByText("Google Calendar collegato")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Scollega" }));

    expect(disconnect).toHaveBeenCalled();
  });
});
