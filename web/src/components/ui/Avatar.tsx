import { useEffect, useState, type ReactNode } from 'react';

type AvatarColor = 'green' | 'blue' | 'purple' | 'orange' | 'gray';

type AvatarProps = {
  label: ReactNode;
  color: AvatarColor;
  size?: 'default' | 'large';
  src?: string;
  alt?: string;
  onImageError?: () => void;
};

export function Avatar({ label, color, size = 'default', src, alt = '头像', onImageError }: AvatarProps) {
  const [imageFailed, setImageFailed] = useState(false);
  const sizeClass = size === 'large' ? ' avatar-large' : '';
  const showImage = Boolean(src) && !imageFailed;

  useEffect(() => {
    setImageFailed(false);
  }, [src]);

  return (
    <div className={`md-avatar avatar avatar-${color}${sizeClass}`} aria-hidden={showImage ? undefined : true}>
      {showImage ? (
        <img
          className="avatar-image"
          src={src}
          alt={alt}
          onError={() => {
            setImageFailed(true);
            onImageError?.();
          }}
        />
      ) : (
        label
      )}
    </div>
  );
}
