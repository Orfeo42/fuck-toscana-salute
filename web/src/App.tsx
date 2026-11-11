import { AppShell, Burger, Group, Stack, Text } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { HeartPulse } from "lucide-react";
import type { JSX } from "react";

import { AppNav } from "./components/AppNav";
import { LoginViewer } from "./components/LoginViewer";
import { SessionMenu } from "./components/SessionMenu";
import { SessionNotice } from "./components/SessionNotice";
import { HealthDataProvider } from "./data/HealthDataProvider";
import { GoogleProvider } from "./google/GoogleProvider";
import { useRoute } from "./lib/route";
import { SessionProvider } from "./session/SessionProvider";
import { chromeColors } from "./theme";
import { DrugsView } from "./views/DrugsView";
import { LinkView } from "./views/LinkView";
import { ReportsView } from "./views/ReportsView";
import { VisitsView } from "./views/VisitsView";

const headerConfig = { height: 56 };

function Content(): JSX.Element {
  const route = useRoute();
  if (route.tab === "collega") {
    return <LinkView />;
  }
  if (route.tab === "referti") {
    return <ReportsView />;
  }
  return route.tab === "farmaci" ? <DrugsView /> : <VisitsView />;
}

export function App(): JSX.Element {
  const [opened, { toggle, close }] = useDisclosure();

  return (
    <SessionProvider>
      <GoogleProvider>
        <HealthDataProvider>
          <AppShell
            header={headerConfig}
            navbar={{
              width: 240,
              breakpoint: "sm",
              collapsed: { mobile: !opened },
            }}
            padding="md"
          >
            <AppShell.Header>
              <Group h="100%" px="md" justify="space-between" wrap="nowrap">
                <Group gap="sm">
                  <Burger
                    opened={opened}
                    onClick={toggle}
                    hiddenFrom="sm"
                    size="sm"
                    color="white"
                    aria-label={opened ? "Chiudi menu" : "Apri menu"}
                  />
                  <HeartPulse
                    size={22}
                    color={chromeColors.secondaryText}
                    aria-hidden="true"
                  />
                  <Text fw={700} size="lg" c="white">
                    Toscana Salute
                  </Text>
                  <Text
                    c={chromeColors.secondaryText}
                    size="sm"
                    visibleFrom="sm"
                  >
                    ma usabile
                  </Text>
                </Group>
                <SessionMenu />
              </Group>
            </AppShell.Header>
            <AppShell.Navbar p="sm">
              <AppNav onNavigate={close} />
            </AppShell.Navbar>
            <AppShell.Main>
              <Stack gap="md">
                <LoginViewer />
                <SessionNotice />
                <Content />
              </Stack>
            </AppShell.Main>
          </AppShell>
        </HealthDataProvider>
      </GoogleProvider>
    </SessionProvider>
  );
}
