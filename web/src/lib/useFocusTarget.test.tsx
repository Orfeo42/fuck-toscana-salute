import { act, render, screen } from "@testing-library/react";
import type { JSX } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { focusId, useFocusTarget, type FocusKind } from "./useFocusTarget";

interface Props {
  kind: FocusKind;
  nre: string;
}

function Card({ kind, nre }: Props): JSX.Element {
  const { id, ref, tabIndex, focused } = useFocusTarget<HTMLDivElement>(
    kind,
    nre,
  );
  return (
    <div id={id} ref={ref} tabIndex={tabIndex} data-focused={String(focused)}>
      card
    </div>
  );
}

function go(hash: string): void {
  act(() => {
    window.location.hash = hash;
    window.dispatchEvent(new HashChangeEvent("hashchange"));
  });
}

describe("useFocusTarget", () => {
  const scrollIntoView = vi.fn();

  beforeEach(() => {
    window.location.hash = "";
    scrollIntoView.mockReset();
    Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
      configurable: true,
      writable: true,
      value: scrollIntoView,
    });
  });

  afterEach(() => {
    Reflect.deleteProperty(HTMLElement.prototype, "scrollIntoView");
  });

  it("builds the id from kind and upper-cased NRE", () => {
    expect(focusId("ricetta", "0900a0000000001")).toBe(
      "ricetta-0900A0000000001",
    );
    render(<Card kind="appuntamento" nre="0900a0000000001" />);
    expect(screen.getByText("card")).toHaveAttribute(
      "id",
      "appuntamento-0900A0000000001",
    );
    expect(screen.getByText("card")).toHaveAttribute("tabindex", "-1");
  });

  it("scrolls and focuses on a matching hash", () => {
    window.location.hash = "#/ricette/0900A0000000001";
    render(<Card kind="ricetta" nre="0900A0000000001" />);

    expect(scrollIntoView).toHaveBeenCalledWith({ block: "center" });
    expect(screen.getByText("card")).toHaveFocus();
    expect(screen.getByText("card")).toHaveAttribute("data-focused", "true");
  });

  it("reacts to a later hashchange", () => {
    render(<Card kind="appuntamento" nre="0900A0000000001" />);
    expect(scrollIntoView).not.toHaveBeenCalled();

    go("#/appuntamenti/0900a0000000001");

    expect(scrollIntoView).toHaveBeenCalledTimes(1);
    expect(screen.getByText("card")).toHaveFocus();
  });

  it("does nothing on the wrong tab", () => {
    window.location.hash = "#/appuntamenti/0900A0000000001";
    render(<Card kind="ricetta" nre="0900A0000000001" />);

    expect(scrollIntoView).not.toHaveBeenCalled();
    expect(screen.getByText("card")).not.toHaveFocus();
    expect(screen.getByText("card")).toHaveAttribute("data-focused", "false");
  });

  it("does nothing for another NRE", () => {
    window.location.hash = "#/ricette/0900A0000000002";
    render(<Card kind="ricetta" nre="0900A0000000001" />);

    expect(scrollIntoView).not.toHaveBeenCalled();
    expect(screen.getByText("card")).toHaveAttribute("data-focused", "false");
  });

  it("still focuses when scrollIntoView is missing", () => {
    Reflect.deleteProperty(HTMLElement.prototype, "scrollIntoView");
    window.location.hash = "#/ricette/0900A0000000001";
    render(<Card kind="ricetta" nre="0900A0000000001" />);

    expect(screen.getByText("card")).toHaveFocus();
  });
});
