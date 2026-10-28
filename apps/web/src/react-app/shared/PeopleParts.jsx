import { Button } from "@/components/ui/button";
import { Surface } from "../controls";
import LoadingState from "./LoadingState";
import { InlineFeedback } from "./Presentation";
export function PeopleReader({ source, label }) {
  return (
    <>
      {source.busy && <LoadingState label={"正在读取" + label + "…"} />}{" "}
      {source.error && (
        <InlineFeedback role="alert">
          {source.error}
          <Button variant="outline" onClick={source.retry}>
            重新读取{label}
          </Button>
        </InlineFeedback>
      )}
    </>
  );
}
export function PeopleSection({
  title,
  description,
  action,
  children,
  className = "",
}) {
  return (
    <Surface className={className}>
      <div className="profile-section-heading">
        <div>
          <h2>{title}</h2>
          {description && <p>{description}</p>}
        </div>
        {action}
      </div>
      {children}
    </Surface>
  );
}
export function PeopleFact({ label, children }) {
  return (
    <div className="profile-key-value">
      <span>{label}</span>
      <div>
        {children ?? <span className="profile-empty-value">还没有填写</span>}
      </div>
    </div>
  );
}
