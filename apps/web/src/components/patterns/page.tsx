import type { ReactNode } from "react";
import WorkspacePageHeader from "@/react-app/shared/WorkspacePageHeader";
import { cn } from "@/lib/utils";

/**
 * PageContainer — page heading + content region. The workspace shell
 * (`.workspace-page`) owns scrolling and gutters; this component owns only the
 * heading/content rhythm and the content max width. Pages must not restate
 * those rules in their own CSS.
 */
export function PageContainer({
  eyebrow,
  title,
  description,
  actions,
  width = "default",
  className,
  children,
}: {
  eyebrow?: string;
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
  /** default: reading width (960px); wide: 1360px; full: shell width. */
  width?: "default" | "wide" | "full";
  className?: string;
  children?: ReactNode;
}) {
  return (
    <section data-ui-page="" data-width={width} className={cn("ui-page", className)}>
      <WorkspacePageHeader eyebrow={eyebrow} title={title} description={description} actions={actions} />
      <div data-ui-page-content="" className="ui-page__content">
        {children}
      </div>
    </section>
  );
}

/** A titled block inside a page. Use instead of ad-hoc cards. */
export function PageSection({
  title,
  description,
  actions,
  className,
  children,
}: {
  title?: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  className?: string;
  children?: ReactNode;
}) {
  return (
    <section data-ui-page-section="" className={cn("ui-page-section", className)}>
      {(title || actions) && (
        <header className="ui-page-section__head">
          <div className="ui-page-section__title">
            {title && <h2>{title}</h2>}
            {description && <p>{description}</p>}
          </div>
          {actions && <div className="ui-actions">{actions}</div>}
        </header>
      )}
      {children}
    </section>
  );
}
