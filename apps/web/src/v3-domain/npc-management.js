import {errorMessage} from '../v3/domain-presentation.mjs';
import {Avatar,bindAvatars} from '../v3/proto/components/avatar.js';
import {JsonDetails} from '../v3/proto/components/records.js';
import { randomId } from '../random-id.mjs';
import {mountResourceLifecycle} from '../v3/proto/components/resource-lifecycle.js';
import {persistentCommand} from '../v3/providers/persistent-command.mjs';
import {mountStudioJobs} from '../v3/proto/components/studio-jobs.js';
import {readStudioDraft,writeStudioDraft,studioWorkingDraftKey} from '../v3/proto/components/studio-working-draft.js';
import {studioDraftStorage} from '../v3/proto/components/studio-draft-storage.js';
import * as store from '../v3/proto/store.js';
import { esc, icon, badge, toast, confirm, requireOwner, drawer, modal, objLink, nameOf } from '../v3/proto/ui.js';
import { Button, Field, SearchForm, DiffView } from '../v3/proto/components/forms.js';
import { Card, EmptyState, EntityHeader } from '../v3/proto/components/cards.js';
import { RoutineEditor, PositionSummary } from '../v3/proto/components/routine.js';
import { ConflictPanel } from '../v3/proto/components/revisions.js';
import { mockClient, mockWorld } from '../v3/proto/services.js';

const drafts = new Map();
store.subscribe(event => { if (event.type === 'reset') drafts.clear(); });
const placeOptions = characterId => store.list('places', place => place.worldId === store.get('characters', characterId).worldId);

export function renderNPCManagement(el, ctx) {
  let alive = true;
  async function paint() {
    const people = await mockClient.npcs(ctx.charId, { query: ctx.query.q || '', archived: ctx.query.archived === '1' }); if (!alive) return;
    el.innerHTML = `${SearchForm({ value: ctx.query.q || '', placeholder: '搜索当前运行世界的姓名或身份', attrs: { 'data-npc-search': true } })}<div class="row" style="margin:12px 0"><a class="btn primary" href="#/c/${ctx.charId}/people/npc/new">${icon('plus')} 创建人物定义</a><a class="btn" href="#/c/${ctx.charId}/people/npcs?archived=${ctx.query.archived === '1' ? '0' : '1'}">${ctx.query.archived === '1' ? '只看启用人物' : '包含已归档人物'}</a><a class="btn ghost" href="#/c/${ctx.charId}/people/list">返回人物目录</a></div>
      <div class="card">${people.length ? people.map(npc => `<div class="lrow npc-management-row">${Avatar({src:npc.avatar,name:npc.name,size:'sm'})}<div class="grow"><b>${esc(npc.name)}</b><div class="small muted">${esc(npc.role)} · ${esc(nameOf(npc.placeId) || '初始地点未设定')}</div></div><div class="row npc-management-actions">${badge(npc.archived ? '已归档' : npc.enabled === false ? '不出现' : '启用', 'plain')}<a class="btn sm" href="#/c/${ctx.charId}/people/npc/${npc.id}">详情</a><a class="btn sm" href="#/c/${ctx.charId}/people/npc/${npc.id}/edit">编辑</a></div></div>`).join('') : EmptyState({ title: ctx.query.q ? '没有匹配的人物' : '尚未添加世界人物', text: ctx.query.q ? '试试其他姓名或身份，或清除筛选查看全部人物。' : '先创建人物定义，再明确应用到当前世界。浏览不会建立关系。', actions: ctx.query.q ? `<a class="btn" href="#/c/${ctx.charId}/people/npcs">清除筛选</a>` : `<a class="btn primary" href="#/c/${ctx.charId}/people/npc/new">创建人物定义</a>` })}</div>
      <div class="banner info small" style="margin-top:12px"> 创建定义、应用到运行世界、已经结识是不同状态。管理目录不会偷偷生成关系或交往记录。</div>`;
    bindAvatars(el);
    el.querySelector('[data-npc-search]').onsubmit = event => { event.preventDefault(); ctx.navigate(`#/c/${ctx.charId}/people/npcs?q=${encodeURIComponent(event.target.q.value)}&archived=${ctx.query.archived || '0'}`); };
  }
  void paint().catch(error => { if (alive) { el.innerHTML = EmptyState({ title: errorMessage(error), actions: '<button class="btn" data-retry>重试</button>' }); el.querySelector('[data-retry]').onclick = () => ctx.refresh(); } });
  return () => { alive = false; };
}

async function renderNPCDefinitionPage(el, ctx) {
  let alive = true, panel=null,lifecyclePanel=null,busy=false,definitionError='', resourceId = ctx.params.resId || null;
  const actor=store.get('session')?.actorId, live=()=>alive&&store.get('session')?.actorId===actor;
  const key = resourceId || `new:${ctx.charId}`;
  let resource = resourceId ? await mockClient.resource(resourceId) : null;
  if(resource && resource.kind !== 'npc') throw new Error('该资源不是 NPC 定义');
  const existing = resource;
  if (resourceId && (!existing || existing.type !== 'NPC')) { el.innerHTML = EmptyState({ title: '未找到 NPC 定义', actions: '<a class="btn" href="#/studio">创作中心</a>' }); return; }
  const draft=readStudioDraft(actor,resourceId||key,resource,{kind:'npc',name:ctx.query.name||''});
  Object.defineProperties(draft,{role:{get:()=>draft.body.role||'',set:value=>{draft.body.role=value;}},bio:{get:()=>draft.body.background||'',set:value=>{draft.body.background=value;}},draftVersion:{get:()=>draft.expectedVersion,set:value=>{draft.expectedVersion=value;}}});
  draft.charId??=ctx.charId;draft.placeId??='';
  draft.dirty=!!resource&&(draft.name!==resource.name||JSON.stringify(draft.body)!==JSON.stringify(resource.body||{}));
  const definitionDiff=()=>DiffView([{field:'名称',before:resource?.name||'',after:draft.name},{field:'简介',before:resource?.body?.role||'',after:draft.role},{field:'背景',before:resource?.body?.background||'',after:draft.bio}]);
  const persist=()=>{draft.dirty=!resource||draft.name!==resource.name||JSON.stringify(draft.body)!==JSON.stringify(resource.body||{});if(live()){writeStudioDraft(actor,resourceId||key,draft);panel?.updateDraft();}};
  const submitted=persistentCommand({key:`fluctlight-studio:${actor}:${key}:npc-command`,storage:studioDraftStorage,execute:(value,idempotencyKey)=>{
   if(!live())throw new Error('Owner或资源范围已切换，请返回原范围核对原提交');
   if(value.operation==='publish')return mockClient.publishNPCDraft(value.id,value.revision,idempotencyKey);
   const fields={...value.fields,idempotencyKey};return value.id?mockClient.saveNPCDraft(value.id,fields):mockClient.createNPCDraft(ctx.charId,fields);
  }});
  async function submitDefinition(value,retry=false){
   if(busy||!(await requireOwner())||!live()||busy)return;busy=true;definitionError='';paint();
   try{const saved=retry?await submitted.retry():await submitted.submit(value);if(!live())return;
    if(value.operation==='save'){
     const wasNew=!resourceId;resourceId=saved.id;resource=saved;draft.expectedVersion=saved.draftVersion;draft.dirty=draft.name!==saved.name||JSON.stringify(draft.body)!==JSON.stringify(saved.body||{});persist();
     if(wasNew){studioDraftStorage().removeItem(studioWorkingDraftKey(actor,key));ctx.navigate(`#/studio/npc/${encodeURIComponent(resourceId)}`);return;}
     resource=await mockClient.resource(resourceId);if(!live())return;persist();toast('NPC草稿已保存并读回，尚未发布','ok');
    }else{resource=await mockClient.resource(value.id);if(!live())return;toast('NPC定义已发布，尚未应用','ok');}
   }catch(error){if(live())definitionError=errorMessage(error);}
   finally{busy=false;if(live())paint();}
  }

  if (store.get('characters', draft.charId)?.status === '已退休' || !store.get('characters', draft.charId)?.worldId) { draft.charId = store.list('characters',c=>c.status!=='已退休' && c.worldId)[0]?.id || ''; draft.placeId = ''; }
  function paint() {
    if (!live()) return; panel?.dispose();lifecyclePanel?.dispose(); const pending=submitted.pending();const published = resource?.versions[resource.published];
    el.innerHTML = `<div class="studio"><div class="stack"><div class="card"><div class="row"><span class="typeicon">${icon('people')}</span><div class="grow"><h2 style="margin:0">${esc(resource?.name || '新建 NPC 定义')}</h2><div class="small muted">NPC · ${resource?.published ? `已发布 ${resource.published}` : '尚未发布'}<span data-npc-dirty>${draft.dirty ? ' · 有未保存修改' : ' · 与保存版本一致'}</span></div></div>${badge('全局定义')}</div><div class="banner info small" style="margin-top:8px">定义不等于完整摇光。发布不会自动把人物放进世界，也不会自动结识。</div></div>
      <form class="card stack" data-definition novalidate>${Field({ label: '名称', name: 'name', value: draft.name, attrs: { required: true, maxlength: 200, 'data-draft': 'name' } })}${Field({ label: '身份 / 简介', name: 'role', value: draft.role, attrs: { maxlength: 512, 'data-draft': 'role' } })}${Field({ label: '背景', name: 'bio', value: draft.bio, type: 'textarea', attrs: { maxlength: 8000, 'data-draft': 'bio' } })}<div class="btngroup"><button class="btn" data-test="npc-save-draft">保存草稿</button>${Button({ label: '发布定义', tone: 'primary', attrs: { 'data-publish-npc': true, disabled: !resource?.draft || draft.dirty || resource?.published===resource?.draftVersion } })}${Button({ label: '放弃修改', tone: 'danger', attrs: { 'data-discard-npc': true } })}</div><p data-definition-error role="alert">${esc(definitionError)}</p>${pending?`<p class="banner info">原提交身份与输入已保留。${pending.rejection?'已明确版本拒绝，请核对后重新采用基线。':'请先重试原提交，等待期间新草稿保持。'}</p><button type="button" class="btn" data-replay-definition ${busy||pending.rejection?'disabled':''}>重试原提交</button>`:''}${resourceId?Button({label:'重新读取版本（保留本地草稿）',attrs:{'data-rebase-definition':true}}):''}</form>
      <div class="card"><h3>已保存版本 → 当前输入</h3><div data-npc-diff>${definitionDiff()}</div></div></div>
      <div class="stack"><div data-studio-jobs></div><div class="card"><h3>应用到运行世界</h3><p class="small muted">只应用已经发布的版本。每个运行世界创建独立人物，关系与记忆不会因应用而产生。</p>${Field({ label: '目标角色 / 世界', name: 'charId', value: draft.charId, type: 'select', options: [{value:'',label:'请选择有运行世界的角色'},...store.list('characters', character => character.status !== '已退休' && character.worldId).map(character => ({ value: character.id, label: `${character.name} · ${store.get('worlds', character.worldId)?.name || '无世界'}` }))], attrs: { 'data-target-char': true } })}${Field({ label: '初始地点', name: 'placeId', value: draft.placeId, type: 'select', options: [{ value: '', label: '未设定' }, ...(draft.charId?placeOptions(draft.charId):[]).map(place => ({ value: place.id, label: place.name }))], attrs: { 'data-target-place': true } })}<a class="btn ghost" href="#/c/${ctx.charId}/worlds">配置运行世界</a><div class="row" style="margin-top:12px">${Button({ label: '确认应用到世界', tone: 'primary', attrs: { 'data-apply-npc': true, disabled: !resource?.published || !draft.charId } })}</div></div>
      <div class="card"><div data-resource-lifecycle></div><h3>版本与边界</h3>${resourceId?`<a class="btn" href="#/studio/preview?res=${encodeURIComponent(resourceId)}">同一份草稿的文本预览沙盒</a>`:''}<p>草稿：${resource?.draft ? '已保存，尚未发布' : '无草稿'}；已发布：${esc(resource?.published || '无')}。</p><p class="small muted">人物生活规律和位置模式在应用后的独立人物详情中编辑。</p><a class="btn" href="#/c/${ctx.charId}/people/npcs">NPC 管理</a><a class="btn ghost" href="#/studio">创作中心</a></div></div></div>`;
    panel=mountStudioJobs(el.querySelector('[data-studio-jobs]'),{actor,resource,alive:live,readDraft:()=>({name:draft.name,body:draft.body}),adopt:value=>{draft.name=value.name;draft.body=value.body;draft.dirty=true;persist();paint();},jobId:ctx.query.job||'',onSelect:id=>{ctx.query.job=id;},linkBase:resourceId?`#/studio/npc/${encodeURIComponent(resourceId)}`:''});
    if(!store.isMock&&resourceId)lifecyclePanel=mountResourceLifecycle(el.querySelector('[data-resource-lifecycle]'),{actor,resource,alive:live,navigate:ctx.navigate,onChanged:()=>ctx.refresh()});
    el.querySelector('[data-replay-definition]')?.addEventListener('click',()=>void submitDefinition(pending.value,true));
    el.querySelector('[data-definition] button[type=submit], [data-test=npc-save-draft]').disabled=busy;
    el.querySelector('[data-publish-npc]').disabled=busy||!resource?.draft||draft.dirty||resource?.published===resource?.draftVersion;
    el.querySelectorAll('[data-draft]').forEach(input => input.oninput = () => { draft[input.dataset.draft] = input.value; persist(); el.querySelector('[data-publish-npc]').disabled=busy||!resource?.draft||draft.dirty||resource?.published===resource?.draftVersion;el.querySelector('[data-npc-dirty]').textContent=draft.dirty?' · 有未保存修改':' · 与保存版本一致';el.querySelector('[data-npc-diff]').innerHTML=definitionDiff(); });
    el.querySelector('[data-definition]').onsubmit=event=>{event.preventDefault();if(!draft.name.trim()){el.querySelector('[data-definition-error]').textContent='请填写NPC名称，不能只输入空格';event.target.elements.name.focus();return;}if(!event.target.reportValidity())return;void submitDefinition({operation:'save',id:resourceId,fields:{name:draft.name,role:draft.role,bio:draft.bio,body:structuredClone(draft.body),draftVersion:draft.expectedVersion}});};
    el.querySelector('[data-rebase-definition]')?.addEventListener('click',async()=>{
     if(!resourceId||busy)return;
     try{const pending=submitted.pending();if(pending&&!pending.rejection)throw new Error('原提交结果尚未确认，请先重试原提交；草稿继续保留。');
      const latest=await mockClient.resource(resourceId);if(!live())return;
      if(!(await confirm({title:'采用最新NPC定义版本',text:'请比较当前保存定义与工作草稿。采用新版本基线后仍保留草稿字段，下一次保存是新的提交。'}))||!live())return;
      if(pending)submitted.releaseRejected();resource=latest;draft.expectedVersion=latest.draftVersion;draft.dirty=true;persist();paint();
     }catch(error){if(live())el.querySelector('[data-definition-error]').textContent=errorMessage(error);}
    });
    el.querySelector('[data-publish-npc]').onclick=async()=>{
     if(busy||!resource||draft.dirty)return;const value={operation:'publish',id:resourceId,revision:resource.version};
     if(!(await confirm({title:'发布NPC保存版本',text:'发布当前已保存的版本；运行世界不会自动改变。'}))||!live())return;
     void submitDefinition(value);
    };
    el.querySelector('[data-discard-npc]').onclick = async () => { if(draft.dirty&&!(await confirm({title:'放弃NPC修改',text:'放弃当前未保存输入并恢复保存版本。原提交身份不会改变。'})))return;if(!live())return; studioDraftStorage().removeItem(studioWorkingDraftKey(actor,resourceId||key)); ctx.refresh(); };
    el.querySelector('[data-target-char]').onchange = async event => { draft.charId = event.target.value; draft.placeId = ''; persist();if(!draft.charId){paint();return;} try { await mockWorld.refresh(draft.charId); paint(); } catch(error) { toast(error.message,'err'); } };
    el.querySelector('[data-target-place]').onchange = event => { draft.placeId = event.target.value; persist(); };
    el.querySelector('[data-apply-npc]').onclick = async () => { try { await npcApplyDrawer(resourceId, draft.charId, draft.placeId); } catch(error) { toast(error.message,'err'); } };
    if(resource?.status==='archived'){el.querySelector('[data-definition]').querySelectorAll('input,textarea,select,button').forEach(c=>c.disabled=true);el.querySelector('[data-apply-npc]').disabled=true;}
  }
  if(draft.charId){try{await mockWorld.refresh(draft.charId);}catch(error){definitionError='目标运行世界暂时无法读取；定义草稿仍可编辑和保存。'+errorMessage(error);}}paint(); return () => { alive = false; panel?.dispose();lifecyclePanel?.dispose(); };
}

export function renderNPCEditor(el, ctx) {
  let alive = true; const id = ctx.params.npcId, key = `npc:${ctx.charId}:${id}`;
  async function paint() {
    const npc = await mockClient.npc(ctx.charId, id); if (!alive) return;
    if (!drafts.has(key)) drafts.set(key, { ...structuredClone(npc), base: npc.version, conflict: false });
    const draft = drafts.get(key); await mockWorld.refresh(ctx.charId); if(!alive) return; const places = placeOptions(ctx.charId);
    el.innerHTML = `<div class="grid g2" style="align-items:start"><form class="card stack" data-npc-form>${EntityHeader({ avatar: npc.avatar, title: npc.name, subtitle: '运行人物 · 当前世界的局部资料', status: badge(npc.archived ? '已归档' : '启用', 'plain') })}
      ${draft.conflict ? ConflictPanel({ baseVersion: draft.base, currentVersion: npc.version }) : ''}${Field({ label: '姓名', name: 'name', value: draft.name, attrs: { required: true, 'data-npc-field': 'name' } })}${Field({ label: '身份 / 简介', name: 'role', value: draft.role, attrs: { 'data-npc-field': 'role' } })}${Field({ label: '背景', name: 'bio', value: draft.bio, type: 'textarea', attrs: { 'data-npc-field': 'bio' } })}${Field({ label: '初始 / 默认地点', name: 'placeId', value: draft.placeId, type: 'select', options: [{ value: '', label: '未设定' }, ...places.map(place => ({ value: place.id, label: place.name }))], attrs: { 'data-npc-field': 'placeId' } })}${Field({ label: '位置模式', name: 'positionMode', value: draft.positionMode || '未知', type: 'select', options: ['未知', '按生活规律推算', '固定地点'], attrs: { 'data-npc-field': 'positionMode' } })}
      <label class="toggle"><input name="enabled" type="checkbox" ${draft.enabled !== false ? 'checked' : ''}> 在世界里出现</label><label class="toggle"><input name="archived" type="checkbox" ${draft.archived ? 'checked' : ''}> 归档并保留所有历史引用</label><div class="row"><button class="btn primary">保存人物资料</button>${Button({ label: '放弃草稿', attrs: { 'data-reset-npc': true } })}<a class="btn ghost" href="#/c/${ctx.charId}/people/npc/${id}">返回详情</a></div></form>
      <div class="stack"><div class="card"><h3>生活规律</h3><p class="small muted">生活规律用于预计位置；实际在场和相遇以运行记录为准。</p>${RoutineEditor({ slots: draft.routine || [], places })}</div><div class="card"><h3>位置与来源</h3>${PositionSummary({ npc, placeName: value => nameOf(value) || '未设定' })}<p class="small muted">当前版本 v${npc.version}。创建人物不代表已经认识。</p>${Button({ label: '移除无引用人物', tone: 'danger', attrs: { 'data-remove-npc': true, disabled: npc.references.length > 0, title: npc.references.length ? '人物被关系或记忆引用，请使用归档' : '仅移除没有历史引用的人物' } })}${npc.references.length ? `<p class="small muted">已有 ${npc.references.length} 条历史引用，可以归档并保留记录。</p>${JsonDetails(npc.references,'查看引用来源')}` : ''}</div></div></div>`;
    bindAvatars(el);
    el.querySelectorAll('[data-npc-field]').forEach(input => input.oninput = input.onchange = () => { draft[input.dataset.npcField] = input.value; });
    el.querySelector('[name=enabled]').onchange = event => { draft.enabled = event.target.checked; };
    el.querySelector('[name=archived]').onchange = event => { draft.archived = event.target.checked; };
    el.querySelector('[data-npc-form]').onsubmit = async event => { event.preventDefault(); if (!(await requireOwner())) return; const form = event.target; draft.enabled = form.enabled.checked; draft.archived = form.archived.checked;
      try { const saved = await mockClient.saveNPC(ctx.charId, id, { ...draft, expectedVersion: draft.base }); drafts.set(key, { ...saved, base: saved.version, conflict: false }); await paint(); toast('人物资料已保存（关系未改变）', 'ok'); }
      catch (error) { if (error.status === 409) { draft.conflict = true; await paint(); } else toast(error.message, 'err'); }
    };
    el.querySelector('[data-reset-npc]').onclick = () => { drafts.delete(key); void paint().catch(error=>toast(error.message,'err')); };
    el.querySelector('[data-keep]')?.addEventListener('click', () => toast('草稿已保留'));
    el.querySelector('[data-refresh]')?.addEventListener('click', () => { draft.base = npc.version; draft.conflict = false; void paint().catch(error=>toast(error.message,'err')); });
    el.querySelector('[data-add-slot]').onclick = () => { (draft.routine ||= []).push({ day: '每天', start: '09:00', end: '17:00', placeId: '', activity: '' }); void paint().catch(error=>toast(error.message,'err')); };
    el.querySelectorAll('[data-slot]').forEach(row => { const index = Number(row.dataset.slot); row.querySelectorAll('input,select').forEach(input => input.onchange = input.oninput = () => { const field = input.name.split('_')[0]; draft.routine[index][field === 'place' ? 'placeId' : field] = input.value; }); });
    el.querySelectorAll('[data-remove-slot]').forEach(button => button.onclick = () => { draft.routine.splice(Number(button.dataset.removeSlot), 1); void paint().catch(error=>toast(error.message,'err')); });
    el.querySelector('[data-remove-npc]').onclick = async () => { if (!(await requireOwner())) return; if (!(await confirm({ title: '移除无引用 NPC', text: '仅从当前运行世界移除，不删除定义。' }))) return; try { await mockClient.removeNPC(ctx.charId, id, npc.version); drafts.delete(key); ctx.navigate(`#/c/${ctx.charId}/people/npcs`); } catch (error) { toast(error.message, 'err'); } };
  }
  void paint().catch(error => { if (alive) { el.innerHTML = EmptyState({ title: error.message, actions: '<button class="btn" data-retry>重试</button>' }); el.querySelector('[data-retry]').onclick = () => ctx.refresh(); } }); return () => { alive = false; };
}

export async function npcApplyDrawer(resourceId, characterId, placeId = '') {
  if (!characterId) throw new Error('请选择可用的运行角色');
  const {resource,preview} = await mockClient.preview(characterId, resourceId), choices = {}, unchanged=!!preview.runtime_id&&!preview.diff.length;
  const fields = {name:'名称',role:'简介',background:'背景'};
  for(const diff of preview.diff) choices[diff.field]='keep';
  const d = drawer({ title: '应用 NPC 定义', body: `<p>${esc(resource.name)} · ${esc(resource.published)}</p>${DiffView(preview.diff.map(diff=>({field:fields[diff.field] || diff.field,before:String(diff.from || ''),after:String(diff.to || '')})))}${preview.diff.map(diff=>Field({label:(fields[diff.field] || diff.field)+' · 当前运行值',name:diff.field,value:'keep',type:'select',options:[{value:'keep',label:'保留当前值（含局部修正）'},{value:'replace',label:'使用新定义'}],attrs:{'data-choice':diff.field}})).join('')}<p class="small muted">${unchanged?'与当前运行人物没有差异，无需再次应用。':preview.runtime_id ? '升级已有运行人物。' : '将建立独立运行人物。'}关系、经历、地点和生活规律保留；应用不会自动结识。</p><div data-apply-error role="alert"></div>`,footer:Button({label:'确认应用已发布定义',tone:'primary',attrs:{'data-apply-version':true,disabled:unchanged}})});
  d.el.querySelectorAll('[data-choice]').forEach(input=>input.onchange=()=>{choices[input.dataset.choice]=input.value;});
  d.el.querySelector('[data-apply-version]').onclick=async event=>{if(unchanged||!(await requireOwner()))return;event.target.disabled=true;try{const n=await mockClient.applyNPC(characterId,resourceId,placeId,choices);drafts.delete(`npc:${characterId}:${n.id}`);d.close();toast('定义已应用并读回，未自动结识','ok');location.hash=`#/c/${characterId}/people/npc/${n.id}`;}catch(error){d.el.querySelector('[data-apply-error]').textContent=error.message;event.target.disabled=false;}};
  return d;
}
export async function encounterDrawer(characterId, npcId) {
  try {
    const [npc,revision] = await Promise.all([mockClient.npc(characterId,npcId),mockClient.encounterContext(characterId)]);
    await mockWorld.refresh(characterId);const places=placeOptions(characterId), idempotencyKey=randomId();
    const d=modal({title:'记录所有者确认的实际接触',body:`<p class="small muted">仅记录你确认实际发生的、与 ${esc(npc.name)} 的接触。此操作会写入真实事件、关系和记忆；不会将手工历史记录称作自动相遇。</p><form class="stack" data-encounter>${Field({label:'接触地点',name:'placeId',value:'',type:'select',options:[{value:'',label:'选择地点'},...places.map(p=>({value:p.id,label:p.name}))]})}${Field({label:'开始时间',name:'startAt',type:'datetime-local',attrs:{required:true}})}${Field({label:'结束时间',name:'endAt',type:'datetime-local',attrs:{required:true}})}${Field({label:'实际经过',name:'summary',type:'textarea',attrs:{required:true}})}${Field({label:'证据引用（逗号分隔）',name:'evidenceRefs',attrs:{required:true}})}<label class="toggle"><input name="confirmed" type="checkbox"> 我确认此事实际发生，并同意记录到生活历史</label></form><p data-encounter-error role="alert"></p>`,footer:Button({label:'确认记录实际接触',tone:'primary',attrs:{'data-encounter-confirm':true}})});
    d.el.querySelector('[data-encounter-confirm]').onclick=async event=>{const form=d.el.querySelector('[data-encounter]');try{if(!form.reportValidity())return;if(!form.confirmed.checked)throw new Error('请确认接触实际发生');if(!(await requireOwner()))return;event.target.disabled=true;const result=await mockClient.recordEncounter(characterId,npcId,{placeId:form.placeId.value,summary:form.summary.value,startAt:new Date(form.startAt.value).toISOString(),endAt:new Date(form.endAt.value).toISOString(),evidenceRefs:form.evidenceRefs.value.split(',').map(v=>v.trim()).filter(Boolean),expectedLifeContextRevision:revision,idempotencyKey});d.close();toast('实际接触已持久记录，事件／关系／记忆来源可回读','ok');location.hash=`#/c/${characterId}/people/npc/${npcId}`;}catch(error){d.el.querySelector('[data-encounter-error]').textContent=error.message;event.target.disabled=false;}};return d;
  } catch(error) {toast(error.message,'err');}
}
export function renderNPCDefinition(el,ctx) {let alive=true,cleanup;el.innerHTML='<div class="statebox" role="status">正在读取 NPC 定义…</div>';const host=document.createElement('div');el.replaceChildren(host);const load=async()=>{try{cleanup=await renderNPCDefinitionPage(host,ctx);if(alive)el.replaceChildren(host);else cleanup?.();}catch(error){if(alive){el.innerHTML=EmptyState({title:errorMessage(error),actions:'<a class="btn" href="#/studio">返回资源目录</a><button class="btn" data-retry>重新读取</button>'});el.querySelector('[data-retry]').onclick=load;}}};void load();return()=>{alive=false;cleanup?.();};}
