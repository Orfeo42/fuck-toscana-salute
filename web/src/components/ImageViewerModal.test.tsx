import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { render } from "../test/render";
import { ImageViewerModal } from "./ImageViewerModal";

describe("ImageViewerModal", () => {
  it("shows the image with the title as alt text", () => {
    render(
      <ImageViewerModal
        url="data:image/png;base64,AAAA"
        title="Radiografia"
        opened
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByRole("img", { name: "Radiografia" })).toHaveAttribute(
      "src",
      "data:image/png;base64,AAAA",
    );
  });

  it("calls onClose from the close button", async () => {
    const onClose = vi.fn();
    render(
      <ImageViewerModal
        url="data:image/png;base64,AAAA"
        title="Radiografia"
        opened
        onClose={onClose}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Chiudi" }));

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("renders nothing when closed", () => {
    render(
      <ImageViewerModal
        url="data:image/png;base64,AAAA"
        title="Radiografia"
        opened={false}
        onClose={vi.fn()}
      />,
    );

    expect(screen.queryByRole("img")).toBeNull();
  });
});
