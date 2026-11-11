import "@testing-library/jest-dom/vitest";
import { cleanNotificationsQueue, notifications } from "@mantine/notifications";
import { cleanup } from "@testing-library/react";
import { afterEach, vi } from "vitest";

const originalGetComputedStyle = window.getComputedStyle.bind(window);
window.getComputedStyle = (elt: Element, pseudoElt?: string | null) =>
  originalGetComputedStyle(elt, pseudoElt);

HTMLElement.prototype.scrollIntoView = () => undefined;

window.matchMedia = vi.fn().mockImplementation((query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addListener: vi.fn(),
  removeListener: vi.fn(),
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
  dispatchEvent: vi.fn(),
}));

class NoopResizeObserver implements ResizeObserver {
  observe(): void {
    return undefined;
  }
  unobserve(): void {
    return undefined;
  }
  disconnect(): void {
    return undefined;
  }
}

window.ResizeObserver = NoopResizeObserver;

if (!("fonts" in document)) {
  Object.defineProperty(document, "fonts", {
    value: { addEventListener: vi.fn(), removeEventListener: vi.fn() },
  });
}

afterEach(() => {
  cleanup();
  notifications.clean();
  cleanNotificationsQueue();
});
