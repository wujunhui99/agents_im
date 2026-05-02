import type { HTMLAttributes, ReactNode } from 'react';

type CardVariant = 'surface' | 'filled' | 'elevated';

type CardProps = HTMLAttributes<HTMLElement> & {
  children: ReactNode;
  ariaLabel?: string;
  variant?: CardVariant;
};

export function Card({ children, ariaLabel, className = '', variant = 'surface', ...props }: CardProps) {
  const classes = ['md-card', `md-card-${variant}`, className].filter(Boolean).join(' ');

  return (
    <section className={classes} aria-label={ariaLabel} {...props}>
      {children}
    </section>
  );
}
