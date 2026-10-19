import {esc,badge} from '../ui.js';
import {JsonDetails} from './records.js';

export const selfClaimHref=(fid,id)=>`#/c/${encodeURIComponent(fid)}/growth/self/claim/${encodeURIComponent(id)}`;
const status={active:'已记录的认识',uncertain:'仍不确定',forgotten:'已遗忘',superseded:'已有替代'};
export function SelfClaimCard(claim,{full=false}={}){
 return `<div class="card"><div class="row"><b class="grow"><a href="${selfClaimHref(claim.fluctlight_id,claim.id)}">${esc(claim.claim)}</a></b>${badge(status[claim.status]||claim.status,'plain')}</div><p class="small muted">${esc(claim.category)} · v${esc(claim.revision)} · 置信度 ${esc(claim.confidence)}</p>${claim.source_valid===false?'<p class="banner warn">所依赖的事实来源已变化，此认识不作为当前有效上下文。</p>':''}${claim.expired?'<p class="small muted">已超过有效期限。</p>':''}${claim.owner_review?`<p class="small muted">所有者已复核当前版本：${esc(claim.owner_review.reason)}。复核不会提高证据或置信度。</p>`:''}${full?`${JsonDetails(claim.value,'认识的具体值')}${JsonDetails(claim.evidence_refs,'证据引用')}${JsonDetails(claim.provenance,'原始来源与生产者')}`:''}<div class="row"><a class="btn sm" href="${selfClaimHref(claim.fluctlight_id,claim.id)}">详情与修订</a><button class="btn sm" data-self-review="${esc(claim.id)}">复核认可</button><button class="btn sm danger" data-self-forget="${esc(claim.id)}" ${claim.status==='forgotten'?'disabled':''}>遗忘此认识</button></div></div>`;
}
export function canRestoreSelfRevision(revision){
 let snapshot=revision.after_value;if(!snapshot||typeof snapshot!=='object'||!snapshot.category)snapshot=revision.candidate;
 return revision.status==='accepted'&&snapshot&&typeof snapshot==='object'&&['category','claim','value','confidence','evidence_refs','provenance','status'].every(key=>Object.hasOwn(snapshot,key));
}
