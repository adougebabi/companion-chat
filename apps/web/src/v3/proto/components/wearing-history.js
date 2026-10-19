import {inventoryClient} from '../services.js';
import {mountCursorRecords} from './cursor-records.js';
import {OutfitStrip} from './cards.js';
import {JsonDetails} from './records.js';
import {instant} from './async-page.js';
import {esc} from '../ui.js';
export function mountWearingHistory(box,{fid,alive}){
 box.innerHTML='<div class="card"><h3>穿搭变化历史</h3><p class="small muted">实际穿着、物品状态变化与Owner修正分别记录；保存套装定义不会产生穿衣记录。</p><div data-history></div></div>';
 return mountCursorRecords(box.querySelector('[data-history]'),{alive,label:'穿搭记录',read:cursor=>inventoryClient.wearingHistory(fid,cursor),empty:'本迁移后还没有穿搭状态变化。当前穿着见上方；旧历史不会补造。',render:row=>{
  const strip=items=>OutfitStrip({items:items.map(item=>({id:item.id,name:item.description,img:''}))});
  return `<div class="card"><p><b>${esc({tool_action:'工具执行',confirmed_event:'确认事件',owner_correction:'Owner登记修正'}[row.source_kind]||row.source_kind)}</b> · 衣柜 v${esc(row.wardrobe_revision)}</p><p class="small muted">${esc(instant(row.occurred_at))} · 来源 ${esc(row.source_ref||'未记录')}</p><h4>此前已登记的穿着</h4>${strip(row.before_items)}<h4>之后已登记的穿着</h4>${strip(row.after_items)}${row.owner_audit?JsonDetails(row.owner_audit,'Owner理由与审计'):''}</div>`;
 }});
}
