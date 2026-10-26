import {errorMessage} from '../v3/domain-presentation.mjs';
import {SemanticValue} from '../v3/proto/components/semantic-fields.js';
import {userInstant} from '../v3/presentation-time.mjs';
import {sourceHref} from '../v3/proto/components/source-navigation.js';
import {openOwnerMediaDrawer} from '../v3/proto/components/media-generation-drawer.js';
import {mountWorldEventRuntime} from '../v3/proto/components/world-event-runtime.js';
import {ActiveWorldEffects} from '../v3/proto/components/world-events.js';
import {mountNPCRuntimeRecords} from '../v3/proto/components/npc-runtime-records.js';
import {PositionSummary} from '../v3/proto/components/routine.js';
import {encounterDrawer} from './npc-management.js';
import {Avatar,bindAvatars} from '../v3/proto/components/avatar.js';
import {NPCRow,PersonCard} from '../v3/proto/components/cards.js';
import * as store from '../v3/proto/store.js';
import {esc,icon,badge,toast,objLink,mgmt,nameOf,confirm,requireOwner} from '../v3/proto/ui.js';
import {KeyValueList,Field,Tabs,SearchForm} from '../v3/proto/components/forms.js';
import {belongsToWorld,mockWorld,mockClient} from '../v3/proto/services.js';
const here=ctx=>store.get('characters',ctx.charId).locationId;
const inWorld=(ctx,object)=>object.worldId?object.worldId===store.get('characters',ctx.charId)?.worldId:store.get('places',object.placeId)?.worldId===store.get('characters',ctx.charId)?.worldId;
function renderPlacePage(el, ctx) {
  const p = store.get('places', ctx.params.placeId), c=store.get('characters',ctx.charId);
  if (!p || !inWorld(ctx, p)) { el.innerHTML = '<div class="statebox"><h2>当前范围未找到该地点</h2></div>'; return; }
  const world=store.get('worlds',c.worldId), category=world?.venueCategories?.find(v=>v.id===(p.venueCategoryId||p.venue_category_id));
  const location=here(ctx)===p.id?`最近确认角色在${p.alias||p.name}。`:`角色最近确认的位置是${nameOf(here(ctx))||'未知'}。`;
  el.innerHTML = `<div class="place-reading stack"><header><p class="small muted"><a href="#/c/${ctx.charId}/world">${esc(world?.name||'当前世界')}</a>${category?' / '+esc(category.name):''} / ${esc(p.name)}</p><div class="row"><h2 class="grow">${esc(p.alias||p.name)}</h2><a class="btn ghost sm" href="#/c/${ctx.charId}/worlds/${esc(c.worldId)}/place/${esc(p.id)}/edit">编辑地点</a></div>${p.alias?`<p class="small muted">正式名称：${esc(p.name)}</p>`:''}<p class="small muted">${esc(location)} 浏览地点不会改变角色的位置。</p></header>
  ${p.scene?`<img class="place-scene" src="${esc(p.scene)}" alt="${esc(p.alias||p.name)}的场景参考" data-place-image>`:''}
  <section class="reading-paper"><h3>这里的样子</h3><p class="memory-content">${esc(p.desc||'场景描述尚未补充')}</p>${KeyValueList([['楼层',esc(p.floor||'尚未补充')],['氛围',esc(p.atmosphere||'尚未补充')],['陈设',SemanticValue(p.furniture)||'尚未补充'],['布局',esc(p.layout||'尚未补充')],['连通说明',esc(p.connections||'尚未补充')]])}</section>
  <div class="grid g3"><section class="card"><h3>这里的人</h3>${store.list('npcs', n => belongsToWorld(ctx.charId,n)&&!n.archived&&n.enabled!==false&&n.currentPlaceId===p.id).map(n=>NPCRow(n,objLink)+badge(n.presence==='confirmed'?'已确认在场':'预计位置','plain')).join('')||'<p class="small muted">当前没有已确认或预计在场的人物。</p>'}</section>
  <section class="card"><h3>相关安排</h3>${store.list('schedule',v=>v.placeId===p.id&&(!v.charId||v.charId===ctx.charId)).map(v=>`<div class="lrow small">${esc(v.time)} ${esc(v.title)} ${badge(v.state)}</div>`).join('')||'<p class="small muted">当前读取范围没有相关安排。</p>'}<a href="#/c/${ctx.charId}/life/schedule">查看日程</a></section>
  <section class="card"><h3>发生过的事</h3>${store.list('events',v=>v.placeId===p.id&&inWorld(ctx,v)).map(v=>`<div class="lrow small">${objLink(v.id)} ${badge(v.state)}</div>`).join('')||'<p class="small muted">当前读取范围没有相关事件。</p>'}<a href="#/c/${ctx.charId}/media">查看媒体库</a></section></div>
  ${mgmt(`${esc(p.id)}${p.resourceId?' · 定义 '+objLink(p.resourceId,p.resourceId):''}`)}</div>`;
  const image=el.querySelector('[data-place-image]');image?.addEventListener('error',()=>image.remove(),{once:true});
}
function renderScenePage(el, ctx, alive, view) {
  const c = store.get('characters', ctx.charId); const p = store.get('places', c.locationId);
  if (!p || p.worldId !== c.worldId) { el.innerHTML = `<div class="statebox"><h2>当前世界的位置尚未确认</h2><p class="muted">可以先查看世界中的地点，或在世界管理中完善地点。浏览不会改变角色的位置。</p><div class="row"><a class="btn" href="#/c/${ctx.charId}/world">查看世界</a><a class="btn" href="#/c/${ctx.charId}/worlds">世界与地点管理</a></div></div>`; return; }
  el.innerHTML = `<section class="hero ${p.scene?'':'no-image'}">${p.scene?`<img class="scene" src="${esc(p.scene)}" alt="">`:''}<div class="over">${Avatar({src:c.avatar,name:c.name,size:'lg'})}<div class="grow"><div class="small muted">远程查看 · 你不在场</div><h2 style="margin:2px 0">${esc(p.name)}</h2><p class="small">${esc(c.doing)}</p></div></div></section>
  <div class="grid g2" style="margin-top:12px"><div class="card"><h3>周围</h3>${store.list('npcs', n => belongsToWorld(ctx.charId, n) && !n.archived && n.enabled !== false && n.currentPlaceId === p.id).map(n => NPCRow(n, objLink) + badge(n.presence === 'confirmed' ? '已确认' : '预计', 'plain')).join('') || '<p class="small muted">当前无已确认或预计在场的 NPC</p>'}<p class="small muted">${esc(p.desc)}</p></div>
   <div class="card"><h3>${icon('camera')} 看看角色</h3><p class="small muted">生成一张此刻的画面，${badge('生成图·非事实')}，不改变任何事实。没有“进入场景”。</p><button class="btn primary" data-photo data-test="scene-photo">${icon('camera')} 拍一张</button></div></div>`;
  el.insertAdjacentHTML('beforeend',ActiveWorldEffects(view?.current_context?.world_effects,{fid:ctx.charId,zone:c.timezone}));
  el.querySelector('[data-photo]').disabled = c.status === '已退休';
  el.querySelector('[data-photo]').onclick = () => openOwnerMediaDrawer({fid:ctx.charId,title:'当前场景图片请求',place:{id:p.id,name:p.name,revision:p.revision},prompt:'请画一张你此刻实际场景中的画面，保留当前有效外貌与合法穿着；图片只是画面，不代表任何移动、换装或新事件。'});
}
function renderPeopleListPage(el, ctx) {
  const kw=ctx.query.q||'',scope=['all','known','world'].includes(ctx.query.scope)?ctx.query.scope:'all';
  const relation=n=>store.list('relations',r=>r.charId===ctx.charId&&(r.npcId===n.id||r.targetActorId===n.actor_id))[0];
  const ns=store.list('npcs',n=>belongsToWorld(ctx.charId,n)&&!n.archived&&(scope!=='known'||relation(n))&&(!kw||[n.name,n.role,nameOf(n.placeId)].some(v=>String(v||'').includes(kw))));
  const directoryHref=`#/c/${ctx.charId}/people${ctx.raw?.split('?')[0].endsWith('/list')?'/list':''}`;
  const link=value=>`${directoryHref}?${new URLSearchParams({q:kw,scope:value})}`;
  el.innerHTML=`<div class="stack"><header class="world-review-heading people-review-heading"><div><h2>人物关系</h2><p class="small muted">查看人物资料与已经记录的关系</p></div><a class="btn" href="#/c/${ctx.charId}/people?view=relations">查看已记录的关系</a></header>${Tabs({value:scope,items:[['all','全部'],['known','已结识'],['world','世界人物']].map(([value,label])=>({value,label,href:link(value)}))})}${SearchForm({value:kw,placeholder:'搜索姓名、身份或地点',attrs:{'data-pf':true},inputAttrs:{'data-test':'people-q'}})}<div class="person-directory">${scope!=='world'&&(!kw||'关于你 所有者 我'.includes(kw))?PersonCard({name:store.get('user')?.name||'你',description:'所有者',status:'角色视角下与你的关系',href:sourceHref(`#/c/${ctx.charId}/people/owner`,ctx,'返回人物目录'),label:'关于你'}):''}${ns.map(n=>PersonCard({name:n.name,avatar:n.avatar,description:n.role||n.bio||'身份尚未补充',status:relation(n)?.stage||'尚无关系记录',href:sourceHref(`#/c/${ctx.charId}/people/npc/${encodeURIComponent(n.id)}`,ctx,'返回人物目录'),attrs:{'data-test':`npc-${n.id}`}})).join('')}</div>${!ns.length?`<div class="statebox"><h3>${kw?'没有匹配的人物':scope==='known'?'尚未记录结识的人物':'尚未添加世界人物'}</h3><p>${kw?'可以换个关键词，或清除筛选查看人物目录。':scope==='known'?'浏览人物不会自动建立关系；实际接触后以已有记录为准。':'可以创建人物定义，再明确应用到这个世界。'}</p>${kw||scope!=='all'?`<a class="btn" href="${directoryHref}">清除筛选</a>`:''}</div>`:''}<div class="row"><a class="btn" href="#/c/${ctx.charId}/people/npcs">管理世界人物</a><a class="btn primary" href="#/c/${ctx.charId}/people/npc/new">创建人物定义</a></div></div>`;
  el.querySelector('[data-pf]').onsubmit=e=>{e.preventDefault();ctx.navigate(`${directoryHref}?${new URLSearchParams({q:e.target.q.value,scope})}`);};
}
async function renderNpcPage(el, ctx,alive=()=>true) {
  const n = await mockClient.npc(ctx.charId, ctx.params.npcId);if(!alive())return;const c=store.get('characters',ctx.charId), context=n.relationship_context || {}, relations=context.relationships || [], history=context.relationship_history || [];
  const refs=n.encounters || [];
  el.innerHTML=`<div class="grid g2" style="align-items:start"><div class="card"><div class="row">${Avatar({src:n.avatar,name:n.name,size:'lg'})}<div class="grow"><h2 style="margin:0">${esc(n.name)}</h2><div class="small muted">${esc(n.role)} · 初始地点 ${esc(nameOf(n.placeId) || '未知')}</div></div></div><p>${esc(n.bio)}</p><div class="banner info small" data-test="npc-not-full">NPC 是独立运行人物，不是完整摇光。生活规律仅用于预计；实际接触必须有事件与证据来源。</div><h3 style="margin-top:12px">与 ${esc(c.name)} 有关的实际接触</h3><div data-npc-runtime-records></div></div><div class="card"><h3>与 ${esc(c.name)} 的关系</h3>${relations.map(r=>`<div class="lrow"><span>${esc(r.summary || r.stage || '已有关系记录')} · v${esc(r.revision)}</span><a class="btn sm" href="#/c/${ctx.charId}/people/relation/${esc(r.id)}">修正关系…</a></div>`).join('') || '<p class="small muted">尚无关系记录</p>'}<h3 style="margin-top:12px">关系变化</h3><div class="chain" data-test="rel-revs">${history.map(r=>`<div class="lrow small">v${esc(r.revision)} · ${esc(r.summary || r.reason || '')}<span class="muted">${esc(userInstant(r.created_at))}</span></div>`).join('') || '<p class="small muted">尚无关系修订</p>'}</div><h3 style="margin-top:12px">位置与来源</h3>${PositionSummary({npc:n,placeName:value=>nameOf(value)||value||'未知'})}<p class="small muted">预计位置：${esc(nameOf(n.planned_position?.place_id)||'未知')}。预计不代表实际相遇。</p></div></div><div class="row" style="margin-top:12px"><a class="btn" href="#/c/${ctx.charId}/people/npc/${n.id}/edit">编辑资料与生活规律</a><a class="btn ghost" href="#/c/${ctx.charId}/people/npcs">NPC 管理</a><button class="btn" data-encounter-npc ${c.status==='已退休'?'disabled':''}>记录所有者确认的实际接触</button></div>`;
  el.querySelector('[data-encounter-npc]').onclick=()=>encounterDrawer(ctx.charId,n.id);
  const panel=mountNPCRuntimeRecords(el.querySelector('[data-npc-runtime-records]'),{fid:ctx.charId,npcId:n.id,zone:c.timezone,alive,readOnly:c.status==='已退休',initial:ctx.query.npcTab,onTab:tab=>{const url=new URL(location.href);url.hash=`#/c/${ctx.charId}/people/npc/${encodeURIComponent(n.id)}?${new URLSearchParams({...ctx.query,npcTab:tab})}`;window.history.replaceState(null,'',url);}});return()=>panel.dispose();
}
async function renderRelEditPage(el,ctx) {
  const projection=store.get('relations',ctx.params.relId);
  if(!projection || projection.charId!==ctx.charId) {el.innerHTML='<div class="statebox"><h2>当前范围未找到该关系</h2></div>';return;}
  const actorId=projection.targetActorId;if(!actorId)throw new Error('关系缺少稳定 Actor 身份，不能提交修正');
  const {relation:r,history}=await mockClient.readRelation(ctx.charId,actorId,projection.id);
  el.innerHTML=`<div class="card" style="max-width:640px"><div class="row"><h3 class="grow">关系修正</h3>${badge('所有者修改')}</div><p class="small muted">以 v${esc(r.revision)} 为基础追加修订，保留旧记录与来源。</p><form class="stack" data-rf>${Field({label:'关系描述',name:'summary',value:r.summary || '',attrs:{required:true}})}${Field({label:'修正原因',name:'reason',attrs:{required:true}})}${Field({label:'证据引用（逗号分隔）',name:'evidenceRefs',attrs:{required:true}})}<details class="mgmt"><summary>关系语义与指标</summary><div class="stack" style="margin-top:8px">${Field({label:'关系语义',name:'semantic',value:r.role?.label || r.role?.primary || ''})}${Field({label:'亲密度（推测，0—100；留空表示未知）',name:'intimacy',value:typeof r.metrics?.intimacy==='number' ? r.metrics.intimacy*100 : '',type:'number',attrs:{min:0,max:100,step:'any'}})}</div></details><div class="row"><button type="button" class="btn" data-back>取消</button><button class="btn accent">保存修正</button></div><p data-error role="alert"></p></form></div><details class="mgmt small muted" style="max-width:640px;margin-top:12px"><summary>对象历史与恢复</summary><p>恢复通过受控命令追加一个新修订，保留来源与生活时间线。</p>${history.map(v=>`<div class="lrow"><span class="grow">v${esc(v.revision)} · ${esc(v.summary || '')}</span><button class="btn sm" data-restore-relation="${esc(v.revision)}">恢复此修订字段</button></div>`).join('') || '<p>暂无可恢复修订</p>'}</details>`;
  el.querySelector('[data-back]').onclick=()=>ctx.navigate(`#/c/${ctx.charId}/people`);
  const evidence=()=>el.querySelector('[name=evidenceRefs]').value.split(',').map(v=>v.trim()).filter(Boolean);
  el.querySelector('[data-rf]').onsubmit=async e=>{e.preventDefault();if(!(await requireOwner()))return;const f=e.target;try{const metrics={...r.metrics};if(f.intimacy.value!=='')metrics.intimacy=Number(f.intimacy.value)/100;else delete metrics.intimacy;await mockClient.saveRelation(ctx.charId,actorId,{profileId:r.profile_id || undefined,expectedRevision:r.revision,summary:f.summary.value,role:{...r.role,label:f.semantic.value},metrics,evidenceRefs:evidence(),reason:f.reason.value});toast('关系修订已保存并读回','ok');ctx.navigate(`#/c/${ctx.charId}/people`);}catch(error){f.querySelector('[data-error]').textContent=error.message;}};
  el.querySelectorAll('[data-restore-relation]').forEach(button=>button.onclick=async()=>{try{if(!(await requireOwner()))return;if(!evidence().length)throw new Error('请先填写本次恢复的证据引用');if(!(await confirm({title:'恢复关系修订字段',text:'以历史字段追加新修订，保留历史与来源。'})))return;await mockClient.restoreRelation(ctx.charId,actorId,{targetRevision:Number(button.dataset.restoreRelation),expectedRevision:r.revision,profileId:r.profile_id || undefined,evidenceRefs:evidence()});ctx.refresh();}catch(error){el.querySelector('[data-error]').textContent=error.message;}});
}
function worldPage(render) {return (el,ctx)=>{let alive=true,epoch=0,dispose;el.innerHTML='<div class="statebox" role="status">正在读取世界…</div>';const load=async()=>{if(!alive)return;const version=++epoch;dispose?.();dispose=null;try{const view=await mockWorld.refresh(ctx.charId);if(!alive||version!==epoch)return;const host=document.createElement('div');el.replaceChildren(host);const cleanup=await render(host,ctx,()=>alive&&version===epoch&&host.isConnected,view);bindAvatars(host);host.querySelectorAll('img.scene').forEach(image=>{const failed=()=>{image.closest('.hero')?.classList.add('no-image');image.remove();};image.addEventListener('error',failed,{once:true});if(image.complete&&!image.naturalWidth)failed();});if(alive&&version===epoch)dispose=typeof cleanup==='function'?cleanup:null;else if(typeof cleanup==='function')cleanup();}catch(error){if(alive&&version===epoch){el.innerHTML=`<div class="statebox"><h2>${esc(errorMessage(error))}</h2><button class="btn" data-retry>重试</button><a class="btn" href="#/c/${ctx.charId}/worlds">世界与地点管理</a></div>`;el.querySelector('[data-retry]').onclick=load;}}};void load();return()=>{alive=false;epoch++;dispose?.();};};}

export const renderPlace=worldPage(renderPlacePage);
export const renderScene=worldPage(renderScenePage);
export const renderPeopleList=worldPage(renderPeopleListPage);
export const renderNpc=worldPage(renderNpcPage);
export const renderRelEdit=worldPage(renderRelEditPage);
export const renderWorldSettings=worldPage((el,ctx,alive)=>mountWorldEventRuntime(el,ctx,{alive}));
