import type { ButtonHTMLAttributes, ReactNode } from 'react';

type ButtonVariant = 'filled' | 'tonal' | 'outlined' | 'text' | 'icon';
type ButtonSize = 'small' | 'medium';

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  variant?: ButtonVariant;
  size?: ButtonSize;
};

export function Button({ children, className = '', variant = 'filled', size = 'medium', type = 'button', ...props }: ButtonProps) {
  const classes = ['md-button', `md-button-${variant}`, `md-button-${size}`, className].filter(Boolean).join(' ');

  return (
    <button type={type} className={classes} {...props}>
      {children}
    </button>
  );
}
