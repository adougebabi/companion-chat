import {esc} from '../ui.js';
import {JsonDetails} from './records.js';
export function InventoryEffects(fid,effects){
 if(!Array.isArray(effects)||effects.length===0)return '';
 const labels={'wardrobe.borrow':'借入结果','wardrobe.return':'归还结果','wardrobe.wear':'穿着结果','item.use':'使用结果'};
 return effects.filter(effect=>labels[effect.capability_name]).map(effect=>`<div class="card"><b>${esc(labels[effect.capability_name])}</b><p>${effect.status==='handled'?(effect.output?.disposition==='no_change'?'本次没有新增变化，目标是否满足见上方请求结果。':'本次未执行该动作，实际原因见收据。'):'动作收据已提交，请重新读取物品状态核对。'}</p>${(Array.isArray(effect.output?.items)?effect.output.items:[]).map(item=>{const id=item.item_id||item.id;return id?`<p><a href="#/c/${encodeURIComponent(fid)}/life/wardrobe/item/${encodeURIComponent(id)}">${esc(item.description||id)}</a></p>`:'';}).join('')}${JsonDetails({call_id:effect.call_id,result:effect.output},'收据与来源')}</div>`).join('');
}
