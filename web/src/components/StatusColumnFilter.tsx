import { Button, Checkbox, Group, Stack } from "@mantine/core";
import type { JSX } from "react";

export interface StatusOption {
  value: string;
  label: string;
  count: number;
}

interface Props {
  options: readonly StatusOption[];
  selected: readonly string[];
  onChange: (next: string[]) => void;
}

export function StatusColumnFilter({
  options,
  selected,
  onChange,
}: Props): JSX.Element {
  return (
    <Stack gap="xs">
      <Group gap="xs">
        <Button
          size="compact-xs"
          variant="light"
          onClick={() => {
            onChange(options.map((option) => option.value));
          }}
        >
          Tutti
        </Button>
        <Button
          size="compact-xs"
          variant="light"
          onClick={() => {
            onChange([]);
          }}
        >
          Nessuno
        </Button>
      </Group>
      <Checkbox.Group value={[...selected]} onChange={onChange}>
        <Stack gap="xs">
          {options.map((option) => (
            <Checkbox
              key={option.value}
              value={option.value}
              label={`${option.label} (${String(option.count)})`}
            />
          ))}
        </Stack>
      </Checkbox.Group>
    </Stack>
  );
}
