import type { ReactNode } from "react";
import { InfoIcon } from "lucide-react";
export default function WorkspaceInfo({ children }: { children: ReactNode }) {
  return (
    <div className="replica-info" role="note">
      <InfoIcon size={16} aria-hidden="true" />
      <div>{children}</div>
    </div>
  );
}
