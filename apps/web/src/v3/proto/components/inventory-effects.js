import {esc} from '../ui.js';
import {JsonDetails} from './records.js';
export function InventoryEffects(fid,effects){
 if(!Array.isArray(effects)||effects.length===0)return '';
 const labels={'wardrobe.borrow':'借入结果','wardrobe.return':'归还结果','wardrobe.wear':'穿着结果','item.use':'使用结果'};
 return effects.filter(effect=>labels[effect.capability_name]).map(effect=>`<div data-domain-ui="card" class="card"><b>${esc(labels[effect.capability_name])}</b><p>${effect.status==='handled'?(effect.output?.disposition==='no_change'?'本次没有新增变化，目标是否满足见上方请求结果。':'本次未执行该动作，实际原因见收据。'):'动作已提交，以下为实际读回的物品。'}</p>${effect.capability_name==='wardrobe.wear'&&Array.isArray(effect.output?.before_items)?`<div class="inventory-wear-diff"><h4>换装前</h4>${effect.output.before_items.map(item=>`<p>${esc(item.description||'已登记服装')}</p>`).join('')||'<p>此前没有已登记穿着。</p>'}<h4>换装后</h4></div>`:''}${(Array.isArray(effect.output?.items)?effect.output.items:[]).map(item=>{const id=item.item_id||item.id;return id?`<p><a href="#/c/${encodeURIComponent(fid)}/life/wardrobe/item/${encodeURIComponent(id)}">${esc(item.description||id)}</a></p>`:'';}).join('')}${JsonDetails({call_id:effect.call_id,result:effect.output},'收据与来源')}</div>`).join('');
}
