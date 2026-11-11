import { act, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { CopyButton } from "./CopyButton";
import { render } from "../test/render";

describe("CopyButton", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("renders the value inside the button", () => {
    render(<CopyButton value="0900A0000000001" label="Copia NRE" />);

    const button = screen.getByRole("button", {
      name: "Copia NRE 0900A0000000001",
    });
    expect(button).toHaveTextContent("0900A0000000001");
    expect(button.querySelector("svg")).toHaveClass("lucide-copy");
    expect(screen.queryByText("NRE copiato")).not.toBeInTheDocument();
  });

  it("shows the label as a tooltip on hover", async () => {
    const user = userEvent.setup();
    render(<CopyButton value="0900A0000000001" label="Copia NRE" />);

    await user.hover(
      screen.getByRole("button", { name: "Copia NRE 0900A0000000001" }),
    );

    expect(await screen.findByRole("tooltip")).toHaveTextContent("Copia NRE");
  });

  it("copies the value and confirms", async () => {
    const user = userEvent.setup();
    const writeText = vi
      .spyOn(navigator.clipboard, "writeText")
      .mockResolvedValue();
    render(<CopyButton value="0900A0000000001" label="Copia NRE" />);

    await user.click(
      screen.getByRole("button", { name: "Copia NRE 0900A0000000001" }),
    );

    const button = screen.getByRole("button", { name: /Copia NRE/ });
    expect(writeText).toHaveBeenCalledWith("0900A0000000001");
    expect(await screen.findByText("NRE copiato")).toBeInTheDocument();
    expect(button.querySelector("svg")).toHaveClass("lucide-check");
  });

  it("reverts to the idle state after 2 seconds", async () => {
    vi.useFakeTimers();
    vi.stubGlobal("jest", { advanceTimersByTime: vi.advanceTimersByTime });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    vi.spyOn(navigator.clipboard, "writeText").mockResolvedValue();
    render(<CopyButton value="0900A0000000001" label="Copia NRE" />);

    await user.click(
      screen.getByRole("button", { name: "Copia NRE 0900A0000000001" }),
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    const button = screen.getByRole("button", { name: /Copia NRE/ });
    expect(button.querySelector("svg")).toHaveClass("lucide-check");

    act(() => {
      vi.advanceTimersByTime(2000);
    });

    expect(button.querySelector("svg")).toHaveClass("lucide-copy");
  });

  it("reports a failed copy", async () => {
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, "writeText").mockRejectedValue(
      new Error("denied"),
    );
    render(<CopyButton value="0900A0000000001" label="Copia NRE" />);

    await user.click(
      screen.getByRole("button", { name: "Copia NRE 0900A0000000001" }),
    );

    const button = screen.getByRole("button", { name: /Copia NRE/ });
    expect(await screen.findByText("Copia non riuscita")).toBeInTheDocument();
    expect(button.querySelector("svg")).toHaveClass("lucide-x");
  });
});
