import {inventoryClient} from '../services.js';
import {get} from '../store.js';
import {mountCursorRecords} from './cursor-records.js';
import {OutfitRecord,requestInventoryAction} from './inventory-actions.js';
import {outfitEditor} from './outfit-editor.js';
import {JsonDetails} from './records.js';
import {drawer,esc} from '../ui.js';
export function mountOutfitCatalogue(box,{fid,revision,worn,alive,onSaved}){
 const readOnly=get('characters',fid)?.status==='已退休';
 box.innerHTML='<div data-domain-ui="card" class="card"><h3>已保存穿搭</h3><div data-outfits></div><button data-domain-ui="button" class="btn" data-save-outfit disabled>保存当前穿搭</button></div>';
 const save=box.querySelector('[data-save-outfit]');
 const panel=mountCursorRecords(box.querySelector('[data-outfits]'),{alive,label:'套装',read:cursor=>inventoryClient.outfits(fid,cursor),render:OutfitRecord,empty:'当前人格还没有已保存穿搭。可保存真实服饰组合；保存本身不改变当前穿着。',bind:(list,data)=>{
  save.disabled=readOnly;save.onclick=()=>outfitEditor(fid,{profileId:data.profile_id,revision,itemIds:worn.map(item=>item.item_id||item.id)},onSaved);
  list.querySelectorAll('[data-outfit-edit]').forEach(button=>{button.disabled=readOnly;button.onclick=()=>outfitEditor(fid,{outfit:data.items.find(o=>o.id===button.dataset.outfitEdit),profileId:data.profile_id,revision},onSaved);});
  list.querySelectorAll('[data-outfit-history]').forEach(button=>button.onclick=()=>{
   let history;const d=drawer({title:'套装修订',body:'<div data-revisions></div>',onClose:()=>history?.dispose()});
   history=mountCursorRecords(d.el.querySelector('[data-revisions]'),{label:'套装修订',read:cursor=>inventoryClient.outfitHistory(fid,button.dataset.outfitHistory,cursor),render:row=>`<div data-domain-ui="card" class="card"><b>套装 v${esc(row.revision)}</b>${JsonDetails(row.before_value,'修改前')}${JsonDetails(row.after_value,'保存时的组合')}${JsonDetails(row.owner_audit,'Owner修正理由')}</div>`,empty:'这套旧穿搭没有修订快照；当前定义仍可查看，之后保存会保留真实修订。'});
  });
  list.querySelectorAll('[data-outfit]').forEach(button=>{button.disabled ||= readOnly;button.onclick=()=>{const outfit=data.items.find(o=>o.id===button.dataset.outfit);requestInventoryAction(fid,'outfit',{outfitId:outfit.id,expectedOutfitRevision:outfit.revision,profileId:data.profile_id},revision,onSaved,alive);};});
 }});return {dispose(){panel.dispose();}};
}
