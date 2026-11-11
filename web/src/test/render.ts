import { render as rtlRender, type RenderResult } from "@testing-library/react";
import type { ReactNode } from "react";

import { TestProviders } from "./TestProviders";

export function render(ui: ReactNode): RenderResult {
  return rtlRender(ui, { wrapper: TestProviders });
}
