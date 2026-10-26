import {esc} from '../ui.js';

// Render actual server references, not optimistic application state. A proposed
// reference always links to the existing governed accept/compile consumer.
export function FoundationSourceLinks(resource){
 const refs=(resource?.referenced_by||[]).filter(ref=>ref.target_kind==='foundation');
 return refs.length?`<div data-domain-ui="card" class="card"><h3>身份核心应用与提案</h3>${refs.map(ref=>`<a data-domain-ui="item" class="lrow" href="#/c/${encodeURIComponent(ref.fluctlight_id)}/governance/foundation/${encodeURIComponent(ref.runtime_id)}"><span class="grow">发布版本 ${esc(ref.running_version)} → 身份核心 v${esc(ref.instance_revision)}<p class="small muted">${esc(ref.fluctlight_id)}</p></span>${ref.currently_effective?'当前生效':ref.status==='proposed'?'提案待审核 · 尚未生效':ref.status==='accepted'?'已接受历史':ref.status==='rejected'?'已拒绝':esc(ref.status)}</a>`).join('')}</div>`:'';
}
