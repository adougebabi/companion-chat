import {visualReview} from '../components/visual-review.js';
import {visualClient} from '../services.js';
import {get} from '../store.js';
import {asyncPage} from '../components/async-page.js';
import {mountCursorRecords} from '../components/cursor-records.js';
import {mountVersionedFields} from '../components/versioned-fields.js';
import {visualBriefFields,VisualIdentityState,VisualTimeline,bindVisualAssetErrors,VisualBriefSource} from '../components/visual-identity-records.js';
import {governanceCommand} from '../components/governance-command.js';
import {GovernanceHistory} from '../components/governance-records.js';
import {JsonDetails} from '../components/records.js';
import {esc} from '../ui.js';
export function renderRealVisualEditor(el,ctx){
 const fid=ctx.charId;
 return asyncPage(el,()=>visualClient.read(fid),(snapshot,reload,alive)=>{
  el.innerHTML=`<div class="row" style="margin-bottom:12px"><a class="btn" href="#/c/${fid}/media/identity">视觉身份与生成结果</a><button class="btn" data-reload>重新读取</button></div><div class="card"><h2>视觉定义草稿</h2><p>本页编辑参考约束，不改变此刻身体与穿搭。保存会留下真实版本记录，并在新的视觉生成会话中使用。</p>${snapshot.brief?'':'<p class="small muted">尚未保存参考定义。下面是你的未提交输入。</p>'}<div data-brief-source>${VisualBriefSource(snapshot.brief)}</div><div data-editor></div></div><div class="card" style="margin-top:12px"><h3>定义修订</h3><div data-history></div></div>`;
  el.querySelector('[data-reload]').onclick=reload;
  const fields=Object.fromEntries(visualBriefFields.map(([name])=>[name,snapshot.brief?.[name]??'']));let history;
  const editor=mountVersionedFields(el.querySelector('[data-editor]'),{scope:[fid,'visual-brief'],snapshot,values:fields,fields:visualBriefFields,read:()=>visualClient.read(fid),save:body=>visualClient.save(fid,body),readOnly:get('characters',fid)?.status==='已退休',onSaved:saved=>{if(alive())el.querySelector('[data-brief-source]').innerHTML=VisualBriefSource(saved.brief);return history.reload({reset:true});}});
  history=mountCursorRecords(el.querySelector('[data-history]'),{alive,label:'定义修订',read:cursor=>visualClient.history(fid,cursor),render:r=>`<div class="card"><h3>参考定义 v${esc(r.revision)}</h3>${GovernanceHistory(r)}${JsonDetails(r.before_value,'修改前')}${JsonDetails(r.after_value,'修改后')}</div>`,empty:'尚无已保存定义。填写后保存即可建立首个版本。'});
  return()=>{editor();history.dispose();};
 });
}
export function renderRealVisualIdentity(el,ctx){
 const fid=ctx.charId;
 return asyncPage(el,()=>visualClient.identity(fid),(initial,reload,alive)=>{
  let identity=initial,timer,refreshes=0;el.innerHTML=`<div class="row" style="margin-bottom:12px"><a class="btn" href="#/c/${fid}/media/identity/edit">编辑视觉定义</a><a class="btn" href="#/c/${fid}/media/avatar">显示头像</a><button class="btn" data-reload>重新读取</button><button class="btn primary" data-generate ${get('characters',fid)?.status==='已退休'?'disabled':''}>按当前定义生成视觉身份</button></div><div class="card" data-identity></div><p role="alert" data-generation-error></p><div class="card" style="margin-top:12px"><div class="row"><h3 class="grow">完整生成与评审记录</h3><button class="btn" data-timeline-refresh>读取最新记录</button></div><div data-timeline></div></div>`;
  const paint=()=>{if(!alive())return;el.querySelector('[data-identity]').innerHTML=VisualIdentityState(identity);bindVisualAssetErrors(el.querySelector('[data-identity]'));el.querySelector('[data-manual-review]')?.addEventListener('click',()=>{if(get('characters',fid)?.status!=='已退休')visualReview(fid,identity,reload);});};paint();
  const timeline=mountCursorRecords(el.querySelector('[data-timeline]'),{alive,label:'视觉记录',read:cursor=>visualClient.timeline(fid,cursor),render:VisualTimeline,empty:'尚无视觉任务记录。可编辑定义后发起真实生成。',bind:bindVisualAssetErrors});
  el.querySelector('[data-reload]').onclick=()=>{refreshes=0;return reload();};
  el.querySelector('[data-timeline-refresh]').onclick=()=>timeline.reload({reset:true});
  el.querySelector('[data-generate]').onclick=async()=>{
   if(get('characters',fid)?.status==='已退休')return;
   try{const [visual,brief]=await Promise.all([visualClient.identity(fid),visualClient.read(fid)]);if(!alive())return;const supersedeSessionId=visual.session?.status==='awaiting_review'?visual.session.id:undefined;const target={expectedBriefRevision:brief.revision,...(supersedeSessionId?{supersedeSessionId}:{})};
    governanceCommand({scope:[fid,'visual-generate'],title:'生成视觉身份',note:'提交真实视觉身份任务，使用当前有效外貌、穿着和已保存参考定义。生成图不会改变生活事实。工作流未配置或生成失败会保留真实错误与已有产物。'+(supersedeSessionId?' 本次将明确结束等待评审的旧会话并重新开始，旧候选和评审保留。':''),revision:visual.current_revision,snapshot:{visual,brief},target,read:async()=>{const [visual,brief]=await Promise.all([visualClient.identity(fid),visualClient.read(fid)]);return {revision:visual.current_revision,visual,brief};},rebaseTarget:latest=>({expectedBriefRevision:latest.brief.revision,...(latest.visual.session?.status==='awaiting_review'?{supersedeSessionId:latest.visual.session.id}:{})}),execute:(body,target)=>visualClient.generate(fid,{...target,expectedVisualRevision:body.expectedRevision,reason:body.reason}),onSaved:()=>{refreshes=0;return reload();}});
   }catch(e){if(alive())el.querySelector('[data-generation-error]').textContent=e.message;}
  };
  async function poll(){if(!alive()||++refreshes>60||!['queued','running','character_sheet_pending'].includes(identity.session?.status))return;try{const before=JSON.stringify([identity.current_revision,identity.session]);identity=await visualClient.identity(fid);paint();if(before!==JSON.stringify([identity.current_revision,identity.session])){await timeline.reload({onlyIfFirst:true});}}catch(e){if(alive())el.querySelector('[data-generation-error]').textContent=e.message;return;}if(alive())timer=setTimeout(poll,3000);}
  if(['queued','running','character_sheet_pending'].includes(identity.session?.status))timer=setTimeout(poll,3000);
  return()=>{clearTimeout(timer);timeline.dispose();};
 });
}
