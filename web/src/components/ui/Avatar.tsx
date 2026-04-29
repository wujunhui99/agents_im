type AvatarColor = 'green' | 'blue' | 'purple' | 'orange' | 'gray';

type AvatarProps = {
  label: string;
  color: AvatarColor;
  size?: 'default' | 'large';
};

export function Avatar({ label, color, size = 'default' }: AvatarProps) {
  const sizeClass = size === 'large' ? ' avatar-large' : '';

  return (
    <div className={`avatar avatar-${color}${sizeClass}`} aria-hidden="true">
      {label}
    </div>
  );
}
