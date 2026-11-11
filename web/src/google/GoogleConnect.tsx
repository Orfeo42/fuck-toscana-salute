import { Alert, Badge, Button, Group, Text } from "@mantine/core";
import { CalendarPlus } from "lucide-react";
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
      <Alert color="red" role="alert">
        {error}
      </Alert>
    );
  }
  if (!status.configured) {
    return (
      <Text size="sm" c="dimmed" title={notConfiguredHint}>
        Google Calendar non configurato
      </Text>
    );
  }
  if (!status.connected) {
    return (
      <Group gap="xs">
        <Button
          variant="default"
          leftSection={<CalendarPlus size={16} />}
          onClick={handleConnect}
        >
          Collega Google Calendar
        </Button>
        {error === undefined ? null : (
          <Text size="sm" c="red" role="alert">
            {error}
          </Text>
        )}
      </Group>
    );
  }
  return (
    <Group gap="xs">
      <Badge color="teal" variant="light">
        Google Calendar collegato
      </Badge>
      <Button variant="subtle" size="compact-sm" onClick={handleDisconnect}>
        Scollega
      </Button>
    </Group>
  );
}
