import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { CopyButton } from "./CopyButton";

describe("CopyButton", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("copies the value and confirms", async () => {
    const user = userEvent.setup();
    const writeText = vi
      .spyOn(navigator.clipboard, "writeText")
      .mockResolvedValue();
    render(<CopyButton value="0900A0000000001" label="Copia NRE" />);

    await user.click(screen.getByRole("button", { name: "Copia NRE" }));

    expect(writeText).toHaveBeenCalledWith("0900A0000000001");
    expect(screen.getByRole("status")).toHaveTextContent("Copiato");
  });

  it("reports a failed copy", async () => {
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, "writeText").mockRejectedValue(
      new Error("denied"),
    );
    render(<CopyButton value="0900A0000000001" label="Copia NRE" />);

    await user.click(screen.getByRole("button", { name: "Copia NRE" }));

    expect(screen.getByRole("status")).toHaveTextContent("Copia non riuscita");
  });
});
