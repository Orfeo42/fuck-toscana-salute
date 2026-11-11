import { act, screen } from "@testing-library/react";
import type { JSX } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { TabId } from "./route";
import { focusId, useFocusTarget } from "./useFocusTarget";
import { render } from "../test/render";

interface Props {
  kind: TabId;
  nre: string;
  ready?: boolean;
}

function Card({ kind, nre, ready = true }: Props): JSX.Element {
  const focus = useFocusTarget(kind, ready);
  return (
    <div id={focusId(kind, nre)} tabIndex={-1}>
      <span>{`focus:${focus ?? "none"}`}</span>
      card
    </div>
  );
}

function cardEl(): HTMLElement | null {
  return document.querySelector("[tabindex='-1']");
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
    expect(focusId("farmaci", "0900a0000000001")).toBe(
      "farmaci-0900A0000000001",
    );
  });

  it("returns the focused NRE for the matching tab", () => {
    window.location.hash = "#/farmaci/0900a0000000001";
    render(<Card kind="farmaci" nre="0900A0000000001" />);

    expect(screen.getByText("focus:0900A0000000001")).toBeInTheDocument();
  });

  it("scrolls and focuses on a matching hash", () => {
    window.location.hash = "#/farmaci/0900A0000000001";
    render(<Card kind="farmaci" nre="0900A0000000001" />);

    expect(scrollIntoView).toHaveBeenCalledWith({ block: "center" });
    expect(cardEl()).toHaveFocus();
  });

  it("reacts to a later hashchange", () => {
    render(<Card kind="visite" nre="0900A0000000001" />);
    expect(scrollIntoView).not.toHaveBeenCalled();

    go("#/visite/0900a0000000001");

    expect(scrollIntoView).toHaveBeenCalledTimes(1);
    expect(cardEl()).toHaveFocus();
  });

  it("does nothing on the wrong tab", () => {
    window.location.hash = "#/visite/0900A0000000001";
    render(<Card kind="farmaci" nre="0900A0000000001" />);

    expect(scrollIntoView).not.toHaveBeenCalled();
    expect(cardEl()).not.toHaveFocus();
  });

  it("does nothing for another NRE", () => {
    window.location.hash = "#/farmaci/0900A0000000002";
    render(<Card kind="farmaci" nre="0900A0000000001" />);

    expect(scrollIntoView).not.toHaveBeenCalled();
    expect(cardEl()).not.toHaveFocus();
  });

  it("waits until ready before focusing", () => {
    window.location.hash = "#/farmaci/0900A0000000001";
    const view = render(
      <Card kind="farmaci" nre="0900A0000000001" ready={false} />,
    );
    expect(scrollIntoView).not.toHaveBeenCalled();

    view.rerender(<Card kind="farmaci" nre="0900A0000000001" ready />);

    expect(scrollIntoView).toHaveBeenCalledTimes(1);
  });

  it("still focuses when scrollIntoView is missing", () => {
    Reflect.deleteProperty(HTMLElement.prototype, "scrollIntoView");
    window.location.hash = "#/farmaci/0900A0000000001";
    render(<Card kind="farmaci" nre="0900A0000000001" />);

    expect(cardEl()).toHaveFocus();
  });
});
