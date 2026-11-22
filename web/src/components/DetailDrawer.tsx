import { Badge, Drawer, Group, Stack, Tabs, Text } from "@mantine/core";
import { useMediaQuery } from "@mantine/hooks";
import { useState } from "react";
import type { JSX, ReactNode } from "react";

export interface DetailTab {
  value: string;
  label: string;
  count?: number | undefined;
  content: ReactNode;
}

interface Props {
  title: string;
  badge?: ReactNode;
  summary?: ReactNode;
  actions?: ReactNode;
  tabs: readonly DetailTab[];
  footer?: ReactNode;
  onClose: () => void;
}

const desktopWidth = 560;

export function DetailDrawer({
  title,
  badge,
  summary,
  actions,
  tabs,
  footer,
  onClose,
}: Props): JSX.Element {
  const phone = useMediaQuery("(max-width: 48em)");
  const [active, setActive] = useState<string | null>(tabs[0]?.value ?? null);

  return (
    <Drawer.Root
      opened
      onClose={onClose}
      position="right"
      size={phone ? "100%" : desktopWidth}
    >
      <Drawer.Overlay />
      <Drawer.Content>
        <Tabs value={active} onChange={setActive}>
          <Drawer.Header
            style={{
              flexDirection: "column",
              alignItems: "stretch",
              gap: "var(--mantine-spacing-sm)",
              paddingBottom: 0,
              borderBottom: "1px solid var(--mantine-color-default-border)",
            }}
          >
            <Group justify="space-between" align="flex-start" wrap="nowrap">
              <Stack gap={6} style={{ minWidth: 0 }}>
                <Drawer.Title fw={600} fz="lg">
                  {title}
                </Drawer.Title>
                {badge === undefined ? null : <div>{badge}</div>}
              </Stack>
              <Drawer.CloseButton aria-label="Chiudi" />
            </Group>
            {summary === undefined ? null : <Stack gap={2}>{summary}</Stack>}
            {actions === undefined ? null : <Group gap="xs">{actions}</Group>}
            {tabs.length > 1 ? (
              <Tabs.List>
                {tabs.map((tab) => (
                  <Tabs.Tab
                    key={tab.value}
                    value={tab.value}
                    rightSection={
                      tab.count === undefined ? undefined : (
                        <Badge size="xs" variant="light" circle>
                          {tab.count}
                        </Badge>
                      )
                    }
                  >
                    {tab.label}
                  </Tabs.Tab>
                ))}
              </Tabs.List>
            ) : null}
          </Drawer.Header>
          <Drawer.Body pt="md">
            {tabs.map((tab) => (
              <Tabs.Panel key={tab.value} value={tab.value}>
                <Stack gap="lg">{tab.content}</Stack>
              </Tabs.Panel>
            ))}
            {footer === undefined ? null : (
              <Text component="div" size="xs" c="dimmed" mt="lg">
                {footer}
              </Text>
            )}
          </Drawer.Body>
        </Tabs>
      </Drawer.Content>
    </Drawer.Root>
  );
}
