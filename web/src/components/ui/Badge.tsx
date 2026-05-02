import type { HTMLAttributes, ReactNode } from 'react';

type BadgeTone = 'error' | 'primary' | 'neutral';

type BadgeProps = HTMLAttributes<HTMLSpanElement> & {
  children: ReactNode;
  tone?: BadgeTone;
};

export function Badge({ children, className = '', tone = 'neutral', ...props }: BadgeProps) {
  const classes = ['md-badge', `md-badge-${tone}`, className].filter(Boolean).join(' ');

  return (
    <span className={classes} {...props}>
      {children}
    </span>
  );
}
