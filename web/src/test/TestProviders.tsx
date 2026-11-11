import { MantineProvider } from "@mantine/core";
import { Notifications } from "@mantine/notifications";
import type { JSX, ReactNode } from "react";

import { theme } from "../theme";

interface Props {
  children: ReactNode;
}

export function TestProviders({ children }: Props): JSX.Element {
  return (
    <MantineProvider theme={theme} env="test">
      {children}
      <Notifications />
    </MantineProvider>
  );
}
