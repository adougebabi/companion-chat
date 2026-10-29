import type { ReactNode } from "react";
import { InfoIcon } from "lucide-react";
import {Feedback,type FeedbackKind} from './Presentation';
export default function WorkspaceInfo({ children,kind='information' }: { children: ReactNode;kind?:FeedbackKind }) {
  if(kind!=='information')return <Feedback kind={kind}>{children}</Feedback>;
  return (
    <div className="replica-info" role="note" data-feedback-kind="information">
      <InfoIcon size={16} aria-hidden="true" />
      <div>{children}</div>
    </div>
  );
}
