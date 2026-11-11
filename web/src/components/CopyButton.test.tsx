import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { CopyButton } from "./CopyButton";

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
    expect(button.querySelector(".icon-tip")).toHaveTextContent("Copia NRE");
    expect(button.querySelector("svg")).toHaveClass("lucide-copy");
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
    expect(screen.getByRole("status")).toHaveTextContent("Copiato");
    expect(button.querySelector("svg")).toHaveClass("lucide-check");
    expect(button.querySelector(".icon-tip")).toHaveTextContent("Copiato");
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
    expect(button.querySelector(".icon-tip")).toHaveTextContent("Copia NRE");
    expect(screen.getByRole("status")).toBeEmptyDOMElement();
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
    expect(screen.getByRole("status")).toHaveTextContent("Copia non riuscita");
    expect(button.querySelector("svg")).toHaveClass("lucide-x");
    expect(button.querySelector(".icon-tip")).toHaveTextContent(
      "Copia non riuscita",
    );
  });
});
