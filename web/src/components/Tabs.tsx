import { Tabs as MantineTabs } from "@mantine/core";
import type { JSX, ReactNode } from "react";

import { useRoute, type TabId } from "../lib/route";

export interface TabItem {
  id: TabId;
  label: string;
  content: ReactNode;
}

interface Props {
  tabs: readonly TabItem[];
}

export function Tabs({ tabs }: Props): JSX.Element {
  const route = useRoute();

  const active = tabs.find((tab) => tab.id === route.tab) ?? tabs[0];

  const select = (value: string | null): void => {
    const target = tabs.find((tab) => tab.id === value);
    if (target === undefined) {
      return;
    }
    window.location.hash = `#/${target.id}`;
  };

  return (
    <MantineTabs
      value={active?.id ?? null}
      onChange={select}
      keepMounted={false}
    >
      <MantineTabs.List aria-label="Sezioni">
        {tabs.map((tab) => (
          <MantineTabs.Tab key={tab.id} value={tab.id}>
            {tab.label}
          </MantineTabs.Tab>
        ))}
      </MantineTabs.List>
      {tabs.map((tab) => (
        <MantineTabs.Panel key={tab.id} value={tab.id} pt="md">
          {tab.content}
        </MantineTabs.Panel>
      ))}
    </MantineTabs>
  );
}
