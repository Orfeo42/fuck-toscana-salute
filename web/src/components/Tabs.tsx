import {
  useCallback,
  useRef,
  useSyncExternalStore,
  type JSX,
  type KeyboardEvent,
  type ReactNode,
} from "react";

export interface TabItem {
  id: string;
  label: string;
  content: ReactNode;
}

interface Props {
  tabs: readonly TabItem[];
}

const hashPrefix = "#/";

function subscribe(onChange: () => void): () => void {
  window.addEventListener("hashchange", onChange);
  return () => {
    window.removeEventListener("hashchange", onChange);
  };
}

function currentHash(): string {
  return window.location.hash;
}

export function Tabs({ tabs }: Props): JSX.Element {
  const hash = useSyncExternalStore(subscribe, currentHash);
  const buttons = useRef(new Map<string, HTMLButtonElement>());

  const hashId = hash.startsWith(hashPrefix)
    ? hash.slice(hashPrefix.length)
    : "";
  const active = tabs.find((tab) => tab.id === hashId) ?? tabs[0];

  const select = useCallback((id: string): void => {
    window.location.hash = `${hashPrefix}${id}`;
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
