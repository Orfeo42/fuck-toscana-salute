import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Copy } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { IconAction } from "./IconAction";
import { render } from "../test/render";

describe("IconAction", () => {
  it("renders an external link with its accessible name", () => {
    render(
      <IconAction
        label="Apri"
        icon={Copy}
        href="https://example.com/"
        external
      />,
    );

    const link = screen.getByRole("link", { name: "Apri" });
    expect(link).toHaveAttribute("href", "https://example.com/");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("omits target and rel on an internal link", () => {
    render(<IconAction label="Apri" icon={Copy} href="/x" />);

    const link = screen.getByRole("link", { name: "Apri" });
    expect(link).not.toHaveAttribute("target");
    expect(link).not.toHaveAttribute("rel");
  });

  it("renders a button that calls onClick", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(<IconAction label="Copia" icon={Copy} onClick={onClick} />);

    await user.click(screen.getByRole("button", { name: "Copia" }));

    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("hides the icon from assistive tech", () => {
    render(<IconAction label="Copia" icon={Copy} onClick={vi.fn()} />);

    const button = screen.getByRole("button", { name: "Copia" });
    expect(button.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
  });

  it("shows the label as a tooltip on hover", async () => {
    const user = userEvent.setup();
    render(<IconAction label="Copia" icon={Copy} onClick={vi.fn()} />);

    await user.hover(screen.getByRole("button", { name: "Copia" }));

    expect(await screen.findByRole("tooltip")).toHaveTextContent("Copia");
  });

  it("renders a labelled button with visible text", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(<IconAction label="Copia" icon={Copy} onClick={onClick} labelled />);

    const button = screen.getByRole("button", { name: "Copia" });
    expect(button).toHaveTextContent("Copia");
    await user.click(button);
    expect(onClick).toHaveBeenCalledOnce();
  });

  it("renders a labelled external link with visible text", () => {
    render(
      <IconAction
        label="Apri"
        icon={Copy}
        href="https://example.com/"
        external
        labelled
      />,
    );

    const link = screen.getByRole("link", { name: "Apri" });
    expect(link).toHaveTextContent("Apri");
    expect(link).toHaveAttribute("href", "https://example.com/");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });
});
