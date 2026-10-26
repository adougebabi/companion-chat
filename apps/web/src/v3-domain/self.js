import {SemanticFields} from '../v3/proto/components/semantic-fields.js';
import {GovernanceTabs} from '../v3/proto/components/governance-tabs.js';
import {selfClient} from '../v3/proto/services.js';
import * as store from '../v3/proto/store.js';
import {asyncPage,instant} from '../v3/proto/components/async-page.js';
import {mountCursorRecords} from '../v3/proto/components/cursor-records.js';
import {CursorPager,bindCursorPager} from '../v3/proto/components/cursor-pager.js';
import {Field,SearchForm} from '../v3/proto/components/forms.js';
import {JsonDetails} from '../v3/proto/components/records.js';
import {SelfClaimCard,canRestoreSelfRevision,selfCategoryLabels} from '../v3/proto/components/self-claims.js';
import {governanceCommand} from '../v3/proto/components/governance-command.js';
import {esc,badge} from '../v3/proto/ui.js';

const growthNavigation=(fid,current)=>`<nav class="reading-module-nav" aria-label="记忆成长">${[['memory','记忆',`#/c/${fid}/growth`],['reflection','反思',`#/c/${fid}/growth/reflection`],['self','成长认识',`#/c/${fid}/growth/self`],['snapshot','人格快照',`#/c/${fid}/growth/self?view=snapshot`]].map(([key,label,href])=>`<a href="${href}" ${current===key?'aria-current="page"':''}>${label}</a>`).join('')}</nav>`;

const tabs=fid=>GovernanceTabs(fid,'developing');
function command(fid,claim,operation,reload,target){
 if(store.get('characters',fid)?.status==='已退休')return;
 const label={review:'复核这条认识',forget:'遗忘这条认识',restore:'恢复这版认识内容'}[operation];
 governanceCommand({scope:[fid,claim.id,operation,target],title:label,note:operation==='review'?'记录你已阅读并认可这个版本；不改变原始证据、置信度或人格。':operation==='forget'?'将这条认识从当前消费中移除，保留原始修订和证据。':'追加新的认识修订。历史来源仍须有效，生活和聊天时间线不变。',revision:claim.revision,snapshot:claim,read:()=>selfClient.read(fid,claim.id),execute:body=>selfClient.command(fid,claim.id,operation,{...body,...(operation==='restore'?{targetRevision:target}:{})}),onSaved:reload});
}
function bindClaims(box,fid,claims,reload){
 for(const [attribute,operation]of [['selfReview','review'],['selfForget','forget']]){
  const selector=attribute==='selfReview'?'[data-self-review]':'[data-self-forget]';
  box.querySelectorAll(selector).forEach(button=>{button.disabled ||= store.get('characters',fid)?.status==='已退休';button.onclick=()=>command(fid,claims.find(c=>c.id===button.dataset[attribute]),operation,reload);});
 }
}
export function renderRealSelfGovernance(el,ctx){
 const cursors=[''];let at=0;const options={q:ctx.query.q||'',status:ctx.query.status||'all',category:ctx.query.category||''};
 return asyncPage(el,()=>selfClient.list(ctx.charId,{...options,cursor:cursors[at]}),(page,reload)=>{
  el.innerHTML=`<h1 class="reading-business-title">认识查阅与历史</h1>${growthNavigation(ctx.charId,'self')}${tabs(ctx.charId)}<div class="row" style="margin:12px 0"><a class="btn" href="#/c/${ctx.charId}/growth/self">自我认知</a><button class="btn" data-reload>重新读取</button></div><div class="card"><h3>发展中的自我</h3><p>认识来自真实证据与反思。复核是独立的所有者记录；遗忘和恢复追加修订，不改写人格或生活历史。</p>${SearchForm({value:options.q,placeholder:'搜索全部认识正文',attrs:{'data-search':true}})}<div class="row" style="margin:12px 0">${Field({label:'认识状态',name:'status',type:'select',value:options.status,options:[['all','全部（含遗忘）'],['active','有效'],['uncertain','不确定'],['forgotten','已遗忘'],['superseded','已替代']].map(([value,label])=>({value,label}))})}${Field({label:'类别',name:'category',type:'select',value:options.category,options:[{value:'',label:'全部类别'},...['preference','habit','sensitivity','emotion_pattern','self_perception','capability','interest'].map(value=>({value,label:selfCategoryLabels[value]||value}))]})}</div></div><div class="stack">${page.items.map(c=>SelfClaimCard(c)).join('')||'<div class="statebox"><h2>当前筛选没有认识记录</h2><p>继续真实交流与生活，反思可以形成新的认识；没有候选不等于运行失败。</p><a class="btn" href="#/c/'+ctx.charId+'/growth/reflection">查看反思</a></div>'}</div>${CursorPager({more:!!page.next_cursor,previous:at>0,label:'认识'})}`;
  const navigate=()=>ctx.navigate(`#/c/${ctx.charId}/governance?${new URLSearchParams({tab:'developing',q:el.querySelector('[data-search] [name=q]').value,status:el.querySelector('[name=status]').value,category:el.querySelector('[name=category]').value})}`);
  el.querySelector('[data-search]').onsubmit=event=>{event.preventDefault();navigate();};el.querySelector('[name=status]').onchange=navigate;el.querySelector('[name=category]').onchange=navigate;el.querySelector('[data-reload]').onclick=reload;
  bindCursorPager(el,{next:()=>{cursors[++at]=page.next_cursor;void reload();},previous:()=>{at--;void reload();}});bindClaims(el,ctx.charId,page.items,reload);
 },{backHref:`#/c/${encodeURIComponent(ctx.charId)}/growth`,backLabel:'返回记忆成长'});
}
export function renderRealSelfClaim(el,ctx){
 const fid=ctx.charId,id=ctx.params.claimId;
 return asyncPage(el,()=>selfClient.read(fid,id),(claim,reload,alive)=>{
  el.innerHTML=`<div class="stack reading-workspace"><h1 class="reading-business-title">认识详情与修订</h1>${growthNavigation(fid,'self')}${SelfClaimCard(claim,{full:true})}<div class="row"><button class="btn" data-reload>重新读取</button><a class="btn" href="#/c/${fid}/governance?tab=developing">全部认识</a></div><div class="card"><h3>逐版修订与恢复</h3><div data-revisions></div></div><div class="card"><h3>所有者复核记录</h3><div data-reviews></div></div></div>`;
  el.querySelector('[data-reload]').onclick=reload;bindClaims(el,fid,[claim],reload);
  const revisions=mountCursorRecords(el.querySelector('[data-revisions]'),{alive,label:'修订',read:cursor=>selfClient.history(fid,id,'revisions',cursor),render:r=>`<div class="lrow"><div class="grow"><b>v${esc(r.revision)} · ${esc(r.change_type)}</b><p class="small muted">${esc(instant(r.created_at))} · ${esc(r.reason_code)} · ${esc(r.source_window||'未记录窗口')}</p>${JsonDetails(r.before_value,'修改前快照')}${JsonDetails(r.after_value,'修改后快照')}${JsonDetails(r.evidence_refs,'该版证据')}</div>${badge(r.status)}${canRestoreSelfRevision(r)?`<button class="btn sm" data-restore="${esc(r.revision)}">恢复此版本内容</button>`:'<span class="small muted">此记录无可恢复的完整有效快照</span>'}</div>`,bind:box=>box.querySelectorAll('[data-restore]').forEach(button=>{button.disabled=store.get('characters',fid)?.status==='已退休';button.onclick=()=>command(fid,claim,'restore',reload,Number(button.dataset.restore));})});
  const reviews=mountCursorRecords(el.querySelector('[data-reviews]'),{alive,label:'复核',read:cursor=>selfClient.history(fid,id,'reviews',cursor),empty:'尚无人工复核；正常反思不以人工审核为前提。',render:r=>`<div class="lrow"><span class="grow">${esc(r.reason)}<div class="small muted">针对认识 v${esc(r.claim_revision)} · ${esc(instant(r.created_at))}</div></span>${badge('所有者已复核','plain')}</div>`});
  return()=>{revisions.dispose();reviews.dispose();};
 },{backHref:`#/c/${encodeURIComponent(ctx.charId)}/growth`,backLabel:'返回记忆成长'});
}
export function renderRealSelf(el,ctx){
 const fid=ctx.charId;
 return asyncPage(el,()=>selfClient.summary(fid),(detail,reload,alive)=>{
  const proposals=Array.isArray(detail.foundation_revisions)?detail.foundation_revisions.filter(p=>p.status==='proposed'):null;
  const snapshot=ctx.query.view==='snapshot';
  el.innerHTML=`<div class="stack reading-workspace"><h1 class="reading-business-title">${snapshot?'人格快照':'成长认识'}</h1>${growthNavigation(fid,snapshot?'snapshot':'self')}<section class="stack" ${snapshot?'hidden':''}><div class="row"><h2 class="grow">她逐渐形成的认识</h2><a class="btn" href="#/c/${fid}/governance?tab=developing">查阅全部与历史</a></div><p class="small muted">认识来自真实交流、经历与反思，可以查阅每条认识的证据与修订。</p><div class="stack" data-claims></div></section><details class="card reading-secondary" ${snapshot?'open':''}><summary>稳定人格、身份提案与工作人格快照${proposals?.length?' · '+proposals.length+' 项待确认':''}</summary><div class="grid g2 reading-maintenance"><div class="card tint-primary"><h3>她眼中的自己</h3><p class="small muted">稳定人格、发展中的认识与临时状态分别读取。</p>${SemanticFields(detail.personality,{empty:'稳定人格尚未记录可读的性格内容。'})}<details class="reading-secondary"><summary>人格定义与工作人格原始快照</summary>${JsonDetails(detail.personality,'稳定人格完整定义')}${JsonDetails(detail.persona_compilation,'实际工作人格编译与版本')}</details><h3>最近的反思</h3><div data-reflections></div></div><div class="card"><h3>人格与身份提案</h3><p class="small muted">以下是实际待确认的身份核心修订。反思的自动成长与提案分别处理。</p>${proposals===null?'<p class="banner warn">当前服务未提供身份修订读面。</p>':proposals.map(p=>`<div class="lrow"><span class="grow">身份核心修订 v${esc(p.revision)}<div class="small muted">${esc(p.source)} · ${esc(p.reason||'')}</div></span>${badge(p.status)}<button class="btn sm" data-reject="${esc(p.id)}">暂不采纳</button><button class="btn sm accent" data-accept="${esc(p.id)}">采纳</button></div>`).join('')||'<p class="small muted">暂无待确认提案。</p>'}<a class="btn" href="#/c/${fid}/governance?tab=foundation">身份核心与完整修订</a></div></div></details></div>`;
  const claims=mountCursorRecords(el.querySelector('[data-claims]'),{alive,label:'认识',read:cursor=>selfClient.list(fid,{status:'active',cursor}),empty:'还没有当前有效的认识记录。继续交流和生活后可在真实反思中形成。',render:c=>SelfClaimCard(c),bind:(box,page)=>bindClaims(box,fid,page.items,reload)});
  const reflections=mountCursorRecords(el.querySelector('[data-reflections]'),{alive,label:'反思',read:cursor=>selfClient.reflections(fid,cursor),empty:'尚无反思记录。',render:r=>`<a class="lrow" href="#/c/${fid}/growth/reflection/${encodeURIComponent(r.id)}"><span class="grow">${esc(r.summary||r.title)}</span>${badge(r.status)}</a>`});
  for(const attribute of ['accept','reject'])el.querySelectorAll(`[data-${attribute}]`).forEach(button=>{button.disabled=store.get('characters',fid)?.status==='已退休';button.onclick=()=>{
   const proposal=proposals.find(p=>p.id===button.dataset[attribute]);
   governanceCommand({scope:[fid,'foundation',proposal.id,attribute],title:attribute==='accept'?'采纳身份提案':'暂不采纳提案',note:'通过实际身份核心受控命令处理；已生效的生活状态和历史保留。',revision:detail.current_revision,snapshot:proposal,read:async()=>{const current=await selfClient.summary(fid);return {...current,revision:current.current_revision};},execute:body=>selfClient.decideFoundation(fid,proposal.id,attribute==='accept',body.expectedRevision,body.reason),onSaved:reload});
  };});
  return()=>{claims.dispose();reflections.dispose();};
 },{backHref:`#/c/${encodeURIComponent(ctx.charId)}/growth`,backLabel:'返回记忆成长'});
}
