import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { App } from "./App";
import { render } from "./test/render";

vi.mock("./session/SessionProvider", () => ({
  SessionProvider: ({ children }: { children: React.ReactNode }) => children,
}));
vi.mock("./google/GoogleProvider", () => ({
  GoogleProvider: ({ children }: { children: React.ReactNode }) => children,
}));
vi.mock("./data/HealthDataProvider", () => ({
  HealthDataProvider: ({ children }: { children: React.ReactNode }) => children,
}));
vi.mock("./components/AppNav", () => ({ AppNav: () => <nav>menu</nav> }));
vi.mock("./components/LoginViewer", () => ({ LoginViewer: () => null }));
vi.mock("./components/SessionMenu", () => ({ SessionMenu: () => null }));
vi.mock("./components/SessionNotice", () => ({ SessionNotice: () => null }));
vi.mock("./views/VisitsView", () => ({ VisitsView: () => <p>visite</p> }));
vi.mock("./views/DrugsView", () => ({ DrugsView: () => <p>farmaci</p> }));
vi.mock("./views/ReportsView", () => ({ ReportsView: () => <p>referti</p> }));
vi.mock("./views/LinkView", () => ({ LinkView: () => <p>collega</p> }));

describe("App", () => {
  beforeEach(() => {
    window.location.hash = "";
  });

  it("renders header, navbar and content", () => {
    render(<App />);

    expect(screen.getByRole("banner")).toHaveTextContent("Toscana Salute");
    expect(screen.getByText("menu")).toBeInTheDocument();
    expect(screen.getByText("visite")).toBeInTheDocument();
  });

  it("renders the reports view on the referti route", () => {
    window.location.hash = "#/referti";
    render(<App />);

    expect(screen.getByText("referti")).toBeInTheDocument();
    expect(screen.queryByText("visite")).not.toBeInTheDocument();
  });
});
