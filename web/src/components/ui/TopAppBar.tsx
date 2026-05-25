import type { ReactNode } from 'react';

type TopAppBarProps = {
  title: string;
  navigation?: ReactNode;
  actions?: ReactNode;
};

export function TopAppBar({ title, navigation, actions }: TopAppBarProps) {
  return (
    <header className="md-top-app-bar top-bar">
      {navigation ? <div className="md-top-app-bar-navigation">{navigation}</div> : null}
      <h1>{title}</h1>
      {actions ? <div className="md-top-app-bar-actions top-actions" aria-label="页面操作">{actions}</div> : null}
    </header>
  );
}
