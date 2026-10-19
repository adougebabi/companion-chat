import {get,list} from '../store.js';
import {momentsClient} from '../services.js';
import {esc} from '../ui.js';
import {Field} from '../components/forms.js';
import {JsonDetails} from '../components/records.js';
import {asyncPage} from '../components/async-page.js';
import {mountCursorRecords} from '../components/cursor-records.js';
import {bindMediaFailures} from '../components/media-assets.js';
import {MomentCard} from '../components/moment-card.js';
import {mountMomentComposer,openMomentAction} from '../components/moment-composer.js';

export function renderRealFeed(el,ctx){
 const owner=get('session')?.actorId;let active=true,filter={fluctlightId:ctx.charId||'',status:'visible',q:''};const alive=()=>active&&el.isConnected&&get('session')?.actorId===owner;
 el.innerHTML=`<form class="card row" data-filters>${Field({label:'角色范围',name:'fid',type:'select',value:filter.fluctlightId,options:[{value:'',label:'全局动态'},...list('characters').map(c=>({value:c.id,label:c.name}))]})}${Field({label:'展示状态',name:'status',type:'select',value:'visible',options:[{value:'visible',label:'可见动态'},{value:'hidden',label:'已隐藏'},{value:'all',label:'全部'}]})}${Field({label:'搜索正文',name:'q',attrs:{maxlength:200}})}<button class="btn">查询</button><button class="btn" type="button" data-refresh>刷新动态</button></form><p role="alert" data-read-error></p><div class="grid g2"><div class="stack" data-feed></div><div class="stack"><div data-publish></div><div data-suggest></div></div></div>`;
 const feed=mountCursorRecords(el.querySelector('[data-feed]'),{alive,label:'动态',read:cursor=>momentsClient.list({...filter,cursor}),render:post=>MomentCard(post),empty:'当前筛选还没有动态。可以以自己的身份发布，或建议角色自行判断是否分享。',bind:(box,page)=>{bindMediaFailures(box);box.querySelectorAll('[data-moment-action]').forEach(b=>b.onclick=()=>{const post=page.items.find(p=>p.id===b.dataset.momentAction);if(post&&alive())openMomentAction(post,b.dataset.action,()=>feed.reload());});}});
 el.querySelector('[data-filters]').insertAdjacentHTML('beforeend','<button class="btn" type="button" data-mark-read>标记所选角色动态已读</button>');
 el.querySelector('[data-mark-read]').onclick=async e=>{if(!filter.fluctlightId){el.querySelector('[data-read-error]').textContent='请先查询一个明确的角色范围，再标记该角色的动态已读。';return;}const fid=filter.fluctlightId;e.target.disabled=true;try{await momentsClient.read(fid);if(alive())el.querySelector('[data-read-error]').textContent='已将所选角色的动态标记为已读。';}catch(error){if(alive())el.querySelector('[data-read-error]').textContent=error.message;}finally{if(alive())e.target.disabled=false;}};
 async function refresh(){if(!alive())return;await Promise.all([feed.reload({reset:true}),suggestions.reload({reset:true})]);}
 el.querySelector('[data-suggest]').insertAdjacentHTML('afterend','<div class="card"><h3>我的建议与实际结果</h3><div data-suggestions></div></div>');
 const suggestions=mountCursorRecords(el.querySelector('[data-suggestions]'),{alive,label:'动态建议',read:cursor=>momentsClient.suggestions(filter.fluctlightId,cursor),render:r=>`<div class="lrow"><div><b>${esc(r.payload?.text)}</b><p class="small muted">${esc(r.status)} · ${esc(r.decision_reason||'等待角色判断')} · ${esc(r.created_at)}</p><a href="#/diag/request/${encodeURIComponent(r.id)}">请求详情与实际任务</a>${(r.result?.effects||[]).filter(e=>e.output?.target_kind==='moment'&&e.output?.target_ref).map(e=>`<a class="btn" href="#/feed/${encodeURIComponent(e.output.target_ref)}">查看角色实际发布</a>`).join('')}</div></div>`,empty:'当前范围尚未提出动态建议。发出建议后可以在这里持续查阅结果。'});
 const publish=mountMomentComposer(el.querySelector('[data-publish]'),{alive,onSaved:refresh});
 const suggest=mountMomentComposer(el.querySelector('[data-suggest]'),{mode:'suggest',fid:ctx.charId||'',alive,onSaved:refresh});
 el.querySelector('[data-filters]').onsubmit=e=>{e.preventDefault();filter={fluctlightId:e.target.elements.fid.value,status:e.target.elements.status.value,q:e.target.elements.q.value.trim()};void refresh();};el.querySelector('[data-refresh]').onclick=()=>void refresh();
 return()=>{active=false;feed.dispose();publish.dispose();suggest.dispose();suggestions.dispose();};
}

export function renderRealPost(el,ctx){
 const owner=get('session')?.actorId;
 return asyncPage(el,()=>momentsClient.record(ctx.params.postId),(post,reload,connected)=>{
  const alive=()=>connected()&&get('session')?.actorId===owner;
  el.innerHTML=`<div class="row"><a class="btn" href="#/feed">返回动态</a><button class="btn" type="button" data-refresh-post>刷新动态与互动状态</button></div>${MomentCard(post,{full:true})}<div class="card"><h3>评论</h3><div data-comments></div></div><div data-comment-compose></div><div class="card"><h3>NPC 互动与来源</h3><p class="small muted">只对有实际结识来源的当前世界人物发起独立判断。可以不回应；远程互动不代表同场，也不新建关系。</p><div data-npc-interactions></div></div>${JsonDetails({id:post.id,author_actor_id:post.author_actor_id,author_type:post.author_type,fluctlight_id:post.owner_fluctlight_id,request_id:post.request_id,source_event_id:post.source_event_id,revision:post.revision},'身份与实际来源')}<p role="alert" data-read-error></p>`;
  el.querySelector('[data-refresh-post]').onclick=reload;
  bindMediaFailures(el);el.querySelectorAll('[data-moment-action]').forEach(b=>b.onclick=()=>{if(alive())openMomentAction(post,b.dataset.action,reload);});
  const comments=mountCursorRecords(el.querySelector('[data-comments]'),{alive,label:'评论',read:cursor=>momentsClient.comments(post.id,cursor),render:c=>`<div class="lrow"><div class="grow"><b>${esc(c.author_type==='user'?'我':c.author_name||'作者名称未记录')}</b><p class="prewrap">${esc(c.text)}</p><span class="small muted">${esc(c.created_at)} · ${esc(c.author_type)}</span></div></div>`,empty:'还没有评论。可在下方以自己的身份回应。'});
  const composer=post.status==='visible'?mountMomentComposer(el.querySelector('[data-comment-compose]'),{mode:'comment',moment:post,alive,onSaved:reload}):null;
  const interactions=mountCursorRecords(el.querySelector('[data-npc-interactions]'),{alive,label:'NPC 互动',read:cursor=>momentsClient.npcInteractions(post.id,cursor),render:r=>`<div class="lrow"><div class="grow"><b>${esc({pending:'等待独立判断',completed:'实际互动已提交',skipped:'本次不回应',suppressed:'来源变化，未产生互动',failed:'执行失败'}[r.status]||r.status)}</b><p class="small muted">NPC ${esc(r.npc_id)} · 尝试 ${esc(r.attempt)} · v${esc(r.revision)}</p><p>${esc(r.response?.reason||r.error_code||'')}</p>${(r.tasks||[]).map(t=>`<a class="btn" href="#/diag/task/${encodeURIComponent(t.task_id)}">实际任务 · ${esc(t.status)}</a>`).join('')}${JsonDetails({encounter_id:r.encounter_id,source_event_id:r.source_event_id,npc_actor_id:r.npc_actor_id,comment_id:r.comment_id,response:r.response,copresent:false},'原结识事件和实际结果')}${['failed','suppressed'].includes(r.status)?`<button class="btn" type="button" data-retry-npc="${esc(r.id)}">重试本次互动</button>`:''}</div></div>`,empty:'没有已受理的 NPC 互动。全局用户动态，或尚未实际结识当前世界人物时，不会生成互动任务。',bind:(box,page)=>box.querySelectorAll('[data-retry-npc]').forEach(b=>b.onclick=()=>{const r=page.items.find(i=>i.id===b.dataset.retryNpc);if(r&&alive())openMomentAction(r,'retry-npc',reload);})});
  if(!composer)el.querySelector('[data-comment-compose]').innerHTML='<p class="banner info">这条动态已隐藏。恢复展示后可以继续评论。</p>';
  return()=>{comments.dispose();composer?.dispose();interactions.dispose();};
 });
}
