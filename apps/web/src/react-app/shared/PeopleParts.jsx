import { Button } from "@/components/ui/button";
import { Surface } from "../controls";
import LoadingState from "./LoadingState";
import { ReaderFeedback } from "./Presentation";
export function PeopleReader({source,label}){
 return <>{source.busy&&!source.ready&&<LoadingState label={'正在读取'+label+'…'}/>}<ReaderFeedback source={source} label={label}/></>;
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
