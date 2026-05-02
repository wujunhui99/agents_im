import type { ComponentType } from 'react';

export type NavigationTabDefinition<T extends string> = {
  key: T;
  label: string;
  icon: ComponentType<{ size?: number; strokeWidth?: number }>;
};

type NavigationBarProps<T extends string> = {
  tabs: NavigationTabDefinition<T>[];
  activeTab: T;
  onChange: (tab: T) => void;
};

export function NavigationBar<T extends string>({ tabs, activeTab, onChange }: NavigationBarProps<T>) {
  return (
    <nav className="md-navigation-bar tab-bar" role="tablist" aria-label="主导航">
      {tabs.map((tab) => {
        const Icon = tab.icon;
        const selected = activeTab === tab.key;

        return (
          <button
            key={tab.key}
            type="button"
            role="tab"
            aria-selected={selected}
            className={selected ? 'md-navigation-item tab-item active' : 'md-navigation-item tab-item'}
            onClick={() => onChange(tab.key)}
          >
            <span className="md-navigation-indicator">
              <Icon size={22} strokeWidth={selected ? 2.6 : 2.1} />
            </span>
            <span className="md-navigation-label">{tab.label}</span>
          </button>
        );
      })}
    </nav>
  );
}
