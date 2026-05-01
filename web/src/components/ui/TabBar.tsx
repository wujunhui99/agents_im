import type { ComponentType } from 'react';
import { NavigationBar } from './NavigationBar';

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
  return <NavigationBar tabs={tabs} activeTab={activeTab} onChange={onChange} />;
}
