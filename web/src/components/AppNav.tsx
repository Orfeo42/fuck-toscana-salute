import { Box, NavLink, Stack, Text } from "@mantine/core";
import { Pill, Stethoscope, type LucideIcon } from "lucide-react";
import type { JSX } from "react";

import { useRoute, type TabId } from "../lib/route";
import { chromeColors } from "../theme";

interface NavItem {
  id: TabId;
  label: string;
  icon: LucideIcon;
}

interface Props {
  onNavigate: () => void;
}

const items: readonly NavItem[] = [
  { id: "visite", label: "Visite ed esami", icon: Stethoscope },
  { id: "farmaci", label: "Farmaci", icon: Pill },
];

export function AppNav({ onNavigate }: Props): JSX.Element {
  const route = useRoute();

  return (
    <Stack h="100%" justify="space-between" gap="md">
      <Box component="nav" aria-label="Sezioni">
        <Stack gap={4}>
          {items.map(({ id, label, icon: Icon }) => {
            const active = route.tab === id;
            return (
              <NavLink
                key={id}
                component="a"
                href={`#/${id}`}
                label={label}
                leftSection={<Icon size={18} aria-hidden="true" />}
                active={active}
                onClick={onNavigate}
                {...(active ? { "aria-current": "page" as const } : {})}
              />
            );
          })}
        </Stack>
      </Box>
      <Text size="xs" c={chromeColors.secondaryText}>
        Dati letti in tempo reale dai portali della Regione Toscana. Sul server
        restano solo la sessione di accesso (cifrata) e il profilo del browser
        di accesso.
      </Text>
    </Stack>
  );
}
