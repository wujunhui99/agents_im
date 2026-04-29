import type { ReactNode } from 'react';

type ListCardProps = {
  children: ReactNode;
  ariaLabel?: string;
  className?: string;
};

export function ListCard({ children, ariaLabel, className = '' }: ListCardProps) {
  const classes = ['list-card', className].filter(Boolean).join(' ');

  return (
    <section className={classes} aria-label={ariaLabel}>
      {children}
    </section>
  );
}
