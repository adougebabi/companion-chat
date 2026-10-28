import type {ReactNode} from 'react';
export default function WorkspacePageHeader({eyebrow='FLUCTLIGHT / YOUR EVERYDAY',title,description,actions}:{eyebrow?:string;title:string;description?:ReactNode;actions?:ReactNode}){
 return <header className="replica-page-heading"><div><div className="replica-eyebrow">{eyebrow}</div><h1 tabIndex={-1}>{title}</h1>{description&&<p>{description}</p>}</div>{actions&&<div className="replica-page-actions">{actions}</div>}</header>;
}
