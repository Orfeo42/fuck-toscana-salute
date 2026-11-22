import { Group, Stack, Text, Title } from "@mantine/core";
import type { JSX, ReactNode } from "react";

interface SectionProps {
  title: string;
  actions?: ReactNode;
  children: ReactNode;
}

export function DetailSection({
  title,
  actions,
  children,
}: SectionProps): JSX.Element {
  return (
    <Stack gap="xs">
      <Group justify="space-between" wrap="nowrap" gap="xs">
        <Title order={5}>{title}</Title>
        {actions === undefined ? null : (
          <Group gap={2} wrap="nowrap">
            {actions}
          </Group>
        )}
      </Group>
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
