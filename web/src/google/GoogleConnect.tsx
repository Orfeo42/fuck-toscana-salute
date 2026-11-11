import { Button, Group, Menu, Text } from "@mantine/core";
import { CalendarCheck, CalendarPlus } from "lucide-react";
import type { JSX } from "react";

import { useGoogle } from "./useGoogle";

export function GoogleConnect(): JSX.Element | null {
  const { status, error, connect, disconnect, openSettings } = useGoogle();

  const handleConnect = (): void => {
    void connect();
  };
  const handleDisconnect = (): void => {
    void disconnect();
  };

  if (status === undefined) {
    return error === undefined ? null : (
      <Text size="sm" c="red" role="alert" px="sm">
        {error}
      </Text>
    );
  }
  if (!status.configured) {
    return (
      <Group justify="space-between" px="sm" py={4} wrap="nowrap">
        <Text size="xs" c="dimmed">
          Google non è configurato
        </Text>
        <Button size="compact-xs" variant="light" onClick={openSettings}>
          Configura
        </Button>
      </Group>
    );
  }
  if (!status.connected) {
    return (
      <>
        <Menu.Item
          leftSection={<CalendarPlus size={16} />}
          onClick={handleConnect}
        >
          Collega Google Calendar
        </Menu.Item>
        {error === undefined ? null : (
          <Text size="xs" c="red" role="alert" px="sm">
            {error}
          </Text>
        )}
      </>
    );
  }
  return (
    <>
      <Text size="xs" c="teal" px="sm">
        Collegato
      </Text>
      <Menu.Item
        leftSection={
          <CalendarCheck size={16} color="var(--mantine-color-teal-filled)" />
        }
        onClick={handleDisconnect}
      >
        Scollega Google Calendar
      </Menu.Item>
    </>
  );
}
