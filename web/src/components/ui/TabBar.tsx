import type { ComponentType } from 'react';

export type TabDefinition<T extends string> = {
  key: T;
  label: string;
  icon: ComponentType<{ size?: number; strokeWidth?: number }>;
};

type TabBarProps<T extends string> = {
  tabs: TabDefinition<T>[];
  activeTab: T;
  onChange: (tab: T) => void;
};

export function TabBar<T extends string>({ tabs, activeTab, onChange }: TabBarProps<T>) {
  return (
    <nav className="tab-bar" role="tablist" aria-label="主导航">
      {tabs.map((tab) => {
        const Icon = tab.icon;
        const selected = activeTab === tab.key;

        return (
          <button
            key={tab.key}
            type="button"
            role="tab"
            aria-selected={selected}
            className={selected ? 'tab-item active' : 'tab-item'}
            onClick={() => onChange(tab.key)}
          >
            <Icon size={24} strokeWidth={selected ? 2.6 : 2.1} />
            <span>{tab.label}</span>
          </button>
        );
      })}
    </nav>
  );
}
