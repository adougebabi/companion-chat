const attentionKinds=['capability','persona_proposal','studio_apply'];
export function pendingSummary(groups){
 const rows=Array.isArray(groups)?groups:[];
 const count=kinds=>rows.filter(g=>kinds.includes(g.kind)&&g.status==='ok').reduce((sum,g)=>sum+g.count,0);
 const known=kinds=>kinds.every(kind=>rows.some(g=>g.kind===kind&&g.status==='ok'));
 return {attention:count(attentionKinds),attentionKnown:known(attentionKinds),progress:count(['owner_request']),progressKnown:known(['owner_request']),service:count(['failed_task']),serviceKnown:known(['failed_task'])};
}
export const pendingCategory=kind=>attentionKinds.includes(kind)?'attention':kind==='owner_request'?'progress':'service';
