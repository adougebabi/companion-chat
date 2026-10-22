import {requestDecisionText,RequestDecisionSource} from '../components/request-failure.js';
import {stateLabel,errorMessage} from '../../domain-presentation.mjs';
import {userInstant} from '../../presentation-time.mjs';
import {bindAvatars} from '../components/avatar.js';
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
 const owner=get('session')?.actorId;let active=true,filter={fluctlightId:ctx.query?.fid??ctx.charId??'',status:['visible','hidden','all'].includes(ctx.query?.status)?ctx.query.status:'visible',q:ctx.query?.q||''};const alive=()=>active&&el.isConnected&&get('session')?.actorId===owner;
 el.innerHTML=`<header class="media-workspace-header"><h2 class="media-business-title">动态</h2></header><form class="card row" data-filters>${Field({label:'角色范围',name:'fid',type:'select',value:filter.fluctlightId,options:[{value:'',label:'全局动态'},...list('characters').map(c=>({value:c.id,label:c.name}))]})}${Field({label:'展示状态',name:'status',type:'select',value:filter.status,options:[{value:'visible',label:'可见动态'},{value:'hidden',label:'已隐藏'},{value:'all',label:'全部'}]})}${Field({label:'搜索正文',name:'q',value:filter.q,attrs:{maxlength:200}})}<button class="btn">查询</button><button class="btn" type="button" data-refresh>刷新动态</button></form><p role="alert" data-read-error></p><div class="moment-workspace"><div class="row moment-compose-actions"><button class="btn" data-toggle-composer="publish">发布我的动态</button><button class="btn" data-toggle-composer="suggest">建议角色发布</button></div><div data-publish hidden></div><div data-suggest hidden></div><div data-feed></div></div>`;
 const feed=mountCursorRecords(el.querySelector('[data-feed]'),{alive,label:'动态',read:cursor=>momentsClient.list({...filter,cursor}),render:post=>MomentCard(post,{returnTo:location.hash}),empty:'当前筛选还没有动态。可以以自己的身份发布，或建议角色自行判断是否分享。',bind:(box,page)=>{if(!page.items.length){const filtered=!!filter.q||filter.status!=='visible';box.querySelector('p').textContent=filtered?'没有符合当前搜索或展示状态的动态。':'当前范围还没有动态。可以发布自己的动态，或建议角色自行判断是否分享。';if(filtered)box.insertAdjacentHTML('beforeend',`<a class="btn" href="#/feed?fid=${encodeURIComponent(filter.fluctlightId)}&status=visible">清除搜索和展示筛选</a>`);}bindMediaFailures(box);bindAvatars(box);box.querySelectorAll('[data-moment-action]').forEach(b=>b.onclick=()=>{const post=page.items.find(p=>p.id===b.dataset.momentAction);if(post&&alive())openMomentAction(post,b.dataset.action,()=>feed.reload(),b.dataset.reaction);});}});

 async function refresh(){if(!alive())return;await Promise.all([feed.reload({reset:true}),suggestions.reload({reset:true})]);}
 el.querySelector('[data-feed]').insertAdjacentHTML('afterend','<details class="card moment-suggestion-results"><summary>我的建议与实际结果</summary><div data-suggestions></div></details>');
 const suggestions=mountCursorRecords(el.querySelector('[data-suggestions]'),{alive,label:'动态建议',read:cursor=>momentsClient.suggestions(filter.fluctlightId,cursor),render:r=>`<div class="lrow"><div><b>${esc(r.payload?.text)}</b><p class="small muted">${esc(stateLabel(r.status))} · ${esc(requestDecisionText(r)||'等待角色判断')} · ${esc(userInstant(r.created_at))}</p><a href="#/diag/request/${encodeURIComponent(r.id)}">请求详情与实际任务</a>${RequestDecisionSource(r)}${(r.result?.effects||[]).filter(e=>e.output?.target_kind==='moment'&&e.output?.target_ref).map(e=>`<a class="btn" href="#/feed/${encodeURIComponent(e.output.target_ref)}">查看角色实际发布</a>`).join('')}</div></div>`,empty:'当前范围尚未提出动态建议。发出建议后可以在这里持续查阅结果。'});
 el.querySelectorAll('[data-toggle-composer]').forEach(button=>button.onclick=()=>{const box=el.querySelector(`[data-${button.dataset.toggleComposer}]`);box.hidden=!box.hidden;button.setAttribute('aria-expanded',String(!box.hidden));if(!box.hidden)box.querySelector('textarea')?.focus();});
 const publish=mountMomentComposer(el.querySelector('[data-publish]'),{fid:filter.fluctlightId,alive,onSaved:async receipt=>{await refresh();if(alive()&&receipt?.id)location.hash=`#/feed/${encodeURIComponent(receipt.id)}?returnTo=${encodeURIComponent(location.hash)}`;}});
 const suggest=mountMomentComposer(el.querySelector('[data-suggest]'),{mode:'suggest',fid:ctx.charId||'',alive,onSaved:refresh});
 el.querySelector('[data-filters]').onsubmit=e=>{e.preventDefault();filter={fluctlightId:e.target.elements.fid.value,status:e.target.elements.status.value,q:e.target.elements.q.value.trim()};const query=new URLSearchParams({fid:filter.fluctlightId,status:filter.status,...(filter.q?{q:filter.q}:{})});history.replaceState(null,'',location.hash.split('?')[0]+'?'+query);void refresh();};el.querySelector('[data-refresh]').onclick=()=>void refresh();
 return()=>{active=false;feed.dispose();publish.dispose();suggest.dispose();suggestions.dispose();};
}

export function renderRealPost(el,ctx){
 const owner=get('session')?.actorId;
 const feedHref=ctx.query?.returnTo?.startsWith('#/feed')?ctx.query.returnTo:'#/feed';
 return asyncPage(el,async()=>{try{return await momentsClient.record(ctx.params.postId);}catch(error){return {readError:error};}},(post,reload,connected)=>{
  if(post.readError){el.innerHTML=`<div class="statebox" role="alert"><h2>动态暂不可读取</h2><p>${esc(errorMessage(post.readError,'动态不存在或当前无权读取。'))}</p><a class="btn" href="${esc(feedHref)}">返回动态</a><button class="btn" data-retry>重新读取</button></div>`;el.querySelector('[data-retry]').onclick=reload;return;}
  const alive=()=>connected()&&get('session')?.actorId===owner;
  el.innerHTML=`<header class="media-workspace-header"><h2 class="media-business-title">动态详情</h2></header><div class="row"><a class="btn" href="${esc(feedHref)}">返回动态</a><button class="btn" type="button" data-refresh-post>刷新动态与互动状态</button></div>${MomentCard(post,{full:true})}<div class="card"><h3>评论</h3><div data-comments></div></div><div data-comment-compose></div><div class="card"><h3>NPC 互动与来源</h3><p class="small muted">只对有实际结识来源的当前世界人物发起独立判断。可以不回应；远程互动不代表同场，也不新建关系。</p><div data-npc-interactions></div></div>${JsonDetails({id:post.id,author_actor_id:post.author_actor_id,author_type:post.author_type,fluctlight_id:post.owner_fluctlight_id,request_id:post.request_id,source_event_id:post.source_event_id,revision:post.revision},'身份与实际来源')}<p role="alert" data-read-error></p>`;
  el.querySelector('[data-refresh-post]').onclick=reload;el.querySelector('[data-scroll-comments]').onclick=()=>el.querySelector('[data-comments]').scrollIntoView({block:'start'});
  bindMediaFailures(el);bindAvatars(el);el.querySelectorAll('[data-moment-action]').forEach(b=>b.onclick=()=>{if(alive())openMomentAction(post,b.dataset.action,reload,b.dataset.reaction);});
  const comments=mountCursorRecords(el.querySelector('[data-comments]'),{alive,label:'评论',read:cursor=>momentsClient.comments(post.id,cursor),render:c=>`<div class="lrow"><div class="grow"><b>${esc(c.author_type==='user'?'我':c.author_name||'作者名称未记录')}</b><p class="prewrap">${esc(c.text)}</p><span class="small muted">${esc(userInstant(c.created_at))} · ${esc(({user:'我',fluctlight:'角色',npc:'世界人物'})[c.author_type]||'作者类型未记录')}</span></div></div>`,empty:'还没有评论。可在下方以自己的身份回应。'});
  const composer=post.status==='visible'?mountMomentComposer(el.querySelector('[data-comment-compose]'),{mode:'comment',moment:post,alive,onSaved:reload}):null;
  const interactions=mountCursorRecords(el.querySelector('[data-npc-interactions]'),{alive,label:'NPC 互动',read:cursor=>momentsClient.npcInteractions(post.id,cursor),render:r=>`<div class="lrow"><div class="grow"><b>${esc({pending:'等待独立判断',completed:'实际互动已提交',skipped:'本次不回应',suppressed:'来源变化，未产生互动',failed:'执行失败'}[r.status]||r.status)}</b><p class="small muted">NPC ${esc(r.npc_id)} · 尝试 ${esc(r.attempt)} · v${esc(r.revision)}</p><p>${esc(r.response?.reason||r.error_code||'')}</p>${(r.tasks||[]).map(t=>`<a class="btn" href="#/diag/task/${encodeURIComponent(t.task_id)}">实际任务 · ${esc(t.status)}</a>`).join('')}${JsonDetails({encounter_id:r.encounter_id,source_event_id:r.source_event_id,npc_actor_id:r.npc_actor_id,comment_id:r.comment_id,response:r.response,copresent:false},'原结识事件和实际结果')}${['failed','suppressed'].includes(r.status)?`<button class="btn" type="button" data-retry-npc="${esc(r.id)}">重试本次互动</button>`:''}</div></div>`,empty:'没有已受理的 NPC 互动。全局用户动态，或尚未实际结识当前世界人物时，不会生成互动任务。',bind:(box,page)=>box.querySelectorAll('[data-retry-npc]').forEach(b=>b.onclick=()=>{const r=page.items.find(i=>i.id===b.dataset.retryNpc);if(r&&alive())openMomentAction(r,'retry-npc',reload);})});
  if(!composer)el.querySelector('[data-comment-compose]').innerHTML='<p class="banner info">这条动态已隐藏。恢复展示后可以继续评论。</p>';
  return()=>{comments.dispose();composer?.dispose();interactions.dispose();};
 });
}
