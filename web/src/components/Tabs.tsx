import {
  useCallback,
  useRef,
  type JSX,
  type KeyboardEvent,
  type ReactNode,
} from "react";

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
  const buttons = useRef(new Map<TabId, HTMLButtonElement>());

  const active = tabs.find((tab) => tab.id === route.tab) ?? tabs[0];

  const select = useCallback((id: TabId): void => {
    window.location.hash = `#/${id}`;
    buttons.current.get(id)?.focus();
  }, []);

  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>): void => {
    const index = tabs.findIndex((tab) => tab.id === active?.id);
    const targets: Record<string, number> = {
      ArrowRight: (index + 1) % tabs.length,
      ArrowLeft: (index - 1 + tabs.length) % tabs.length,
      Home: 0,
      End: tabs.length - 1,
    };
    const target = tabs[targets[event.key] ?? -1];
    if (target === undefined) {
      return;
    }
    event.preventDefault();
    select(target.id);
  };

  return (
    <div className="tabs">
      <div role="tablist" aria-label="Sezioni" className="tablist">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            ref={(node) => {
              if (node === null) {
                buttons.current.delete(tab.id);
                return;
              }
              buttons.current.set(tab.id, node);
            }}
            type="button"
            role="tab"
            id={`tab-${tab.id}`}
            aria-selected={tab.id === active?.id}
            aria-controls={`panel-${tab.id}`}
            tabIndex={tab.id === active?.id ? 0 : -1}
            onClick={() => {
              select(tab.id);
            }}
            onKeyDown={handleKeyDown}
          >
            {tab.label}
          </button>
        ))}
      </div>
      {active !== undefined && (
        <div
          role="tabpanel"
          id={`panel-${active.id}`}
          aria-labelledby={`tab-${active.id}`}
          tabIndex={0}
          className="tabpanel"
        >
          {active.content}
        </div>
      )}
    </div>
  );
}
