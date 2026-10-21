import {WorldVenueDirectory} from '../components/world-venues.js';
import {openOwnerMediaDrawer} from '../components/media-generation-drawer.js';
import {mountWorldEventRuntime} from '../components/world-event-runtime.js';
import {ActiveWorldEffects} from '../components/world-events.js';
import {mountNPCRuntimeRecords} from '../components/npc-runtime-records.js';
import {PositionSummary} from '../components/routine.js';
import {providerPage} from './provider-page.js';
import { encounterDrawer } from './npc-management.js';
import {Avatar,bindAvatars} from '../components/avatar.js';
import { NPCRow, ImageCard } from '../components/cards.js';
// 世界人物扩展页：世界、地点、场景、事件、随机事件设定、人物、媒体
import * as store from '../store.js';
import { esc, icon, badge, modal, toast, objLink, mgmt, nameOf, confirm, requireOwner } from '../ui.js';
import { genImg } from './chat.js';
import { photoModal } from './wardrobe.js';
import { suggestScheduleDrawer } from './life-extra.js';
import { hm } from './common.js';
import { KeyValueList, Field, Tabs, SearchForm } from '../components/forms.js';
import { belongsToWorld, mockWorld, mockClient } from '../services.js';

const here = ctx => store.get('characters', ctx.charId).locationId;
const inWorld = (ctx, object) => object.worldId ? object.worldId === store.get('characters', ctx.charId)?.worldId : store.get('places', object.placeId)?.worldId === store.get('characters', ctx.charId)?.worldId;
function renderMapPage(el, ctx, alive, view) {
  el.innerHTML = `<div class="banner info small">${icon('globe')} ${esc(store.get('worlds', store.get('characters', ctx.charId).worldId)?.name || '运行世界未配置')} · 你在远程浏览，点开地点不会让角色移动。</div>
  <section class="card" style="margin-top:12px"><h2>${esc(store.get('worlds',store.get('characters',ctx.charId).worldId)?.name||'世界设定')}</h2><p>${esc(store.get('worlds',store.get('characters',ctx.charId).worldId)?.description||store.get('worlds',store.get('characters',ctx.charId).worldId)?.background||'世界背景尚未补充')}</p></section><div style="margin-top:12px" data-test="map">${WorldVenueDirectory({categories:store.get('worlds',store.get('characters',ctx.charId).worldId)?.venueCategories||[],places:store.list('places',p=>inWorld(ctx,p)),renderPlace:p=>ImageCard({as:'a',img:p.scene,attrs:{href:`#/c/${ctx.charId}/world/place/${p.id}`,'data-test':`place-${p.id}`},caption:`<b>${esc(p.alias||p.name)}</b><span class="small muted">${esc(p.name)}${p.floor?' · '+esc(p.floor):''}</span><p class="small muted">${esc(p.desc||'描述尚未补充')}</p>${here(ctx)===p.id?badge('当前位置 · '+(store.get('characters',ctx.charId)?.locationAuthority||'未知'),'plain'):''}<span class="small muted">${store.list('npcs',n=>belongsToWorld(ctx.charId,n)&&!n.archived&&n.enabled!==false&&n.currentPlaceId===p.id).map(n=>esc(n.name)+'（'+(n.presence==='confirmed'?'已确认':'预计')+'）').join('、')}</span>`})})}</div>
  <div class="card" style="margin-top:16px"><h3>${icon('warn')} 最近的世界变化</h3>${store.list('events', e => inWorld(ctx, e)).map(e => `<div class="lrow"><span class="grow">${objLink(e.id)} <span class="small muted">${esc(nameOf(e.placeId))}</span></span><span class="small muted">${e.at}</span>${badge(e.state)}</div>`).join('')||'<p class="small muted">当前范围没有已记录的世界变化。</p>'}</div>
  ${store.isMock?'':ActiveWorldEffects(view?.active_effects,{fid:ctx.charId,zone:store.get('characters',ctx.charId)?.timezone})}<div class="row" style="margin-top:12px"><a class="btn" href="#/c/${ctx.charId}/world/scene">她身边的场景</a><a class="btn" href="#/c/${ctx.charId}/world/events">事件流</a><a class="btn ghost" href="#/c/${ctx.charId}/world/settings">随机事件设定</a><a class="btn" href="#/c/${ctx.charId}/worlds">世界与地点管理</a><a class="btn" href="#/c/${ctx.charId}/people/npcs">NPC 管理</a><a class="btn" href="#/c/${ctx.charId}/world/items">世界物品目录</a></div>`;
}
function renderPlacePage(el, ctx) {
  const p = store.get('places', ctx.params.placeId);
  if (!p || !inWorld(ctx, p)) { el.innerHTML = '<div class="statebox"><h2>当前范围未找到该地点</h2></div>'; return; }
  const isHere = here(ctx) === p.id;
  el.innerHTML = `<section class="hero ${p.scene?'':'no-image'}">${p.scene?`<img class="scene" src="${esc(p.scene)}" alt="">`:''}<div class="over"><div><h2>${esc(p.name)}</h2><p class="small">${esc(p.desc)}</p></div></div></section>
  <div class="banner ${isHere ? 'ok' : 'info'}" style="margin-top:12px" data-test="place-loc">${icon('pin')} ${isHere ? `${esc(store.get('characters', ctx.charId)?.name || '角色')}的当前地点：${esc(store.get('characters', ctx.charId)?.locationAuthority || '未知')}。` : `你在远程看${esc(p.name)}；${esc(store.get('characters', ctx.charId)?.name || '角色')}仍在 <b data-test="char-loc">${esc(nameOf(here(ctx)) || '位置未知')}</b>，浏览不会移动她。`}</div>
  <div class="grid g3" style="margin-top:12px"><div class="card"><h3>这里的人</h3>${store.list('npcs', n => belongsToWorld(ctx.charId, n) && !n.archived && n.enabled !== false && n.currentPlaceId === p.id).map(n => NPCRow(n, objLink) + badge(n.presence === 'confirmed' ? '已确认' : '预计', 'plain')).join('') || '<p class="small muted">当前无已确认或预计在场的 NPC</p>'}</div>
   <div class="card"><h3>她在这里的日程</h3>${store.list('schedule', s => s.placeId === p.id).map(s => `<div class="lrow small">${esc(s.time)} ${esc(s.title)} ${badge(s.state)}</div>`).join('') || '<p class="small muted">当前范围未找到</p>'}</div>
   <div class="card"><h3>发生过</h3>${store.list('events', e => e.placeId === p.id).map(e => `<div class="lrow small">${objLink(e.id)} ${badge(e.state)}</div>`).join('') || '<p class="small muted">当前范围未找到</p>'}</div></div>
  <div class="row" style="margin-top:12px"><button class="btn primary" data-sug data-test="place-suggest">${icon('calendar')} 建议她来这里</button><a class="btn" href="#/c/${ctx.charId}/chat?say=${encodeURIComponent(p.name + '最近怎么样？')}">在对话里问问</a></div>
  ${mgmt(`${p.id}${p.resourceId ? ' · 定义 ' + objLink(p.resourceId, p.resourceId) : ''} · page key world.place.detail`)}`;
  el.querySelector('[data-sug]').disabled = store.get('characters',ctx.charId)?.status === '已退休';
  el.querySelector('[data-sug]').onclick = () => suggestScheduleDrawer(p.id);
}
function renderScenePage(el, ctx, alive, view) {
  const c = store.get('characters', ctx.charId); const p = store.get('places', c.locationId);
  if (!p || p.worldId !== c.worldId) { el.innerHTML = `<div class="statebox"><h2>当前世界的位置尚未确认</h2><p class="muted">可以先查看世界中的地点，或在世界管理中完善地点。浏览不会改变她的位置。</p><div class="row"><a class="btn" href="#/c/${ctx.charId}/world">查看世界</a><a class="btn" href="#/c/${ctx.charId}/worlds">世界与地点管理</a></div></div>`; return; }
  el.innerHTML = `<section class="hero ${p.scene?'':'no-image'}">${p.scene?`<img class="scene" src="${esc(p.scene)}" alt="">`:''}<div class="over">${Avatar({src:c.avatar,name:c.name,size:'lg'})}<div class="grow"><div class="small muted">远程查看 · 你不在场</div><h2 style="margin:2px 0">${esc(p.name)}</h2><p class="small">${esc(c.doing)}</p></div></div></section>
  <div class="grid g2" style="margin-top:12px"><div class="card"><h3>周围</h3>${store.list('npcs', n => belongsToWorld(ctx.charId, n) && !n.archived && n.enabled !== false && n.currentPlaceId === p.id).map(n => NPCRow(n, objLink) + badge(n.presence === 'confirmed' ? '已确认' : '预计', 'plain')).join('') || '<p class="small muted">当前无已确认或预计在场的 NPC</p>'}<p class="small muted">${esc(p.desc)}</p></div>
   <div class="card"><h3>${icon('camera')} 看看她</h3><p class="small muted">生成一张此刻的画面，${badge('生成图·非事实')}，不改变任何事实。没有“进入场景”。</p><button class="btn primary" data-photo data-test="scene-photo">${icon('camera')} 拍一张</button></div></div>`;
  if(!store.isMock)el.insertAdjacentHTML('beforeend',ActiveWorldEffects(view?.current_context?.world_effects,{fid:ctx.charId,zone:c.timezone}));
  el.querySelector('[data-photo]').disabled = c.status === '已退休';
  el.querySelector('[data-photo]').onclick = () => store.isMock?photoModal(null):openOwnerMediaDrawer({fid:ctx.charId,title:'当前场景图片请求',place:{id:p.id,name:p.name,revision:p.revision},prompt:'请画一张你此刻实际场景中的画面，保留当前有效外貌与合法穿着；图片只是画面，不代表任何移动、换装或新事件。'});
}
function renderEventsPage(el, ctx) {
 if(!store.isMock)return providerPage(()=>import('./real-world-events.js'),'renderRealWorldEvents',el,ctx);
  el.innerHTML = `<div class="card chain">${store.list('events', e => inWorld(ctx, e)).sort((a, b) => b.at > a.at ? 1 : -1).map(e => `<a class="lrow click" href="#/c/${ctx.charId}/world/event/${e.id}" data-test="ev-${e.id}"><b class="small" style="width:90px">${e.at}</b><span class="grow">${esc(e.title)}<div class="small muted">${esc(nameOf(e.placeId))} · 影响 ${e.impacts.length} 处</div></span>${badge(e.state)}</a>`).join('')}</div>`;
}
const TYPE = { schedule: '日程', diary: '日记', memory: '记忆', relation: '关系', item: '物品' };
function renderEventPage(el, ctx) {
 if(!store.isMock)return providerPage(()=>import('./real-world-events.js'),'renderRealWorldEvent',el,ctx);
  const e = store.get('events', ctx.params.evId);
  if (!e || !inWorld(ctx, e)) { el.innerHTML = '<div class="statebox"><h2>当前范围未找到该事件</h2></div>'; return; }
  el.innerHTML = `<div class="card"><div class="row"><h2 class="grow" style="margin:0">${esc(e.title)}</h2>${badge(e.state)}</div><p class="small">${e.at} · ${objLink(e.placeId)}</p>${mgmt(`${e.id} · page key world.event.detail`)}</div>
  <div class="card" style="margin-top:12px"><h3>影响链 <span class="small muted">点一项看现在的状态</span></h3><div class="chain">${e.impacts.map((i, k) => { const f = store.find(i.id); const o = f?.obj || {};
    return `<div data-test="impact"><button class="lrow click" style="width:100%;text-align:left;background:none;border:0" data-imp="${k}" data-test="impact-open-${k}">${badge(TYPE[i.type] || i.type, 'plain')}<span class="grow">${esc(i.text)}</span>${icon('more')}</button>
    <div class="card tint-sun small" data-impd="${k}" hidden data-test="impact-detail-${k}">${objLink(i.id)} · 现在：${o.state ? badge(o.state) : o.status ? `<span class="badge st-${o.status}">${o.status}</span>` : o.stage ? badge(o.stage, 'plain') : '已记录'}${o.note ? ' · ' + esc(o.note) : ''}</div></div>`; }).join('')}</div></div>
  <div class="row" style="margin-top:12px"><a class="btn" href="#/c/${ctx.charId}/world/events">全部事件</a><a class="btn" href="#/c/${ctx.charId}/chat?say=${encodeURIComponent('封路那天还好吗？')}">在对话里问问她</a></div>`;
  el.querySelectorAll('[data-imp]').forEach(b => b.onclick = () => { const d = el.querySelector(`[data-impd="${b.dataset.imp}"]`); d.hidden = !d.hidden; });
}
/* world.instance.settings —— 随机事件设定；永久伤害不可开启、不可绕过 */
async function renderWorldSettingsPage(el,ctx,alive=()=>true) {
  if(!store.isMock)return mountWorldEventRuntime(el,ctx,{alive});
  let saved=await mockWorld.templates(ctx.charId);
  const paint=()=>{el.innerHTML=`<div class="grid g2" style="align-items:start"><form class="card stack" data-wf><h3>${icon('sparkle')} 随机事件模板</h3>${saved.items.map(t=>`<label class="toggle"><input type="checkbox" data-template="${esc(t.template_id)}" ${t.enabled?'checked':''} ${t.locked||t.permanent_harm?'disabled':''}> ${esc(t.name)}</label>`).join('') || '<p class="small muted">尚无已配置事件模板。</p>'}<div class="lockrow" data-test="harm-row"><label class="toggle"><input type="checkbox" disabled data-test="harm-toggle"> 永久伤害类事件</label><p class="small">系统强制关闭，保存、发布、导入和执行均受服务端保护。</p></div><p class="small muted">频率与自动执行状态由服务端模板配置决定。</p><a class="btn" href="#/studio">配置事件资源</a><p data-error role="alert"></p></form><div class="card tint-sun"><h3>安全与来源</h3><p class="small muted">模板开关不会立即触发事件；实际执行与拦截结果在事件流和运行诊断查看。</p><a class="btn" href="#/c/${ctx.charId}/world/events">事件流</a><a class="btn" href="#/diag">运行诊断</a></div></div>`;el.querySelectorAll('[data-template]').forEach(input=>input.onchange=async()=>{input.disabled=true;try{if(!(await requireOwner())){paint();return;}saved=await mockWorld.setTemplate(ctx.charId,input.dataset.template,input.checked,saved.revision);paint();toast('模板开关已保存并读回','ok');}catch(error){el.querySelector('[data-error]').textContent=error.message;input.disabled=false;}});};paint();
}
/* 人物 */
function renderGraphPage(el, ctx) {
  el.innerHTML = `<div class="grid g2">${store.list('relations', r => r.charId === ctx.charId).map(r => { const n = store.get('npcs', r.npcId) || store.list('npcs', n => n.actor_id === r.targetActorId)[0]; if(!n || !belongsToWorld(ctx.charId,n)) return ''; return `<a class="card" style="display:block" href="#/c/${ctx.charId}/people/npc/${n.id}" data-test="rel-${r.id}"><div class="row">${Avatar({src:n.avatar,name:n.name,size:'lg'})}<div class="grow"><h3 style="margin:0">${esc(n.name)}</h3><span class="small muted">${esc(n.role)}</span></div>${badge(r.stage, 'plain')}</div>
    <div class="small" style="margin-top:6px">亲密度 ${(r.intimacy?.value ?? '未知')} ${badge('推测')} · 最近：${esc((r.revisions?.at(-1)?.text || r.stage || ''))}</div></a>`; }).join('')||'<div class="statebox"><h2>还没有已记录的人物关系</h2><p class="muted">可以先查看人物目录和实际接触记录。人物出现在目录中，不代表双方已经建立关系。</p></div>'}</div>
  <div class="row" style="margin-top:12px"><a class="btn" href="#/c/${ctx.charId}/people/list">人物目录</a><a class="btn" href="#/c/${ctx.charId}/people/owner">关于你</a></div>`;
}
function renderPeopleListPage(el, ctx) {
  const kw = ctx.query.q || '';
  const ns = store.list('npcs', n => belongsToWorld(ctx.charId, n) && !n.archived && (!kw || n.name.includes(kw) || n.role.includes(kw)));
  el.innerHTML = `${SearchForm({ value: kw, placeholder: '找人（名字或身份）', attrs: { 'data-pf': true, style: 'margin-bottom:12px' }, inputAttrs: { 'data-test': 'people-q' } })}
  <div class="card"><div class="list"><a class="lrow click" href="#/c/${ctx.charId}/people/owner"><img class="av sm" src="/v3/assets/avatars/user.svg" alt=""><b>我</b><span class="grow small muted">所有者</span></a>
   ${ns.map(n => { const r = store.list('relations', x => x.charId === ctx.charId && (x.npcId === n.id || x.targetActorId === n.actor_id))[0]; return `<a class="lrow click" href="#/c/${ctx.charId}/people/npc/${n.id}" data-test="npc-${n.id}">${Avatar({src:n.avatar,name:n.name,size:'sm'})}<b>${esc(n.name)}</b><span class="grow small muted">${esc(n.role)} · ${esc(nameOf(n.placeId))}</span>${r ? badge(r.stage || '已有关系', 'plain') : '<span class="small muted">未建立关系</span>'}</a>`; }).join('')}</div></div>`;
  el.insertAdjacentHTML('beforeend', `<div class="row" style="margin-top:12px"><a class="btn" href="#/c/${ctx.charId}/people/npcs">管理 NPC</a><a class="btn primary" href="#/c/${ctx.charId}/people/npc/new">新建 NPC 定义</a></div>`);
  el.querySelector('[data-pf]').onsubmit = e => { e.preventDefault(); ctx.navigate(`#/c/${ctx.charId}/people/list?q=${encodeURIComponent(e.target.q.value)}`); };
}
async function renderNpcPage(el, ctx,alive=()=>true) {
  const n = await mockClient.npc(ctx.charId, ctx.params.npcId);if(!alive())return;const c=store.get('characters',ctx.charId), context=n.relationship_context || {}, relations=context.relationships || [], history=context.relationship_history || [];
  const refs=n.encounters || [];
  el.innerHTML=`<div class="grid g2" style="align-items:start"><div class="card"><div class="row">${Avatar({src:n.avatar,name:n.name,size:'lg'})}<div class="grow"><h2 style="margin:0">${esc(n.name)}</h2><div class="small muted">${esc(n.role)} · 初始地点 ${esc(nameOf(n.placeId) || '未知')}</div></div></div><p>${esc(n.bio)}</p><div class="banner info small" data-test="npc-not-full">NPC 是独立运行人物，不是完整摇光。生活规律仅用于预计；实际接触必须有事件与证据来源。</div><h3 style="margin-top:12px">与 ${esc(c.name)} 有关的实际接触</h3>${store.isMock?refs.map(e=>`<div class="lrow small"><span class="grow">${esc(e.summary)}<div class="muted">${esc(e.occurred_at)} · ${esc(nameOf(e.place_id) || e.place_id)}</div><div>事件 ${objLink(e.life_event_id)} · 关系 ${esc(e.relationship_id)} · 记忆 ${objLink(e.memory_id)}</div><div>证据：${e.evidence_refs.map(ref=>esc(ref)).join('、')}</div></span></div>`).join('') || '<p class="small muted">尚无接触记录；创建与应用不会自动结识。</p>':'<div data-npc-runtime-records></div>'}</div><div class="card"><h3>与 ${esc(c.name)} 的关系</h3>${relations.map(r=>`<div class="lrow"><span>${esc(r.summary || r.stage || '已有关系记录')} · v${esc(r.revision)}</span><a class="btn sm" href="#/c/${ctx.charId}/people/relation/${esc(r.id)}">修正关系…</a></div>`).join('') || '<p class="small muted">尚无关系记录</p>'}<h3 style="margin-top:12px">关系变化</h3><div class="chain" data-test="rel-revs">${history.map(r=>`<div class="lrow small">v${esc(r.revision)} · ${esc(r.summary || r.reason || '')}<span class="muted">${esc(r.created_at || '')}</span></div>`).join('') || '<p class="small muted">尚无关系修订</p>'}</div><h3 style="margin-top:12px">位置与来源</h3>${PositionSummary({npc:n,placeName:value=>nameOf(value)||value||'未知'})}<p class="small muted">预计位置：${esc(nameOf(n.planned_position?.place_id)||'未知')}。预计不代表实际相遇。</p></div></div><div class="row" style="margin-top:12px"><a class="btn" href="#/c/${ctx.charId}/people/npc/${n.id}/edit">编辑资料与生活规律</a><a class="btn ghost" href="#/c/${ctx.charId}/people/npcs">NPC 管理</a><button class="btn" data-encounter-npc ${c.status==='已退休'?'disabled':''}>记录所有者确认的实际接触</button></div>`;
  el.querySelector('[data-encounter-npc]').onclick=()=>encounterDrawer(ctx.charId,n.id);
  if(!store.isMock){const panel=mountNPCRuntimeRecords(el.querySelector('[data-npc-runtime-records]'),{fid:ctx.charId,npcId:n.id,zone:c.timezone,alive,readOnly:c.status==='已退休',initial:ctx.query.npcTab,onTab:tab=>{const url=new URL(location.href);url.hash=`#/c/${ctx.charId}/people/npc/${encodeURIComponent(n.id)}?npcTab=${tab}`;window.history.replaceState(null,'',url);}});return()=>panel.dispose();}
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
export function renderOwner(el, ctx) {
  const b = store.get('user').background;
  el.innerHTML = `<form class="card" style="max-width:640px" data-of><div class="row"><img class="av lg" src="/v3/assets/avatars/user.svg" alt=""><div class="grow"><h3 style="margin:0">我的背景</h3><p class="small muted">${esc(store.get('characters', ctx.charId)?.name || '角色')}会把这些当成“关于你的事实”。只填你愿意让她知道的。</p></div>${badge('角色范围')}</div>
   <div class="stack">${Object.entries(b).map(([k, v]) => `<label class="field">${k}<input class="input" name="${k}" value="${esc(v)}" data-test="owner-${k}"></label>`).join('')}</div>
   <div class="row" style="margin-top:12px"><button type="reset" class="btn" data-test="owner-cancel">撤销更改</button><button class="btn accent" data-test="owner-save">保存</button></div></form>`;
  const f = el.querySelector('[data-of]');
  f.onsubmit = async e => { e.preventDefault(); if (!(await requireOwner())) return; const nb = Object.fromEntries(Object.keys(b).map(k => [k, f[k].value]));
    store.update(d => { d.user.background = nb; }, { identity: '所有者修改', summary: '更新所有者背景', target: null }); toast('已保存', 'ok'); };
}

function worldPage(render) {return (el,ctx)=>{let alive=true,epoch=0,dispose;el.innerHTML='<div class="statebox" role="status">正在读取世界…</div>';const load=async()=>{if(!alive)return;const version=++epoch;dispose?.();dispose=null;try{const view=await mockWorld.refresh(ctx.charId);if(!alive||version!==epoch)return;const host=document.createElement('div');el.replaceChildren(host);const cleanup=await render(host,ctx,()=>alive&&version===epoch&&host.isConnected,view);bindAvatars(host);host.querySelectorAll('img.scene').forEach(image=>{const failed=()=>{image.closest('.hero')?.classList.add('no-image');image.remove();};image.addEventListener('error',failed,{once:true});if(image.complete&&!image.naturalWidth)failed();});if(alive&&version===epoch)dispose=typeof cleanup==='function'?cleanup:null;else if(typeof cleanup==='function')cleanup();}catch(error){if(alive&&version===epoch){el.innerHTML=`<div class="statebox"><h2>${esc(error.message)}</h2><button class="btn" data-retry>重试</button><a class="btn" href="#/c/${ctx.charId}/worlds">世界与地点管理</a></div>`;el.querySelector('[data-retry]').onclick=load;}}};void load();return()=>{alive=false;epoch++;dispose?.();};};}
export const renderMap = worldPage(renderMapPage);
export const renderPlace = worldPage(renderPlacePage);
export const renderScene = worldPage(renderScenePage);
export const renderEvents = worldPage(renderEventsPage);
export const renderEvent = worldPage(renderEventPage);
export const renderGraph = worldPage(renderGraphPage);
export const renderPeopleList = worldPage(renderPeopleListPage);
export const renderNpc = worldPage(renderNpcPage);
export const renderRelEdit = worldPage(renderRelEditPage);
export const renderWorldSettings = worldPage(renderWorldSettingsPage);
/* 媒体 */
export function renderGallery(el, ctx) {
  if(!store.isMock)return providerPage(()=>import('./real-media.js'),'renderRealMediaLibrary',el,ctx);
  const ms = store.list('media', m => m.charId === 'c1').reverse();
  el.innerHTML = `<div class="banner warn">${badge('生成图·非事实')} 这里的图都是生成画面，不代表发生过。</div><div class="wgrid" data-test="media-grid" style="margin-top:12px">${ms.map(m => `<a class="card" style="padding:8px;display:block" href="#/c/${ctx.charId}/media/${m.id}" data-test="media-${m.id}">${genImg(m)}<div class="small" style="margin-top:6px"><b>${esc(m.title)}</b>${m.itemId ? `<br>物品：${esc(nameOf(m.itemId))}` : ''}${m.outfitSnapshot ? `<br>穿搭：${m.outfitSnapshot.map(i => esc(nameOf(i))).join('、')}` : ''}</div></a>`).join('')}</div>
  <div class="row" style="margin-top:12px"><a class="btn primary" href="#/c/${ctx.charId}/media/generate">${icon('sparkle')} 生成一张</a><a class="btn" href="#/c/${ctx.charId}/media/identity">视觉身份</a></div>`;
}
export function renderMediaDetail(el, ctx) {
  if(!store.isMock)return providerPage(()=>import('./real-media.js'),'renderRealMediaDetail',el,ctx);
  const m = store.get('media', ctx.params.mdId);
  if (!m) { el.innerHTML = `<div class="statebox"><h2>当前范围未找到这张图</h2><a class="btn" href="#/c/${ctx.charId}/media">回媒体库</a></div>`; return; }
  el.innerHTML = `<div class="grid g2" style="align-items:start"><div class="card">${genImg(m)}</div><div class="card"><h2>${esc(m.title)}</h2>${badge('生成图·非事实')}
   ${KeyValueList([['画面地点', m.placeId ? objLink(m.placeId) : '—'], ['物品', m.itemId ? objLink(m.itemId) : '—'], ['穿搭快照', m.outfitSnapshot ? m.outfitSnapshot.map(i => objLink(i)).join('、') : '—'], ['出现在', m.sourceMsg ? objLink(m.sourceMsg, '对话消息') : '媒体库']], { attrs: { style: 'margin-top:8px' } })}
   <p class="small muted">快照只记录生成那一刻的穿搭，不会反过来改变她的穿着。</p><a class="btn primary" href="#/c/${ctx.charId}/media/generate?ref=${m.id}">以此为参考再生成</a>${mgmt(`${m.id} · nonFact=${m.nonFact}`)}</div></div>`;
}
export function renderIdentity(el, ctx) {
 if(!store.isMock)return providerPage(()=>import('./real-visual.js'),'renderRealVisualIdentity',el,ctx);
  const c = store.get('characters', ctx.charId);
  el.innerHTML = `<div class="grid g3"><div class="card" style="text-align:center"><img src="${c.avatar}" alt="" style="width:160px"><h3>标准头像</h3>${badge('全局定义')}</div>
   <div class="card"><h3>外貌要点</h3><ul class="small"><li>齐肩短发、浅棕色</li><li>常穿暖色系、碎花和条纹</li><li>爱戴草编帽</li></ul><p class="small muted">生成图都参考这里，保证她看起来是同一个人。</p></div>
   <div class="card"><h3>参考图</h3><div class="row">${store.list('media', m => m.charId === 'c1').slice(0, 3).map(m => `<img src="${m.img}" alt="" style="width:80px;border-radius:8px">`).join('')}</div><p class="small muted">在视觉定义页编辑参考约束，当前全为 Mock。</p><a class="btn" href="#/c/${ctx.charId}/media/identity/edit">编辑视觉身份</a></div></div>`;
  const definition = store.get('meta').visualIdentity?.[ctx.charId];
  if (definition) el.insertAdjacentHTML('beforeend', `<div class="card" style="margin-top:12px"><h3>已保存的视觉定义 · v${definition.version}</h3>${KeyValueList([['外貌要点', esc(definition.appearance)], ['服装风格', esc(definition.clothing)], ['一致性约束', esc(definition.constraints)], ['参考说明', esc(definition.references)]])}</div>`);
}
export function renderGenerate(el, ctx) {
 if(!store.isMock)return providerPage(()=>import('./real-generate.js'),'renderRealGenerate',el,ctx);
  const c = store.get('characters', 'c1'); const ref = ctx.query.ref && store.get('media', ctx.query.ref);
  const owned = store.list('items', i => i.charId === 'c1' && ['拥有', '使用', '借用'].includes(i.status));
  el.innerHTML = `<div class="grid g2" style="align-items:start"><form class="card" data-gf><h3>${icon('sparkle')} 生成一张图</h3><div class="banner warn small">${badge('生成图·非事实')} 只是画面；不会改变她的位置、穿着或任何事实。</div>
   <div class="stack" style="margin-top:8px"><label class="field">画面描述<input class="input" name="prompt" value="${esc(ref ? ref.title + '（再来一张）' : '小遥在海边画画')}" data-test="gen-prompt"></label>
   <label class="field">画面地点<select class="select" name="place">${store.list('places').map(p => `<option value="${p.id}" ${p.id === (ref?.placeId || 'p_book') ? 'selected' : ''}>${esc(p.name)}</option>`).join('')}</select></label>
   <label class="field">带上物品<select class="select" name="item"><option value="">不带</option>${owned.map(i => `<option value="${i.id}">${esc(i.name)}</option>`).join('')}</select></label></div>
   <div class="row" style="margin-top:12px"><button type="reset" class="btn">重置</button><button class="btn primary" data-test="gen-submit">${icon('sparkle')} 生成</button></div></form>
   <div class="card" data-out><h3>结果</h3><p class="small muted">小遥此刻实际在：${esc(nameOf(c.locationId))}</p><div data-res></div></div></div>`;
  el.querySelector('[data-gf]').onsubmit = e => { e.preventDefault(); const f = e.target; const out = el.querySelector('[data-res]'); const loc0 = c.locationId;
    out.innerHTML = '<div class="skel" style="height:140px"></div><p class="small muted">生成中…</p>';
    setTimeout(() => { const id = store.uid('md_'); const p = store.get('places', f.place.value);
      store.update(d => { d.media[id] = { id, charId: 'c1', img: p.scene, overlay: f.item.value ? d.items[f.item.value].img : null, title: f.prompt.value, nonFact: true, itemId: f.item.value || null, outfitSnapshot: d.outfit.c1.current.slice(), placeId: p.id, at: hm() }; }, { identity: '交流', summary: '媒体生成（生成图·非事实）', target: id });
      const md = store.get('media', id); const box = document.querySelector('[data-res]');
      if (box) box.innerHTML = `${genImg(md)}<p class="small" data-test="gen-page-result">已存入媒体库 · <a href="#/c/c1/media/${id}">查看</a><br>${store.get('characters', 'c1').locationId === loc0 ? '✓ 她的位置没有改变' : ''}</p>`; }, 400); };
}
