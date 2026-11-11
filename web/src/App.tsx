import { Container, Stack, Title } from "@mantine/core";
import type { JSX } from "react";

import { HealthDataProvider } from "./data/HealthDataProvider";
import { SessionBar } from "./components/SessionBar";
import { Tabs, type TabItem } from "./components/Tabs";
import { GoogleProvider } from "./google/GoogleProvider";
import { SessionProvider } from "./session/SessionProvider";
import { DrugsView } from "./views/DrugsView";
import { VisitsView } from "./views/VisitsView";

const tabs: readonly TabItem[] = [
  { id: "visite", label: "Visite ed esami", content: <VisitsView /> },
  { id: "farmaci", label: "Farmaci", content: <DrugsView /> },
];

const containerPadding = { base: 16, md: 24 };

export function App(): JSX.Element {
  return (
    <SessionProvider>
      <GoogleProvider>
        <HealthDataProvider>
          <Container fluid px={containerPadding}>
            <Stack gap="md" py="md">
              <Title order={1}>Toscana Salute, ma usabile</Title>
              <SessionBar />
              <Tabs tabs={tabs} />
            </Stack>
          </Container>
        </HealthDataProvider>
      </GoogleProvider>
    </SessionProvider>
  );
}
