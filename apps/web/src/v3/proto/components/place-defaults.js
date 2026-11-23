import {userInstant} from '../../presentation-time.mjs';
import {stateLabel} from '../../domain-presentation.mjs';
import {isInternalId} from '../../internal-id.mjs';
import {mockWorld} from '../services.js';
import {get} from '../store.js';
import {drawer,esc} from '../ui.js';
import {Field,DiffRows} from './forms.js';
import {JsonDetails,RawRecordDownload} from './records.js';
import {ReadingFields} from './reading-fields.js';
import {readingFields} from '../../../approved-ui/reading-fields.mjs';
import {mountCursorRecords} from './cursor-records.js';
import {durableCommandDrawer} from './durable-command-drawer.js';

export function placeDefaultsDrawer(fid,wid,pid,{onRestored=()=>{}}={}){
 const owner=get('session')?.actorId;let active=true,epoch=0,panel=null;
 const d=drawer({title:'地点默认字段与恢复',body:'<p role="status" data-domain-ui="skeleton">正在读取实际基准…</p>',onClose:()=>{active=false;epoch++;panel?.dispose();}});
 const alive=()=>active&&d.el.isConnected&&get('session')?.actorId===owner;
 async function load(source='recorded'){
  const token=++epoch;panel?.dispose();d.setBody('<p role="status" data-domain-ui="skeleton">正在读取实际地点、基准和差异…</p>');
  try{const value=await mockWorld.placeDefaults(fid,wid,pid,source);if(!alive()||token!==epoch)return;
   const baseline=value.baseline,sourceNames={initial_saved:'该地点首次保存的字段',published_definition:'精确已应用的地点定义版本',owner_pinned_saved:'你明确冻结的已保存字段'};
   d.setBody(`<p>${esc(value.semantics)}</p><p class="small muted">地点 v${esc(value.place_revision)} · 基准 v${esc(value.default_revision)}</p><div class="row"><button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-source="recorded">已记录的恢复基准</button><button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-source="current_saved">预览当前已保存字段作为基准</button><button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-source="current_definition">预览当前已应用定义作为基准</button></div>${baseline?`<h3>${esc(sourceNames[baseline.source_kind]||baseline.source_kind)}</h3>${JsonDetails({kind:baseline.source_kind,resource_id:baseline.source_resource_id,source_version:baseline.source_version,created_at:baseline.created_at,revision:baseline.revision},'实际来源与版本')}${DiffRows(value.diff)}${JsonDetails(baseline.definition,'完整默认字段')}<button data-domain-ui="button" data-domain-variant="${source==='recorded'?'destructive':'default'}" data-domain-size="default" data-command>${source==='recorded'?'确认恢复这些字段':'明确冻结此快照为恢复基准'}</button>`:'<p data-domain-ui="alert" class="banner info">这个旧地点没有可验证的默认快照。请选择当前已保存字段，或存在的已应用定义版本，明确设置基准。不会把当前状态冒充历史初始值，也不会加载演示数据。</p>'}<h3>实际字段修订与恢复历史</h3><div data-history></div><p data-domain-ui="alert" role="alert" data-error></p>`);
   d.el.querySelectorAll('[data-source]').forEach(button=>button.onclick=()=>void load(button.dataset.source));
   d.el.querySelector('[data-command]')?.addEventListener('click',()=>{
    const restoring=source==='recorded',body={expectedRevision:value.place_revision,expectedDefaultRevision:value.default_revision,operation:restoring?'restore':'capture',source};
    durableCommandDrawer({slot:`place-default:${fid}:${wid}:${pid}`,title:restoring?'恢复地点默认字段':'设定地点恢复基准',body:`<p>${esc(restoring?'恢复将保存这些文本与场景字段，并以实际读回结果替换地点编辑页的未保存草稿；不会移动角色或更改引用身份。':'只记录已核对的恢复基准，不改变地点当前字段，也不保存编辑页尚未提交的草稿。')}</p>${DiffRows(value.diff)}${Field({label:'操作理由',name:'reason',type:'textarea',attrs:{required:true,maxlength:1000}})}`,read:box=>({...body,reason:box.querySelector('[name=reason]').value}),execute:(command,key)=>mockWorld.restorePlace(fid,wid,pid,{...command,idempotencyKey:key}),onSaved:async result=>{if(!alive())return;if(!result){await load();return;}if(result.status==='baseline_saved'){await load();return;}await mockWorld.world(fid,wid);if(!alive())return;d.close();await onRestored(result);}});
   });
   panel=mountCursorRecords(d.el.querySelector('[data-history]'),{alive,label:'地点历史',read:cursor=>mockWorld.placeDefaultHistory(fid,wid,pid,cursor),render:row=>`<article data-domain-ui="item" class="lrow"><div class="grow"><b>${esc(stateLabel(row.operation,'字段修订'))} · 地点 v${esc(row.place_revision)} · 基准 v${esc(row.default_revision)}</b><p>${esc(row.reason)} · ${esc(userInstant(row.created_at))}</p>${JsonDetails(row,'字段前后值、基准来源与审计')}</div></article>`,empty:'没有保留的字段或默认基准历史。没有历史时不会补造初始快照。'});
  }catch(error){if(!alive()||token!==epoch)return;d.setBody(`<p role="alert">${esc(error.message)}</p><button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-retry>重新读取</button><button data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" data-current>核对当前已保存字段</button>`);d.el.querySelector('[data-retry]').onclick=()=>void load(source);d.el.querySelector('[data-current]').onclick=()=>void load('current_saved');}
 }
 void load();return d;
}

const historyActions = new Map([
 ['initial_saved','首次保存'],
 ['edit_fields','修改地点字段'],
 ['set_baseline','保存恢复基准'],
 ['restore_fields','恢复默认字段'],
]);
const historySources = new Map([
 ['initial_saved','地点首次保存的字段'],
 ['published_definition','已应用的地点定义'],
 ['owner_pinned_saved','明确保存的地点字段'],
]);
const historyFields = [
 ['name','正式名称'], ['alias','别名'], ['description','场景描述'],
 ['group','分组'], ['floor','楼层'], ['atmosphere','氛围'], ['layout','空间布局'],
 ['furniture','固定陈设'], ['connections','连通描述'], ['room_items','房间物品'],
 ['scene','场景素材'],
];
const historyCollections = new Set(['furniture','connections','room_items']);

function historyText(value) {
 if (typeof value !== 'string' || isInternalId(value)) return '';
 return value.replace(/[a-z0-9_-]+/gi, token => isInternalId(token) ? '相关记录' : token).trim();
}

function historyFieldText(field,value) {
 if (!historyCollections.has(field)) return historyText(value);
 const items = Array.isArray(value) ? value : [value];
 return items.map(item => {
  if (!item || typeof item !== 'object' || Array.isArray(item)) return historyText(item);
  const name = historyText(item.name) || historyText(item.label);
  const description = historyText(item.description);
  if (name && description && description !== name) return `${name}（${description}）`;
  return name || description;
 }).filter(Boolean).join('、');
}

const collectionDetails = readingFields('quantity position placement material color condition notes details');

function historyCollectionDetails(value) {
 return (Array.isArray(value) ? value : [value]).map(item => {
  if (!item || typeof item !== 'object') return '';
  const details = ReadingFields(item,collectionDetails);
  if (!details) return '';
  const name = historyText(item.name) || historyText(item.label);
  return `${name ? `<p>${esc(name)}</p>` : ''}${details}`;
 }).join('');
}

function historyFieldRows(before,after) {
 if (!after || typeof after !== 'object') return '';
 const rows = historyFields.map(([field,label]) => {
  const previous = before?.[field], next = after[field];
  if (!Object.hasOwn(after,field) || previous === next) return '';
  let change;
  if (field === 'scene') {
   const from = typeof previous === 'string' ? previous.trim() : '';
   const to = typeof next === 'string' ? next.trim() : '';
   if (from === to) return '';
   change = to ? (from ? '已更换' : '已设置') : '已清除';
  } else {
   const from = historyFieldText(field,previous);
   const to = historyFieldText(field,next);
   const beforeDetails = historyCollections.has(field) ? historyCollectionDetails(previous) : '';
   const afterDetails = historyCollections.has(field) ? historyCollectionDetails(next) : '';
   if (from === to && beforeDetails === afterDetails) return '';
   const cleared = next == null || (typeof next === 'string' && !next.trim()) ||
    (Array.isArray(next) && next.length === 0);
   change = from === to ? '' : from && to ? `${from} → ${to}` : to || (cleared ? `${from} → 已清除` : `原值：${from}`);
   if (beforeDetails !== afterDetails) {
    const details = `${beforeDetails ? `<p>变更前补充</p>${beforeDetails}` : ''}${afterDetails ? `<p>变更后补充</p>${afterDetails}` : '<p>已清除补充说明</p>'}`;
    return `<div><dt>${esc(label)}</dt><dd>${esc(change)}${details}</dd></div>`;
   }
  }
  return `<div><dt>${esc(label)}</dt><dd>${esc(change)}</dd></div>`;
 }).join('');
 return rows ? `<dl class="record-facts">${rows}</dl>` : '';
}

function placeHistoryRecord(row) {
 const {before_value:before,after_value:after,source,operation,created_at:createdAt} = row;
 const name = historyText(after?.name) || historyText(before?.name) ||
  historyText(after?.alias) || historyText(before?.alias);
 const action = historyActions.get(operation) || '地点字段修订';
 const reason = historyText(row.reason);
 const time = typeof createdAt === 'string' && Number.isFinite(Date.parse(createdAt));
 const sourceLabel = historySources.get(source?.source_kind);
 const baseline = operation === 'set_baseline';
 const fields = baseline ? historyFieldRows(null,source?.definition) : historyFieldRows(before,after);
 return `<article data-domain-ui="card" class="card">
  ${name ? `<h3>${esc(name)}</h3>` : ''}
  <p><b>${esc(action)}</b></p>
  ${reason ? `<p>原因：${esc(reason)}</p>` : ''}
  ${time ? `<p class="small muted">修订时间：<time datetime="${esc(createdAt)}">${esc(userInstant(createdAt))}</time></p>` : ''}
  ${sourceLabel ? `<p class="small muted">基准来源：${esc(sourceLabel)}</p>` : ''}
  ${fields ? `<p><b>${baseline ? '恢复基准字段' : '变化的字段'}</b></p>${fields}` : ''}
  ${RawRecordDownload(row,'原始地点修订记录（诊断）')}
 </article>`;
}

export function renderPlaceHistory(el,ctx){
 const owner=get('session')?.actorId;let active=true;const alive=()=>active&&el.isConnected&&get('session')?.actorId===owner;
 el.innerHTML=`<div class="stack"><div class="row"><a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="#/data/revisions">对象修订目录</a><a data-domain-ui="button" data-domain-variant="outline" data-domain-size="default" href="#/c/${encodeURIComponent(ctx.charId)}/worlds">当前世界目录</a></div><div data-domain-ui="card" class="card"><h2>地点字段与默认基准历史</h2><p>这里展示保留的实际修订，已移除地点的记录也可读取。历史字段不代表角色当前位置或当前世界状态。</p></div><div class="stack" data-history></div></div>`;
 const panel=mountCursorRecords(el.querySelector('[data-history]'),{alive,label:'地点历史',read:cursor=>mockWorld.placeDefaultHistory(ctx.charId,ctx.params.worldId,ctx.params.placeId,cursor),render:placeHistoryRecord,empty:'没有保留的地点字段或恢复基准历史。'});return()=>{active=false;panel.dispose();};
}
