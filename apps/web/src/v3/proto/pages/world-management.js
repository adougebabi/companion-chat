import {errorMessage} from '../../domain-presentation.mjs';
import {WorldEditorTree} from '../components/world-venues.js';
import {mountRuntimeWorldFramework} from '../components/runtime-world-framework.js';
import {placeDefaultsDrawer} from '../components/place-defaults.js';
import * as store from '../store.js';
import { esc, badge, toast, confirm, requireOwner } from '../ui.js';
import { Field, Button, KeyValueList } from '../components/forms.js';
import { Card, EmptyState } from '../components/cards.js';
import { ConflictPanel } from '../components/revisions.js';
import { mockWorld, sceneOptions } from '../services.js';

async function renderWorldDirectoryPage(el, ctx) {
  const worlds = await mockWorld.worlds(ctx.charId);
  const character = store.get('characters', ctx.charId);
  if (!character) { el.innerHTML = EmptyState({ title: '未找到角色' }); return; }
  el.innerHTML = `<div class="row" style="margin-bottom:12px"><a class="btn" href="#/c/${ctx.charId}/world">返回世界总览</a><a class="btn" href="#/c/${ctx.charId}/people/npcs">NPC 管理</a></div><div class="grid g2">${worlds.map(world => Card({ title: world.name, body: `<p class="small muted">${world.active ? '当前运行世界' : '尚未使用'}</p><div class="row"><a class="btn" href="#/c/${ctx.charId}/worlds/${world.id}/edit">编辑世界与地点</a>${!world.active ? Button({ label: '切换到此世界', attrs: { 'data-select-world': world.id } }) : ''}${Button({ label: '移除', tone: 'ghost danger', attrs: { 'data-delete-world': world.id } })}</div>` })).join('')}${Card({ title: '新建世界', body: `<form class="stack" data-new-world>${Field({ label: '世界名称', name: 'worldName', attrs: { required: true } })}<button class="btn primary">创建空白世界</button><p class="small muted">创建不改变当前世界。切换要另行确认。</p></form>` })}</div>`;
  el.querySelector('[data-new-world]').onsubmit = async event => { event.preventDefault(); const name = event.target.worldName.value; if (!(await requireOwner()) || !(await confirm({ title: '创建世界', text: `创建 ${esc(name)}，尚不切换当前运行世界。` }))) return; try { const id = await mockWorld.create(ctx.charId, name); ctx.navigate(`#/c/${ctx.charId}/worlds/${id}/edit`); } catch (error) { toast(errorMessage(error), 'err'); } };
  el.querySelectorAll('[data-select-world]').forEach(button => button.onclick = async () => { if (!(await requireOwner()) || !(await confirm({ title: '切换运行世界', text: '这是显式世界切换操作。旧世界保留，新世界的角色位置将显示为尚未确认；仅浏览页面不会切换。' }))) return; try { await mockWorld.select(ctx.charId, button.dataset.selectWorld); ctx.refresh(); } catch(error) { toast(errorMessage(error), 'err'); } });
  el.querySelectorAll('[data-delete-world]').forEach(button => button.onclick = async () => { if (!(await requireOwner()) || !(await confirm({ title: '移除世界', type:'danger', okText:'确认移除世界', text: `移除「${esc(worlds.find(w=>w.id===button.dataset.deleteWorld)?.name||'当前世界')}」；有角色、地点或人物引用时会阻止移除。` }))) return; try { await mockWorld.removeWorld(ctx.charId, button.dataset.deleteWorld, worlds.find(w => w.id === button.dataset.deleteWorld).version); ctx.refresh(); } catch (error) { toast(errorMessage(error), 'err'); } });
}
const editorDrafts = new Map();
const treeStates = new Map();
let draftOwner = store.get('session')?.actorId;
store.subscribe(() => { const owner=store.get('session')?.actorId; if(owner!==draftOwner){editorDrafts.clear();treeStates.clear();draftOwner=owner;} });
const clone = value => structuredClone(value);
const same = (a,b) => JSON.stringify(a)===JSON.stringify(b);
const editorHref = (ctx,wid,pid) => `#/c/${encodeURIComponent(ctx.charId)}/worlds/${encodeURIComponent(wid)}/${pid ? `place/${encodeURIComponent(pid)}/edit` : 'edit'}`;
async function renderWorldEditorPage(el,ctx,alive) { return renderEditor(el,ctx,alive,false); }
async function renderPlaceEditorPage(el,ctx,alive) { return renderEditor(el,ctx,alive,true); }
async function renderEditor(el,ctx,alive=()=>true,isPlace=false) {
  const worldId=ctx.params.worldId, owner=store.get('session')?.actorId;
  const live=()=>alive()&&owner===store.get('session')?.actorId;
  let placeId=isPlace ? ctx.params.placeId||null : null;
  const world=await mockWorld.world(ctx.charId,worldId);
  const places=await mockWorld.places?.(ctx.charId,worldId)||store.list('places',p=>p.worldId===worldId);
  let saved=isPlace ? placeId ? await mockWorld.place(ctx.charId,worldId,placeId) : {name:'',venueCategoryId:ctx.query.category||'',scene:'',version:0} : world;
  if(!live())return;
  const key=[owner,ctx.charId,worldId,isPlace?placeId||'new:'+ (ctx.query.category||''):'world'].join(':');
  let state=editorDrafts.get(key);
  if(!state){state={draft:clone(saved),saved:clone(saved),conflict:null,busy:false,error:''};editorDrafts.set(key,state);}
  // An old draft keeps its original CAS baseline until the user explicitly rebases.
  if(state.saved.version!==saved.version&&!same(state.draft,state.saved))state.conflict=clone(saved);
  else if(same(state.draft,state.saved)){state.saved=clone(saved);state.draft=clone(saved);}
  const treeKey=[owner,ctx.charId,worldId].join(':');
  const tree=treeStates.get(treeKey)||{query:'',closed:new Set(),scroll:0};treeStates.set(treeKey,tree);
  let frameworkPanel;
  const rememberView=()=>{const control=document.activeElement;if(control&&el.contains(control)&&control.dataset.editorField)state.focus={field:control.dataset.editorField,start:control.selectionStart,end:control.selectionEnd};};
  const fields=isPlace ? [['name','正式名称（创建后固定）'],['alias','别名'],['floor','楼层'],['desc','场景描述'],['atmosphere','氛围'],['furniture','固定陈设'],['layout','布局描述']] : [['name','世界名称'],['flavor','场景风味'],['background','世界观'],['itemPool','环境物品池']];
  // Compare the editable payload, so absent optional values and empty controls agree.
  const editable=fields.map(([name])=>name).concat(isPlace?['venueCategoryId','scene','connectionPlaceIds','parentPlaceId']:[]);
  const values=value=>Object.fromEntries(editable.map(name=>[name,value[name]??'']));
  const dirty=()=>!same(values(state.draft),values(state.saved));
  const canSave=()=>!state.busy&&store.get('characters',ctx.charId)?.status!=='已退休'&&dirty()
    &&Number.isInteger(state.saved.version)&&state.saved.version>=(isPlace&&!placeId?0:1)
    &&state.draft.version===state.saved.version&&!!String(state.draft.name||'').trim()
    &&!(isPlace&&!placeId&&(world.venueCategories||[]).length&&!state.draft.venueCategoryId);
  const notice=()=>{const n=el.querySelector('[data-save-state]');if(n)n.textContent=state.busy?'正在保存提交的修改…':!String(state.draft.name||'').trim()?'请填写名称':isPlace&&!placeId&&(world.venueCategories||[]).length&&!state.draft.venueCategoryId?'请选择所属场地分类，再创建地点':dirty()?'有尚未保存的修改，切换左侧对象会保留草稿':'当前显示已保存资料';const button=el.querySelector('[data-editor-save]');if(button){button.disabled=!canSave();button.textContent=state.busy?'保存中…':isPlace&&!placeId?'创建地点':'保存修改';}};
  function paintTree(){
    const nav=el.querySelector('[data-world-tree]');nav.innerHTML=WorldEditorTree({categories:world.venueCategories||[],places,query:tree.query,closed:tree.closed,selected:isPlace?placeId||'new':'world',worldHref:editorHref(ctx,worldId),placeHref:p=>editorHref(ctx,worldId,p.id)});
    nav.scrollTop=tree.scroll;nav.onscroll=()=>{tree.scroll=nav.scrollTop;};
    nav.querySelectorAll('[data-tree-category]').forEach(d=>d.ontoggle=()=>{if(d.open)tree.closed.delete(d.dataset.treeCategory);else tree.closed.add(d.dataset.treeCategory);});
  }
  function paint(){
    rememberView();frameworkPanel?.dispose();frameworkPanel=null;
    const draft=state.draft,conflict=state.conflict;
    el.innerHTML=`<div class="world-editor-review stack"><header class="world-review-heading"><div><h2>${isPlace?placeId?'编辑地点':'添加地点':'修改世界'}</h2><p class="small muted">${esc(world.name)} · 浏览和编辑不会移动角色</p></div><a class="btn" href="#/c/${ctx.charId}/world">返回世界页</a></header><div class="world-editor-workspace"><aside class="world-editor-sidebar"><label class="field world-tree-search"><span class="sr-only">搜索场地或地点</span><input class="input" type="search" aria-label="搜索场地或地点" placeholder="搜索场地或地点" value="${esc(tree.query)}" data-tree-search></label><nav data-world-tree aria-label="世界与地点"></nav><a class="btn world-tree-add" href="#/c/${ctx.charId}/worlds/${encodeURIComponent(worldId)}/place/new">＋ 新建一个地点</a></aside><section class="world-editor-content"><form class="world-editor-form" data-editor-form><div class="world-editor-body stack"><div class="row"><h3 class="grow">${esc(isPlace?draft.alias||draft.name||'新地点':draft.name)}</h3>${badge(world.source_resource_id?'资源定义':'自定义','plain')}</div>${conflict?ConflictPanel({baseVersion:draft.version,currentVersion:conflict.version,rows:fields.map(([name,label])=>({field:label,before:conflict[name]||'',after:draft[name]||''}))}):''}<div class="world-editor-fields">${fields.map(([name,label])=>Field({label,name,value:draft[name]||'',type:['background','description','itemPool','desc','furniture','layout'].includes(name)?'textarea':'text',rows:name==='background'?5:3,hint:name==='itemPool'?'场景中可使用的环境物品描述':name==='name'&&isPlace?'正式名称创建后不可修改，请确认后保存；之后可通过别名补充称呼。':'',attrs:{'data-editor-field':name,required:name==='name',readonly:isPlace&&name==='name'&&!!placeId}})).join('')}</div>${isPlace?Field({label:'所属场地分类',name:'venueCategoryId',value:draft.venueCategoryId||'',type:'select',options:[{value:'',label:'尚未归类'},...(world.venueCategories||[]).map(c=>({value:c.id,label:c.name}))],attrs:{'data-editor-field':'venueCategoryId',disabled:!!placeId,required:!placeId&&(world.venueCategories||[]).length>0}}):''}${isPlace?`<details class="mgmt"><summary>场景素材与已有连接</summary><div class="stack">${Field({label:'场景素材',name:'scene',value:draft.scene||'',type:'select',options:[{value:'',label:'尚未配置场景图片'},...sceneOptions.filter(s=>s.value).map(s=>({...s,label:({cafe:'咖啡馆',oldstreet:'老街',flowershop:'花店',seaside:'海边',apartment:'公寓',shop:'商店'})[s.label]||s.label})),...(draft.scene&&!sceneOptions.some(s=>s.value===draft.scene)?[{value:draft.scene,label:'当前场景图片'}]:[])],attrs:{'data-editor-field':'scene'}})}${Field({label:'父地点',name:'parentPlaceId',value:draft.parentPlaceId||'',type:'select',options:[{value:'',label:'无父地点'},...places.filter(p=>p.id!==placeId).map(p=>({value:p.id,label:p.alias||p.name}))],attrs:{'data-editor-field':'parentPlaceId'}})}<fieldset><legend>连通地点</legend>${places.filter(p=>p.id!==placeId).map(p=>`<label class="toggle"><input type="checkbox" data-connected-place="${esc(p.id)}" ${(draft.connectionPlaceIds||[]).includes(p.id)?'checked':''}> ${esc(p.alias||p.name)}</label>`).join('')||'<p class="small muted">这个世界尚无其他地点可选择。</p>'}</fieldset>${KeyValueList([['连通说明',esc(typeof draft.connections==='string'?draft.connections:JSON.stringify(draft.connections||[]))],['原有父地点',esc(places.find(p=>p.id===draft.parentPlaceId)?.name||'无')]])}</div></details>`:''}<p role="alert" data-editor-error>${esc(state.error)}</p></div><footer class="world-editor-footer"><p class="small muted" data-save-state></p><div class="row">${isPlace&&placeId?'<button type="button" class="btn ghost" data-defaults>默认字段与恢复</button><button type="button" class="btn ghost danger" data-remove-place>移除地点</button>':''}<button type="button" class="btn" data-revert>撤销更改</button><a class="btn" href="#/c/${ctx.charId}/world">取消并返回</a><button class="btn primary" data-editor-save ${canSave()?'':'disabled'}>${state.busy?'保存中…':isPlace&&!placeId?'创建地点':'保存修改'}</button></div></footer></form>${!isPlace?'<details class="world-framework-tools"><summary>世界框架与创作助手</summary><div data-world-framework></div></details>':''}</section></div></div>`;
    paintTree();notice();
    el.querySelectorAll('[data-connected-place]').forEach(input=>input.onchange=()=>{const ids=new Set(state.draft.connectionPlaceIds||[]);input.checked?ids.add(input.dataset.connectedPlace):ids.delete(input.dataset.connectedPlace);state.draft.connectionPlaceIds=[...ids];notice();});
    el.querySelector('[data-tree-search]').oninput=e=>{tree.query=e.target.value;paintTree();};
    el.querySelectorAll('[data-editor-field]').forEach(input=>input.oninput=input.onchange=()=>{state.draft[input.dataset.editorField]=input.value;state.focus={field:input.dataset.editorField,start:input.selectionStart,end:input.selectionEnd};notice();});
    el.querySelector('[data-editor-form]').addEventListener('focusout',event=>{const input=event.target;if(input.dataset.editorField)state.focus={field:input.dataset.editorField,start:input.selectionStart,end:input.selectionEnd};});
    el.querySelector('[data-revert]').onclick=()=>{if(state.busy)return;state.draft=clone(state.conflict||state.saved);state.saved=clone(state.draft);state.conflict=null;state.error='';paint();};
    el.querySelector('[data-keep]')?.addEventListener('click',()=>toast('草稿已保留'));
    el.querySelector('[data-refresh]')?.addEventListener('click',()=>{state.draft.version=state.conflict.version;state.saved=clone(state.conflict);state.conflict=null;state.error='';paint();});
    el.querySelector('[data-editor-form]').onsubmit=async event=>{
      event.preventDefault();if(!canSave()||!event.target.reportValidity()||!(await requireOwner())||!live()||!canSave())return;
      const submitted=clone(state.draft);state.busy=true;state.error='';el.querySelector('[data-editor-save]').disabled=true;notice();
      try{
        let id=placeId;
        if(isPlace){const {id:ignored,version,...values}=submitted;id=await mockWorld.savePlace(ctx.charId,worldId,placeId,values,version);}
        else await mockWorld.save(ctx.charId,worldId,{name:submitted.name,flavor:submitted.flavor,background:submitted.background,itemPool:submitted.itemPool},submitted.version);
        const readback=isPlace?await mockWorld.place(ctx.charId,worldId,id):await mockWorld.world(ctx.charId,worldId);
        if(owner!==store.get('session')?.actorId)return;
        // A receipt acknowledges only the submitted values, never later typing.
        const next=clone(readback);for(const [name,value]of Object.entries(state.draft))if(name!=='version'&&!same(value,submitted[name]))next[name]=clone(value);
        state.saved=clone(readback);state.draft=next;state.conflict=null;
        if(isPlace&&!placeId){editorDrafts.delete(key);editorDrafts.set([owner,ctx.charId,worldId,id].join(':'),state);placeId=id;if(live()){toast('地点已创建并保存','ok');ctx.navigate(editorHref(ctx,worldId,id));}return;}
        if(live()){paint();toast('已保存并读取最新资料','ok');}
      }catch(error){state.error=errorMessage(error);if(error.status===409){try{state.conflict=isPlace?await mockWorld.place(ctx.charId,worldId,placeId):await mockWorld.world(ctx.charId,worldId);}catch(readError){state.error=errorMessage(readError);}}if(live())paint();}
      finally{state.busy=false;if(live())notice();}
    };
    el.querySelector('[data-remove-place]')?.addEventListener('click',async()=>{if(state.busy||!(await requireOwner())||!(await confirm({title:'移除地点',type:'danger',okText:'确认移除地点',text:`移除「${esc(state.draft.alias||state.draft.name)}」；仅移除没有角色、人物、日程或历史引用的地点。`}))||!live())return;try{await mockWorld.removePlace(ctx.charId,worldId,placeId,state.draft.version);editorDrafts.delete(key);if(live())ctx.navigate(editorHref(ctx,worldId));}catch(error){if(live())el.querySelector('[data-editor-error]').textContent=errorMessage(error);}});
    el.querySelector('[data-defaults]')?.addEventListener('click',async()=>{if(state.busy)return;const before=clone(state.draft);const restored=async()=>{const readback=await mockWorld.place(ctx.charId,worldId,placeId);if(!live())return;const next=clone(readback);for(const [name,value]of Object.entries(state.draft))if(name!=='version'&&!same(value,before[name]))next[name]=clone(value);state.saved=clone(readback);state.draft=next;state.conflict=null;paint();};if(!store.isMock){placeDefaultsDrawer(ctx.charId,worldId,placeId,{onRestored:restored});return;}if(!(await requireOwner())||!(await confirm({title:'恢复地点默认字段',text:'恢复将替换当前未保存的地点字段。'})))return;try{await mockWorld.restorePlace(ctx.charId,worldId,placeId);await restored();}catch(error){if(live())el.querySelector('[data-editor-error]').textContent=errorMessage(error);}});
    if(!isPlace&&!store.isMock)frameworkPanel=mountRuntimeWorldFramework(el.querySelector('[data-world-framework]'),{fid:ctx.charId,world:state.saved,alive:live,onApplied:()=>{if(live())ctx.refresh();}});
    if(store.get('characters',ctx.charId)?.status==='已退休')el.querySelectorAll('button,input,select,textarea').forEach(c=>c.disabled=true);
  }
  paint();
  if(state.focus)requestAnimationFrame(()=>{if(!live())return;const input=[...el.querySelectorAll('[data-editor-field]')].find(c=>c.dataset.editorField===state.focus.field);input?.focus({preventScroll:true});if(input?.setSelectionRange&&['text','textarea'].includes(input.type))input.setSelectionRange(state.focus.start,state.focus.end);});
  return()=>{rememberView();frameworkPanel?.dispose();};
}

function mounted(render) { return (el,ctx) => { let alive=true,dispose; el.innerHTML='<div class="statebox" role="status">正在读取…</div>'; const host=document.createElement('div'); const paint=async()=>{try {dispose?.();dispose=await render(host,ctx,()=>alive); if(alive) { if(store.get('characters',ctx.charId)?.status === '已退休') host.querySelectorAll('button,input,select,textarea').forEach(control=>control.disabled=true); el.replaceChildren(host); }}catch(error){if(alive){el.innerHTML=EmptyState({title:errorMessage(error),actions:'<button class="btn" data-retry>重试</button>'});el.querySelector('[data-retry]').onclick=paint;}}};void paint();return()=>{alive=false;dispose?.();};}; }
export const renderWorldDirectory = mounted(renderWorldDirectoryPage);
export const renderWorldEditor = mounted(renderWorldEditorPage);
export const renderPlaceEditor = mounted(renderPlaceEditorPage);
