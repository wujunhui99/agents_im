import { useId, type InputHTMLAttributes, type ReactNode } from 'react';

type TextFieldProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'size'> & {
  label: string;
  leadingIcon?: ReactNode;
  hideLabel?: boolean;
  fieldClassName?: string;
};

export function TextField({ label, leadingIcon, hideLabel = false, fieldClassName = '', className = '', id, ...props }: TextFieldProps) {
  const generatedId = useId();
  const inputId = id ?? generatedId;
  const labelClassName = hideLabel ? 'sr-only' : 'md-text-field-label';
  const classes = ['md-text-field', fieldClassName].filter(Boolean).join(' ');
  const controlClasses = ['md-text-field-control', className].filter(Boolean).join(' ');

  return (
    <label className={classes} htmlFor={inputId}>
      <span className={labelClassName}>{label}</span>
      <span className={controlClasses}>
        {leadingIcon ? <span className="md-text-field-leading">{leadingIcon}</span> : null}
        <input id={inputId} aria-label={hideLabel ? label : undefined} {...props} />
      </span>
    </label>
  );
}
