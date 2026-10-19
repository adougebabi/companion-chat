import {mockWorld} from '../services.js';
import {get} from '../store.js';
import {drawer,esc} from '../ui.js';
import {Field,DiffRows} from './forms.js';
import {JsonDetails} from './records.js';
import {mountCursorRecords} from './cursor-records.js';
import {durableCommandDrawer} from './durable-command-drawer.js';

export function placeDefaultsDrawer(fid,wid,pid,{onRestored=()=>{}}={}){
 const owner=get('session')?.actorId;let active=true,epoch=0,panel=null;
 const d=drawer({title:'地点默认字段与恢复',body:'<p role="status">正在读取实际基准…</p>',onClose:()=>{active=false;epoch++;panel?.dispose();}});
 const alive=()=>active&&d.el.isConnected&&get('session')?.actorId===owner;
 async function load(source='recorded'){
  const token=++epoch;panel?.dispose();d.setBody('<p role="status">正在读取实际地点、基准和差异…</p>');
  try{const value=await mockWorld.placeDefaults(fid,wid,pid,source);if(!alive()||token!==epoch)return;
   const baseline=value.baseline,sourceNames={initial_saved:'该地点首次保存的字段',published_definition:'精确已应用的地点定义版本',owner_pinned_saved:'你明确冻结的已保存字段'};
   d.setBody(`<p>${esc(value.semantics)}</p><p class="small muted">地点 v${esc(value.place_revision)} · 基准 v${esc(value.default_revision)}</p><div class="row"><button class="btn" data-source="recorded">已记录的恢复基准</button><button class="btn" data-source="current_saved">预览当前已保存字段作为基准</button><button class="btn" data-source="current_definition">预览当前已应用定义作为基准</button></div>${baseline?`<h3>${esc(sourceNames[baseline.source_kind]||baseline.source_kind)}</h3>${JsonDetails({kind:baseline.source_kind,resource_id:baseline.source_resource_id,source_version:baseline.source_version,created_at:baseline.created_at,revision:baseline.revision},'实际来源与版本')}${DiffRows(value.diff)}${JsonDetails(baseline.definition,'完整默认字段')}<button class="btn ${source==='recorded'?'danger':'primary'}" data-command>${source==='recorded'?'确认恢复这些字段':'明确冻结此快照为恢复基准'}</button>`:'<p class="banner info">这个旧地点没有可验证的默认快照。请选择当前已保存字段，或存在的已应用定义版本，明确设置基准。不会把当前状态冒充历史初始值，也不会加载演示数据。</p>'}<h3>实际字段修订与恢复历史</h3><div data-history></div><p role="alert" data-error></p>`);
   d.el.querySelectorAll('[data-source]').forEach(button=>button.onclick=()=>void load(button.dataset.source));
   d.el.querySelector('[data-command]')?.addEventListener('click',()=>{
    const restoring=source==='recorded',body={expectedRevision:value.place_revision,expectedDefaultRevision:value.default_revision,operation:restoring?'restore':'capture',source};
    durableCommandDrawer({slot:`place-default:${fid}:${wid}:${pid}`,title:restoring?'恢复地点默认字段':'设定地点恢复基准',body:`<p>${esc(restoring?'恢复将保存这些文本与场景字段，并以实际读回结果替换地点编辑页的未保存草稿；不会移动角色或更改引用身份。':'只记录已核对的恢复基准，不改变地点当前字段，也不保存编辑页尚未提交的草稿。')}</p>${DiffRows(value.diff)}${Field({label:'操作理由',name:'reason',type:'textarea',attrs:{required:true,maxlength:1000}})}`,read:box=>({...body,reason:box.querySelector('[name=reason]').value}),execute:(command,key)=>mockWorld.restorePlace(fid,wid,pid,{...command,idempotencyKey:key}),onSaved:async result=>{if(!alive())return;if(!result){await load();return;}if(result.status==='baseline_saved'){await load();return;}await mockWorld.world(fid,wid);if(!alive())return;d.close();await onRestored(result);}});
   });
   panel=mountCursorRecords(d.el.querySelector('[data-history]'),{alive,label:'地点历史',read:cursor=>mockWorld.placeDefaultHistory(fid,wid,pid,cursor),render:row=>`<article class="lrow"><div class="grow"><b>${esc(row.operation)} · 地点 v${esc(row.place_revision)} · 基准 v${esc(row.default_revision)}</b><p>${esc(row.reason)} · ${esc(row.created_at)}</p>${JsonDetails(row,'字段前后值、基准来源与审计')}</div></article>`,empty:'没有保留的字段或默认基准历史。没有历史时不会补造初始快照。'});
  }catch(error){if(!alive()||token!==epoch)return;d.setBody(`<p role="alert">${esc(error.message)}</p><button class="btn" data-retry>重新读取</button><button class="btn" data-current>核对当前已保存字段</button>`);d.el.querySelector('[data-retry]').onclick=()=>void load(source);d.el.querySelector('[data-current]').onclick=()=>void load('current_saved');}
 }
 void load();return d;
}

export function renderPlaceHistory(el,ctx){
 const owner=get('session')?.actorId;let active=true;const alive=()=>active&&el.isConnected&&get('session')?.actorId===owner;
 el.innerHTML=`<div class="row"><a class="btn" href="#/data/revisions">对象修订目录</a><a class="btn" href="#/c/${encodeURIComponent(ctx.charId)}/worlds">当前世界目录</a></div><div class="card"><h2>地点字段与默认基准历史</h2><p class="small muted">地点 ${esc(ctx.params.placeId)} · 原世界 ${esc(ctx.params.worldId)}</p><p>这里展示保留的实际修订，已移除地点的记录也可读取。历史字段不代表角色当前位置或当前世界状态。</p></div><div data-history></div>`;
 const panel=mountCursorRecords(el.querySelector('[data-history]'),{alive,label:'地点历史',read:cursor=>mockWorld.placeDefaultHistory(ctx.charId,ctx.params.worldId,ctx.params.placeId,cursor),render:row=>`<article class="card"><b>${esc(row.operation)} · 地点 v${esc(row.place_revision)} · 基准 v${esc(row.default_revision)}</b><p>${esc(row.reason)} · ${esc(row.created_at)}</p>${JsonDetails(row,'字段前后值与实际来源')}</article>`,empty:'没有保留的地点字段或恢复基准历史。'});return()=>{active=false;panel.dispose();};
}
