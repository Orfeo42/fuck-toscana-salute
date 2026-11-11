import type { JSX } from "react";

import { HealthDataProvider } from "./data/HealthDataProvider";
import { SessionBar } from "./components/SessionBar";
import { Tabs, type TabItem } from "./components/Tabs";
import { SessionProvider } from "./session/SessionProvider";
import { AppointmentsView } from "./views/AppointmentsView";
import { PrescriptionsView } from "./views/PrescriptionsView";

const tabs: readonly TabItem[] = [
  {
    id: "appuntamenti",
    label: "Appuntamenti",
    content: <AppointmentsView />,
  },
  { id: "ricette", label: "Ricette", content: <PrescriptionsView /> },
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
