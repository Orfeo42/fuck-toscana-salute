import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { JSX } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { SessionBody } from "../api/client";
import { SessionContext } from "../session/useSession";
import { callsTo } from "../test/calls";
import { HealthDataProvider } from "./HealthDataProvider";
import { useHealthData } from "./useHealthData";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
}));

vi.mock("../api/client", () => ({
  api: { GET: mocks.get },
}));

const appointmentsBody = {
  appointments: [
    {
      id: "a1",
      nre: "0900A0000000001",
      status: "PRENOTATO",
      at: "2026-11-12T09:30:00Z",
      unit: { name: "", description: "", address: "", city: "", province: "" },
    },
  ],
  failed: [{ nre: "0900A0000000009", reason: "not_in_cup" }],
};

const prescriptionsBody = { prescriptions: [{ nre: "0900A0000000002" }] };

function sessionWith(
  cupState: "active" | "expired",
  fseState: "active" | "expired" = "active",
  gestioneReady = true,
): SessionBody {
  return {
    gestioneReady,
    portals: [
      { portal: "cup", state: cupState, login: { phase: "idle" } },
      { portal: "fse", state: fseState, login: { phase: "idle" } },
    ],
  };
}

function respond(responses: Record<string, unknown>): void {
  mocks.get.mockImplementation((path: string) =>
    Promise.resolve(responses[path]),
  );
}

function Probe(): JSX.Element {
  const health = useHealthData();
  return (
    <div>
      <p data-testid="appointments">
        {`${String(health.appointments.loading)}|${health.appointments.error ?? "-"}|${String(health.appointments.data?.appointments?.length ?? "-")}`}
      </p>
      <p data-testid="prescriptions">
        {`${String(health.prescriptions.loading)}|${health.prescriptions.error ?? "-"}|${String(health.prescriptions.data?.prescriptions?.length ?? "-")}`}
      </p>
      <p data-testid="indexes">
        {`${[...health.bookingByNre.keys()].join(",")}|${[...health.notInCup].join(",")}|${[...health.prescriptionNres].join(",")}`}
      </p>
      <button
        type="button"
        onClick={() => {
          health.appointments.reload();
        }}
      >
        reload-appointments
      </button>
      <button
        type="button"
        onClick={() => {
          health.prescriptions.reload();
        }}
      >
        reload-prescriptions
      </button>
    </div>
  );
}

function Harness({ session }: { session: SessionBody }): JSX.Element {
  return (
    <SessionContext.Provider
      value={{
        session,
        error: undefined,
        refresh: vi.fn(),
        loginAll: vi.fn(),
        logout: vi.fn(),
      }}
    >
      <HealthDataProvider>
        <Probe />
      </HealthDataProvider>
    </SessionContext.Provider>
  );
}

describe("HealthDataProvider", () => {
  beforeEach(() => {
    mocks.get.mockReset();
    respond({
      "/api/appointments": { data: appointmentsBody },
      "/api/prescriptions": { data: prescriptionsBody },
    });
  });

  it("fetches both endpoints in parallel before either resolves", () => {
    mocks.get.mockReturnValue(new Promise(() => undefined));
    render(<Harness session={sessionWith("active")} />);

    expect(callsTo(mocks.get, "/api/appointments")).toBe(1);
    expect(callsTo(mocks.get, "/api/prescriptions")).toBe(1);
    expect(screen.getByTestId("appointments")).toHaveTextContent("true|-|-");
    expect(screen.getByTestId("prescriptions")).toHaveTextContent("true|-|-");
  });

  it("exposes data and indexes", async () => {
    render(<Harness session={sessionWith("active")} />);

    await waitFor(() => {
      expect(screen.getByTestId("appointments")).toHaveTextContent("false|-|1");
    });
    await waitFor(() => {
      expect(screen.getByTestId("prescriptions")).toHaveTextContent(
        "false|-|1",
      );
    });
    expect(screen.getByTestId("indexes")).toHaveTextContent(
      "0900A0000000001|0900A0000000009|0900A0000000002",
    );
  });

  it("keeps errors independent", async () => {
    respond({
      "/api/appointments": { error: {}, response: { status: 409 } },
      "/api/prescriptions": { data: prescriptionsBody },
    });
    render(<Harness session={sessionWith("active")} />);

    await waitFor(() => {
      expect(screen.getByTestId("appointments")).toHaveTextContent(
        "false|needs_gestione|-",
      );
    });
    await waitFor(() => {
      expect(screen.getByTestId("prescriptions")).toHaveTextContent(
        "false|-|1",
      );
    });
  });

  it("maps statuses per resource", async () => {
    respond({
      "/api/appointments": { error: {}, response: { status: 401 } },
      "/api/prescriptions": { error: {}, response: { status: 409 } },
    });
    render(<Harness session={sessionWith("active")} />);

    await waitFor(() => {
      expect(screen.getByTestId("appointments")).toHaveTextContent(
        "false|unauthorized|-",
      );
    });
    await waitFor(() => {
      expect(screen.getByTestId("prescriptions")).toHaveTextContent(
        "false|unavailable|-",
      );
    });
  });

  it("reports unavailable when a request throws", async () => {
    mocks.get.mockImplementation((path: string) =>
      path === "/api/prescriptions"
        ? Promise.reject(new Error("network"))
        : Promise.resolve({ data: appointmentsBody }),
    );
    render(<Harness session={sessionWith("active")} />);

    await waitFor(() => {
      expect(screen.getByTestId("prescriptions")).toHaveTextContent(
        "false|unavailable|-",
      );
    });
    await waitFor(() => {
      expect(screen.getByTestId("appointments")).toHaveTextContent("false|-|1");
    });
  });

  it("reloads each resource on its own", async () => {
    const user = userEvent.setup();
    render(<Harness session={sessionWith("active")} />);
    await waitFor(() => {
      expect(screen.getByTestId("prescriptions")).toHaveTextContent(
        "false|-|1",
      );
    });

    await user.click(
      screen.getByRole("button", { name: "reload-appointments" }),
    );
    await waitFor(() => {
      expect(callsTo(mocks.get, "/api/appointments")).toBe(2);
    });
    expect(callsTo(mocks.get, "/api/prescriptions")).toBe(1);

    await user.click(
      screen.getByRole("button", { name: "reload-prescriptions" }),
    );
    await waitFor(() => {
      expect(callsTo(mocks.get, "/api/prescriptions")).toBe(2);
    });
    expect(callsTo(mocks.get, "/api/appointments")).toBe(2);
  });

  it("refetches only the resources whose session key changed", async () => {
    const { rerender } = render(<Harness session={sessionWith("active")} />);
    await waitFor(() => {
      expect(screen.getByTestId("appointments")).toHaveTextContent("false|-|1");
    });

    rerender(<Harness session={sessionWith("active")} />);
    expect(callsTo(mocks.get, "/api/appointments")).toBe(1);
    expect(callsTo(mocks.get, "/api/prescriptions")).toBe(1);

    rerender(<Harness session={sessionWith("active", "expired")} />);
    await waitFor(() => {
      expect(callsTo(mocks.get, "/api/prescriptions")).toBe(2);
    });
    expect(callsTo(mocks.get, "/api/appointments")).toBe(1);

    rerender(<Harness session={sessionWith("active", "expired", false)} />);
    await waitFor(() => {
      expect(callsTo(mocks.get, "/api/appointments")).toBe(2);
    });
    expect(callsTo(mocks.get, "/api/prescriptions")).toBe(2);

    await act(async () => {
      rerender(<Harness session={sessionWith("expired", "expired", false)} />);
      await Promise.resolve();
    });
    await waitFor(() => {
      expect(callsTo(mocks.get, "/api/appointments")).toBe(3);
    });
    expect(callsTo(mocks.get, "/api/prescriptions")).toBe(3);
  });

  it("throws outside the provider", () => {
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    expect(() => render(<Probe />)).toThrow(
      "useHealthData must be used inside HealthDataProvider",
    );
  });
});
