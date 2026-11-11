import type { JSX } from "react";

import { HealthDataProvider } from "./data/HealthDataProvider";
import { SessionBar } from "./components/SessionBar";
import { Tabs, type TabItem } from "./components/Tabs";
import { SessionProvider } from "./session/SessionProvider";
import { DrugsView } from "./views/DrugsView";
import { VisitsView } from "./views/VisitsView";

const tabs: readonly TabItem[] = [
  { id: "visite", label: "Visite ed esami", content: <VisitsView /> },
  { id: "farmaci", label: "Farmaci", content: <DrugsView /> },
];

export function App(): JSX.Element {
  return (
    <SessionProvider>
      <HealthDataProvider>
        <header className="app-header">
          <h1>Toscana Salute, ma usabile</h1>
        </header>
        <main className="app-main">
          <SessionBar />
          <Tabs tabs={tabs} />
        </main>
      </HealthDataProvider>
    </SessionProvider>
  );
}
