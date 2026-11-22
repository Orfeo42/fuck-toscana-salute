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

  it("submits a form as a submit button", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn((event: Event) => {
      event.preventDefault();
    });
    render(
      <form
        onSubmit={(event) => {
          onSubmit(event.nativeEvent);
        }}
      >
        <IconAction label="Salva" icon={Copy} type="submit" tone="primary" />
      </form>,
    );

    await user.click(screen.getByRole("button", { name: "Salva" }));

    expect(onSubmit).toHaveBeenCalledOnce();
  });

  it("does not call onClick when disabled", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(
      <IconAction
        label="Elimina"
        icon={Copy}
        tone="danger"
        disabled
        onClick={onClick}
      />,
    );

    const button = screen.getByRole("button", { name: "Elimina" });
    expect(button).toBeDisabled();
    await user.click(button);
    expect(onClick).not.toHaveBeenCalled();
  });

  it("defaults to a plain button type", () => {
    render(<IconAction label="Copia" icon={Copy} onClick={vi.fn()} />);

    expect(screen.getByRole("button", { name: "Copia" })).toHaveAttribute(
      "type",
      "button",
    );
  });
});
