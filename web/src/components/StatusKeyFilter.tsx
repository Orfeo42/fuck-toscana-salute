import { Button, Checkbox, Group, Stack } from "@mantine/core";
import type { JSX } from "react";

import type { StatusOptionBody } from "../api/client";

interface Props {
  options: readonly StatusOptionBody[];
  onChange: (excludedKeys: string[]) => void;
}

export function StatusKeyFilter({ options, onChange }: Props): JSX.Element {
  const shown = options
    .filter((option) => !option.excluded)
    .map((option) => option.key);

  return (
    <Stack gap="xs">
      <Group gap="xs">
        <Button
          size="compact-xs"
          variant="light"
          onClick={() => {
            onChange([]);
          }}
        >
          Tutti
        </Button>
        <Button
          size="compact-xs"
          variant="light"
          onClick={() => {
            onChange(options.map((option) => option.key));
          }}
        >
          Nessuno
        </Button>
      </Group>
      <Checkbox.Group
        value={shown}
        onChange={(next) => {
          onChange(
            options
              .map((option) => option.key)
              .filter((key) => !next.includes(key)),
          );
        }}
      >
        <Stack gap="xs">
          {options.map((option) => (
            <Checkbox
              key={option.key}
              value={option.key}
              label={`${option.label} (${String(option.count)})`}
            />
          ))}
        </Stack>
      </Checkbox.Group>
    </Stack>
  );
}
