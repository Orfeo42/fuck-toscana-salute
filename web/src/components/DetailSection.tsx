import { Stack, Text, Title } from "@mantine/core";
import type { JSX, ReactNode } from "react";

interface SectionProps {
  title: string;
  children: ReactNode;
}

export function DetailSection({ title, children }: SectionProps): JSX.Element {
  return (
    <Stack gap="xs">
      <Title order={5}>{title}</Title>
      {children}
    </Stack>
  );
}

interface FieldProps {
  label: string;
  children: ReactNode;
}

export function DetailField({ label, children }: FieldProps): JSX.Element {
  return (
    <div>
      <Text size="xs" c="dimmed">
        {label}
      </Text>
      {typeof children === "string" ? <Text>{children}</Text> : children}
    </div>
  );
}
