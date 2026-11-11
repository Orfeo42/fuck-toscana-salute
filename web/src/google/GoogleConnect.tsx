import { Menu, Text } from "@mantine/core";
import { CalendarCheck, CalendarPlus } from "lucide-react";
import type { JSX } from "react";

import { notConfiguredHint, useGoogle } from "./useGoogle";

export function GoogleConnect(): JSX.Element | null {
  const { status, error, connect, disconnect } = useGoogle();

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
      <Text size="xs" c="dimmed" px="sm" title={notConfiguredHint}>
        Google Calendar non configurato
      </Text>
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
