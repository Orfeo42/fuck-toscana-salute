import { Box } from "@mantine/core";
import type { JSX, MouseEvent, ReactNode } from "react";

interface Props {
  children: ReactNode;
}

export function StopClick({ children }: Props): JSX.Element {
  return (
    <Box
      component="span"
      display="inline-block"
      onClick={(event: MouseEvent) => {
        event.stopPropagation();
      }}
    >
      {children}
    </Box>
  );
}
