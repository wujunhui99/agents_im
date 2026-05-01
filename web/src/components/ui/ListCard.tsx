import type { ReactNode } from 'react';
import { Card } from './Card';

type ListCardProps = {
  children: ReactNode;
  ariaLabel?: string;
  className?: string;
};

export function ListCard({ children, ariaLabel, className = '' }: ListCardProps) {
  const classes = ['list-card', className].filter(Boolean).join(' ');

  return (
    <Card className={classes} ariaLabel={ariaLabel} variant="surface">
      {children}
    </Card>
  );
}
