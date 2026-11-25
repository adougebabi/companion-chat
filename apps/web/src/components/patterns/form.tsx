import * as React from "react";
import { cn } from "@/lib/utils";

/**
 * FormSection — titled group of fields. One column on phones; `columns={2}`
 * becomes two columns from 768px. Long labels and values wrap.
 */
export function FormSection({
  title,
  description,
  columns = 1,
  className,
  children,
}: {
  title?: React.ReactNode;
  description?: React.ReactNode;
  columns?: 1 | 2;
  className?: string;
  children?: React.ReactNode;
}) {
  // A heading labelled group rather than <legend>: a rendered legend straddles
  // the fieldset's top edge, which is where the section separator is drawn.
  const titleId = React.useId();
  return (
    <fieldset data-ui-form-section="" data-columns={columns} aria-labelledby={title ? titleId : undefined} className={cn("ui-form-section", className)}>
      {title && <div id={titleId} className="ui-form-section__title">{title}</div>}
      {description && <p className="ui-form-section__description">{description}</p>}
      <div className="ui-form-section__fields">{children}</div>
    </fieldset>
  );
}

type ControlProps = { id: string; "aria-describedby"?: string; "aria-invalid"?: boolean; required?: boolean };

/**
 * FormField — label, control, hint and error wired for assistive technology.
 * `children` receives the id/aria props to spread on the control.
 */
export function FormField({
  label,
  hint,
  error,
  required = false,
  wide = false,
  inline = false,
  children,
}: {
  label: React.ReactNode;
  hint?: React.ReactNode;
  error?: React.ReactNode;
  required?: boolean;
  /** Span both columns of a two-column FormSection. */
  wide?: boolean;
  /** Checkbox/switch layout: control before the label on one row. */
  inline?: boolean;
  children: (control: ControlProps) => React.ReactNode;
}) {
  const id = React.useId();
  const hintId = hint ? `${id}-hint` : undefined;
  const errorId = error ? `${id}-error` : undefined;
  const describedBy = [errorId, hintId].filter(Boolean).join(" ") || undefined;
  const control = children({ id, "aria-describedby": describedBy, "aria-invalid": error ? true : undefined, required: required || undefined });
  return (
    <div data-ui-form-field="" data-wide={wide || undefined} data-inline={inline || undefined} data-invalid={error ? true : undefined} className="ui-form-field">
      {inline ? (
        <label htmlFor={id} className="ui-form-field__inline">
          {control}
          <span className="ui-form-field__label">{label}</span>
        </label>
      ) : (
        <>
          <label htmlFor={id} className="ui-form-field__label">
            {label}
            {required && <span aria-hidden="true" className="ui-form-field__required"> *</span>}
          </label>
          {control}
        </>
      )}
      {hint && <p id={hintId} className="ui-form-field__hint">{hint}</p>}
      {error && <p id={errorId} className="ui-form-field__error" data-feedback-kind="error">{error}</p>}
    </div>
  );
}

/**
 * FormActions — action row for in-page forms. `sticky` pins it to the bottom
 * of the page scroller. Inside sheets use EditSheet's footer instead.
 */
export function FormActions({
  children,
  status,
  sticky = false,
  className,
}: {
  children: React.ReactNode;
  status?: React.ReactNode;
  sticky?: boolean;
  className?: string;
}) {
  return (
    <div data-ui-form-actions="" data-sticky={sticky || undefined} className={cn("ui-form-actions", className)}>
      {status && <div className="ui-form-actions__status" role="status">{status}</div>}
      <div className="ui-actions">{children}</div>
    </div>
  );
}
