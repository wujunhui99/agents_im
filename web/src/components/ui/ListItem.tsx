import type { ReactNode } from 'react';

type ListItemProps = {
  leading?: ReactNode;
  headline: ReactNode;
  supportingText?: ReactNode;
  trailing?: ReactNode;
  className?: string;
  onClick?: () => void;
  ariaLabel?: string;
  ariaDisabled?: boolean;
};

export function ListItem({ leading, headline, supportingText, trailing, className = '', onClick, ariaLabel, ariaDisabled }: ListItemProps) {
  const classes = ['md-list-item', className].filter(Boolean).join(' ');
  const ariaDisabledValue = ariaDisabled ? true : undefined;
  const content = (
    <>
      {leading ? <span className="md-list-item-leading">{leading}</span> : null}
      <span className="md-list-item-main row-main">
        <strong>{headline}</strong>
        {supportingText ? <p>{supportingText}</p> : null}
      </span>
      {trailing ? <span className="md-list-item-trailing row-trailing">{trailing}</span> : null}
    </>
  );

  if (onClick) {
    return (
      <button type="button" className={classes} aria-label={ariaLabel} aria-disabled={ariaDisabledValue} onClick={onClick}>
        {content}
      </button>
    );
  }

  return (
    <article className={classes} aria-label={ariaLabel} aria-disabled={ariaDisabledValue}>
      {content}
    </article>
  );
}
