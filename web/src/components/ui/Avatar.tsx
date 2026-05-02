import type { ReactNode } from 'react';

type AvatarColor = 'green' | 'blue' | 'purple' | 'orange' | 'gray';

type AvatarProps = {
  label: ReactNode;
  color: AvatarColor;
  size?: 'default' | 'large';
};

export function Avatar({ label, color, size = 'default' }: AvatarProps) {
  const sizeClass = size === 'large' ? ' avatar-large' : '';

  return (
    <div className={`md-avatar avatar avatar-${color}${sizeClass}`} aria-hidden="true">
      {label}
    </div>
  );
}
