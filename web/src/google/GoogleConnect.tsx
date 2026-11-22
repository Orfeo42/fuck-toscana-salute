import { Button, Group, Menu, Text } from "@mantine/core";
import { CalendarCheck, LogIn } from "lucide-react";
import type { JSX } from "react";

import { useGoogle } from "./useGoogle";

export function GoogleConnect(): JSX.Element | null {
  const { status, error, signIn, disconnect, openSettings } = useGoogle();

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
          leftSection={<LogIn size={16} />}
          onClick={() => {
            void signIn();
          }}
        >
          Accedi con Google
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
        Scollega Google
      </Menu.Item>
    </>
  );
}
