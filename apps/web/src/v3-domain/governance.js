import {SemanticFields} from '../v3/proto/components/semantic-fields.js';
import {stateLabel} from '../v3/domain-presentation.mjs';
import {governanceClient} from '../v3/proto/services.js';
import * as store from '../v3/proto/store.js';
import {asyncPage} from '../v3/proto/components/async-page.js';
import {GovernanceTabs} from '../v3/proto/components/governance-tabs.js';
import {GovernanceRecord,GovernanceHistory} from '../v3/proto/components/governance-records.js';
import {foundationProposal} from '../v3/proto/components/foundation-proposal.js';
import {governanceCommand} from '../v3/proto/components/governance-command.js';
import {mountCursorRecords} from '../v3/proto/components/cursor-records.js';
import {CursorPager,bindCursorPager} from '../v3/proto/components/cursor-pager.js';
import {Field,SearchForm} from '../v3/proto/components/forms.js';
import {JsonDetails} from '../v3/proto/components/records.js';
import {esc,badge} from '../v3/proto/ui.js';
const retired=fid=>store.get('characters',fid)?.status==='已退休';
const scopeTab=kind=>kind==='foundation'?'foundation':'capabilities';
export function renderRealGovernance(el,ctx){
 const fid=ctx.charId,kind=ctx.query.tab==='foundation'?'foundation':'capability',tab=scopeTab(kind),cursors=[''];let at=0;
 const options={q:ctx.query.q||'',status:ctx.query.status||'all'};
 return asyncPage(el,async()=>({current:await governanceClient.current(fid),page:await governanceClient.list(fid,kind,{...options,cursor:cursors[at]})}),({current,page},reload)=>{
  const statuses=kind==='foundation'?['all','proposed','accepted','rejected']:['all','proposed','reviewing','accepted','fulfilled','rejected','cancelled'];
  el.innerHTML=`${GovernanceTabs(fid,tab)}<div class="row" style="margin:12px 0"><a data-domain-ui="button" class="btn" href="#/c/${fid}/profile">角色资料</a><a data-domain-ui="button" class="btn" href="#/c/${fid}/growth/self">自我认知</a><button data-domain-ui="button" class="btn" data-reload>重新读取</button></div><div class="card stack"><h3>${kind==='foundation'?'身份核心':'能力申请与审核'}</h3><p>${kind==='foundation'?'对照提案创建时的基线审核。接受与应用是同一次受控操作；恢复身份对象会追加版本。':'申请来自角色真实运行。接受需求保留审核结果；标记已交付需要匹配真实注册实现，实际执行另受授权与策略约束。'}</p>${kind==='foundation'?`<p>当前生效身份 v${esc(current.revision)}</p>${SemanticFields(current.identity,{empty:'当前身份尚未提供可阅读的字段。'})}${JsonDetails({identity:current.identity,personality:current.personality,behavioral_policy:current.behavioral_policy,life_profile:current.life_profile},'当前完整身份字段')}<button data-domain-ui="button" class="btn" data-propose ${retired(fid)?'disabled':''}>创建修订提议</button>`:'<a data-domain-ui="button" class="btn" href="#/settings/capabilities">能力目录（管理中心）</a>'}${SearchForm({value:options.q,placeholder:kind==='foundation'?'搜索修订字段':'搜索申请标题、说明或能力名称',attrs:{'data-search':true}})}${Field({label:'状态',name:'status',type:'select',value:options.status,options:statuses.map(value=>({value,label:value==='all'?'全部状态':stateLabel(value)}))})}</div><div class="stack section-gap">${page.items.map(row=>GovernanceRecord(row,kind)).join('')||`<div class="statebox"><h2>当前筛选没有记录</h2><p>${kind==='foundation'?'可以创建身份核心修订提议，或调整筛选查看已生效历史。':'当真实运行缺少能力时，会形成有来源的申请。可前往能力目录查看当前实现与授权。'}</p></div>`}</div>${CursorPager({more:!!page.next_cursor,previous:at>0,label:'治理记录'})}`;
  el.querySelector('[data-reload]').onclick=reload;
  el.querySelector('[data-propose]')?.addEventListener('click',()=>{if(!retired(fid))foundationProposal(fid,current,reload);});
  const navigate=()=>ctx.navigate(`#/c/${fid}/governance?${new URLSearchParams({tab,status:el.querySelector('[name=status]').value,q:el.querySelector('[name=q]').value})}`);
  el.querySelector('[name=status]').onchange=navigate;el.querySelector('[data-search]').onsubmit=e=>{e.preventDefault();navigate();};
  bindCursorPager(el,{next:()=>{cursors[++at]=page.next_cursor;void reload();},previous:()=>{at--;void reload();}});
 },{backHref:`#/c/${encodeURIComponent(ctx.charId)}/growth`,backLabel:'返回记忆成长'});
}
export function renderRealGovernanceDetail(el,ctx){
 const fid=ctx.charId,id=ctx.params.governanceId,kind=ctx.params.governanceKind;
 if(kind!=='foundation'&&kind!=='capability'){el.textContent='治理对象类型无效';return;}
 return asyncPage(el,async()=>({current:await governanceClient.current(fid),record:await governanceClient[kind](fid,id)}),({current,record},reload,alive)=>{
  const stale=kind==='foundation'&&record.status==='proposed'&&record.base_revision!==current.revision;
  const actions=kind==='foundation'?record.status==='proposed'?[['accept','接受并应用'],['reject','拒绝提议']]:record.status==='accepted'&&record.revision<current.revision?[['restore','恢复此版身份内容']]:[]:({proposed:[['reviewing','开始评估'],['accepted','接受需求'],['rejected','拒绝'],['cancelled','取消申请']],reviewing:[['accepted','接受需求'],['rejected','拒绝'],['cancelled','取消申请']],accepted:[['fulfilled','记录已交付实现'],['cancelled','取消需求']]}[record.status]||[]);
  el.innerHTML=`${GovernanceTabs(fid,scopeTab(kind))}<div class="row" style="margin:12px 0"><a data-domain-ui="button" class="btn" href="#/c/${fid}/governance?tab=${scopeTab(kind)}">返回全部记录</a><button data-domain-ui="button" class="btn" data-reload>重新读取</button></div>${GovernanceRecord(record,kind,{detail:true})}${stale?'<p class="banner warn">这条提议的基线已经过期。请保留变更，对照最新身份重新创建提议。它仍可以被拒绝。</p>':''}<div class="row">${actions.map(([action,label])=>`<button data-domain-ui="button" class="btn" data-command="${esc(action)}" ${retired(fid)||(stale&&action==='accept')?'disabled':''}>${esc(label)}</button>`).join('')}</div>${kind==='capability'&&record.status==='accepted'?Field({label:'实际注册的能力版本',name:'capabilityVersion',value:record.registered_capability?.version||'',hint:'必须与已注册实现精确一致；审核不会安装插件。',attrs:{maxlength:128}}):''}<div class="card" style="margin-top:12px"><h3>完整治理记录</h3><div data-history></div></div>`;
  el.querySelector('[data-reload]').onclick=reload;
  const history=mountCursorRecords(el.querySelector('[data-history]'),{alive,label:'治理历史',read:cursor=>governanceClient.history(fid,kind,id,cursor),render:GovernanceHistory,empty:'尚无治理操作；旧记录不会补造审核历史。'});
  el.querySelectorAll('[data-command]').forEach(button=>button.onclick=()=>{
   if(retired(fid))return;const action=button.dataset.command,version=el.querySelector('[name=capabilityVersion]')?.value?.trim();
   governanceCommand({scope:[fid,kind,id,action,version],title:button.textContent,note:kind==='foundation'?'生效前会校验当前身份与提案基线，编译 Working Persona，并保留有效生活状态。':'保存真实审核与版本记录，不修改原申请证据，不自动增加 Tool 授权。',revision:kind==='foundation'?current.revision:record.revision,snapshot:record,read:()=>kind==='foundation'?governanceClient.current(fid):governanceClient.capability(fid,id),execute:body=>kind==='foundation'?action==='restore'?governanceClient.restore(fid,record.revision,body):governanceClient.decide(fid,id,action,body):governanceClient.review(fid,id,{expectedRevision:body.expectedRevision,note:body.reason,status:action,...(action==='fulfilled'?{capabilityVersion:version}:{})}),onSaved:async()=>{await governanceClient.refresh();await reload();}});
  });return()=>history.dispose();
 },{backHref:`#/c/${encodeURIComponent(ctx.charId)}/growth`,backLabel:'返回记忆成长'});
}
