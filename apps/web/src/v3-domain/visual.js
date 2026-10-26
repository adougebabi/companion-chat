import {MediaWorkspaceHeader} from './media.js';
import {Avatar,bindAvatars} from '../v3/proto/components/avatar.js';
import {visualReview} from '../v3/proto/components/visual-review.js';
import {visualClient,mediaClient} from '../v3/proto/services.js';
import {get} from '../v3/proto/store.js';
import {asyncPage} from '../v3/proto/components/async-page.js';
import {mountCursorRecords} from '../v3/proto/components/cursor-records.js';
import {mountVersionedFields} from '../v3/proto/components/versioned-fields.js';
import {visualBriefFields,VisualIdentityState,VisualTimeline,bindVisualAssetErrors,VisualBriefSource} from '../v3/proto/components/visual-identity-records.js';
import {governanceCommand} from '../v3/proto/components/governance-command.js';
import {JsonDetails} from '../v3/proto/components/records.js';
import {esc} from '../v3/proto/ui.js';
import {VisualBriefDiff} from '../v3/proto/components/visual-identity-records.js';
export function renderRealVisualEditor(el,ctx){
 const fid=ctx.charId;
 return asyncPage(el,()=>visualClient.read(fid),(snapshot,reload,alive)=>{
  el.innerHTML=`${MediaWorkspaceHeader(ctx,{title:'视觉定义',active:'identity'})}<div class="row [margin-bottom:12px]"><a data-domain-ui="button" class="btn" href="#/c/${fid}/media/identity">视觉身份与生成结果</a><button data-domain-ui="button" class="btn" data-reload>重新读取</button></div><div data-domain-ui="card" class="card"><h2>视觉定义草稿</h2><p>本页编辑参考约束，不改变此刻身体与穿搭。保存会留下真实版本记录，并在新的视觉生成会话中使用。</p><p class="small muted" data-brief-status>${snapshot.brief?'已保存参考定义 v'+esc(snapshot.revision)+'；规范参考图需另行生成与评审。':'尚未保存参考定义。下面是你的未提交输入。'}</p><div data-brief-source>${VisualBriefSource(snapshot.brief)}</div><div data-editor></div></div><div data-domain-ui="card" class="card [margin-top:12px]"><h3>定义修订</h3><div data-history></div></div>`;
  el.querySelector('[data-reload]').onclick=reload;
  const fields=Object.fromEntries(visualBriefFields.map(([name])=>[name,snapshot.brief?.[name]??'']));let history;
  const editor=mountVersionedFields(el.querySelector('[data-editor]'),{scope:[fid,'visual-brief'],snapshot,values:fields,fields:visualBriefFields,read:()=>visualClient.read(fid),save:body=>visualClient.save(fid,body),readOnly:get('characters',fid)?.status==='已退休',onSaved:saved=>{if(alive()){el.querySelector('[data-brief-source]').innerHTML=VisualBriefSource(saved.brief);el.querySelector('[data-brief-status]').textContent=`已保存参考定义 v${saved.revision}；规范参考图需另行生成与评审。`;}return history.reload({reset:true});}});
  history=mountCursorRecords(el.querySelector('[data-history]'),{alive,label:'定义修订',read:cursor=>visualClient.history(fid,cursor),render:r=>`<div data-domain-ui="card" class="card"><h3>参考定义 v${esc(r.revision)}</h3>${VisualBriefDiff(r.before_value,r.after_value)}${JsonDetails(r,'完整修订与审核记录')}</div>`,empty:'尚无已保存定义。填写后保存即可建立首个版本。'});
  return()=>{editor();history.dispose();};
 });
}
export function renderRealVisualIdentity(el,ctx){
 const fid=ctx.charId,owner=get('session')?.actorId;
 let active=true,identity=null,avatar=null,timer=null,reads=0,epoch=0,sessionId='',busy=false,refreshAgain=false;
 const errors={identity:'',avatar:'',observer:''},live=()=>active&&el.isConnected&&get('session')?.actorId===owner;
 el.innerHTML=`<div class="stack">${MediaWorkspaceHeader(ctx,{title:'视觉身份与头像',active:'identity'})}<div class="row"><a data-domain-ui="button" class="btn" href="#/c/${fid}/media/identity/edit">编辑视觉定义</a><a data-domain-ui="button" class="btn ghost" href="#/c/${fid}/media/avatar">设置显示头像</a><button data-domain-ui="button" class="btn ghost" data-reload>重新读取</button><button data-domain-ui="button" class="btn" data-generate ${get('characters',fid)?.status==='已退休'?'disabled':''}>生成视觉身份</button></div><div class="grid visual-identity-layout"><div class="card" data-identity role="status" data-domain-ui="skeleton">正在读取规范参考与候选…</div><aside data-domain-ui="card" class="card" data-avatar role="status">正在读取显示头像…</aside></div><p role="status" data-observer-status></p><p data-domain-ui="alert" role="alert" data-generation-error></p><details data-domain-ui="disclosure" class="card"><summary>完整生成与评审记录</summary><button data-domain-ui="button" class="btn ghost sm" data-timeline-refresh>读取最新记录</button><div data-timeline></div></details></div>`;
 function patch(host,markup){
  if(host.dataset.markup===markup)return;
  const open=[...host.querySelectorAll('details')].map((n,i)=>n.open?i:-1),focus=(document.activeElement?.closest?.('[data-domain-react-control]')?.domainSource||document.activeElement)?.matches('[data-manual-review]');
  host.innerHTML=markup;host.dataset.markup=markup;
  host.querySelectorAll('details').forEach((n,i)=>{n.open=open.includes(i);});
  bindVisualAssetErrors(host);if(focus)host.querySelector('[data-manual-review]')?.focus({preventScroll:true});
 }
 function paint(){
  if(!live())return;
  if(identity){patch(el.querySelector('[data-identity]'),VisualIdentityState(identity));const review=el.querySelector('[data-manual-review]');if(review)review.onclick=()=>{if(get('characters',fid)?.status!=='已退休')visualReview(fid,identity,()=>refresh());};}
  else el.querySelector('[data-identity]').innerHTML=errors.identity?'<p>规范参考读取失败，可重新读取。</p>':'正在读取规范参考与候选…';
  if(avatar){patch(el.querySelector('[data-avatar]'),`<h3>当前显示头像</h3>${Avatar({src:avatar.avatar?.url?mediaClient.url(avatar.avatar.asset_id):'',name:get('characters',fid)?.name||'角色',size:'lg'})}<p class="small muted">${avatar.mode==='asset'?'使用选定素材':avatar.mode==='canonical'?'跟随规范参考':'显示方式待确认'}</p><p class="small muted">显示头像与规范参考分别保存。</p>`);bindAvatars(el.querySelector('[data-avatar]'));}
  else if(errors.avatar)el.querySelector('[data-avatar]').innerHTML='<h3>显示头像</h3><p>读取失败，可重新读取。</p>';
  el.querySelector('[data-generation-error]').textContent=Object.values(errors).filter(Boolean).join('；');
  el.querySelector('[data-observer-status]').textContent=busy?'正在读取已有状态…':pending()?reads>=60?'自动跟进已暂停；原任务仍保留，不代表超时失败。可以重新读取。':'正在自动跟进这次视觉准备。':'';
 }
 const pending=()=>['queued','running','character_sheet_pending'].includes(identity?.session?.status);
 const timeline=mountCursorRecords(el.querySelector('[data-timeline]'),{alive:live,label:'视觉记录',read:cursor=>visualClient.timeline(fid,cursor),render:VisualTimeline,empty:'尚无视觉任务记录。',bind:bindVisualAssetErrors});
 function schedule(){clearTimeout(timer);if(live()&&!document.hidden&&pending()&&reads<60)timer=setTimeout(()=>void refresh(false),3000);}
 async function refresh(explicit=true){
  if(!live())return;if(busy){if(explicit)refreshAgain=true;return;}
  if(!explicit&&document.hidden)return;
  busy=true;const token=++epoch,captured=sessionId;if(explicit){reads=0;errors.observer='';}paint();
  const results=await Promise.allSettled([visualClient.identity(fid),mediaClient.avatar(fid)]);
  if(!live()||token!==epoch)return;
  const next=results[0];
  if(next.status==='fulfilled'){
   if(!explicit&&captured&&next.value.session?.id!==captured){errors.observer='已有会话已变化。保留当前参考，请重新读取新会话。';reads=60;}
   else {const changed=JSON.stringify(identity)!==JSON.stringify(next.value);identity=next.value;sessionId=identity.session?.id||'';errors.identity='';if(changed)void timeline.reload({onlyIfFirst:true});}
  }else errors.identity='视觉状态更新失败：'+next.reason.message+'。已有图片保留。';
  if(results[1].status==='fulfilled'){avatar=results[1].value;errors.avatar='';}else errors.avatar='显示头像更新失败：'+results[1].reason.message+'。已有头像保留。';
  reads++;busy=false;paint();if(refreshAgain){refreshAgain=false;void refresh();}else schedule();
 }
 el.querySelector('[data-reload]').onclick=()=>void refresh();
 el.querySelector('[data-timeline-refresh]').onclick=()=>timeline.reload({reset:true});
 el.querySelector('[data-generate]').onclick=async()=>{
  if(get('characters',fid)?.status==='已退休')return;
  try{const [visual,brief]=await Promise.all([visualClient.identity(fid),visualClient.read(fid)]);if(!live())return;const supersedeSessionId=visual.session?.status==='awaiting_review'?visual.session.id:undefined;const target={expectedBriefRevision:brief.revision,...(supersedeSessionId?{supersedeSessionId}:{})};
   governanceCommand({scope:[fid,'visual-generate'],title:'生成视觉身份',note:'使用已保存参考定义提交新的视觉准备任务，已有图片与评审记录保留。'+(supersedeSessionId?' 这次会结束等待评审的旧会话并重新开始。':''),revision:visual.current_revision,snapshot:{visual,brief},target,read:async()=>{const [visual,brief]=await Promise.all([visualClient.identity(fid),visualClient.read(fid)]);return {revision:visual.current_revision,visual,brief};},rebaseTarget:latest=>({expectedBriefRevision:latest.brief.revision,...(latest.visual.session?.status==='awaiting_review'?{supersedeSessionId:latest.visual.session.id}:{})}),execute:(body,target)=>visualClient.generate(fid,{...target,expectedVisualRevision:body.expectedRevision,reason:body.reason}),onSaved:()=>refresh()});
  }catch(error){if(live()){errors.observer=error.message;paint();}}
 };
 const visibility=()=>{clearTimeout(timer);if(!document.hidden&&pending()&&reads<60)void refresh(false);};document.addEventListener('visibilitychange',visibility);
 void refresh();
 return()=>{active=false;epoch++;clearTimeout(timer);document.removeEventListener('visibilitychange',visibility);timeline.dispose();};
}
