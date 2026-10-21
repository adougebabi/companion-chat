import {esc,badge} from './controls.js';
import {clothingTypeOptions,clothingTypeOf,clothingSeasons} from '../../../lib/wardrobe-form.ts';

export function OutfitPreview(outfit){
 const items=outfit.items || [],types=[...clothingTypeOptions];
 const legacy=items.filter(i=>!types.some(t=>t.value===clothingTypeOf(i)));
 return `<div class="stack outfit-preview"><div><h3>${esc(outfit.name)}</h3><div class="row">${(outfit.tags||[]).map(t=>badge(t,'plain')).join('')||'<span class="small muted">尚未标注场景标签</span>'}</div></div><div class="outfit-slot-grid">${types.map(type=>`<section class="outfit-slot"><h4>${type.label}</h4>${items.filter(i=>clothingTypeOf(i)===type.value).map(i=>`<p>${esc(i.description)}${i.archived_at?badge('已删除登记','plain'):''}<span class="small muted">${(i.seasons||[]).map(s=>clothingSeasons.find(v=>v.value===s)?.label||'').filter(Boolean).join(' / ')}</span></p>`).join('')||'<p class="small muted">未选择</p>'}</section>`).join('')}</div>${legacy.length?`<section><h4>原有服装</h4>${legacy.map(i=>`<p>${esc(i.description)}</p>`).join('')}</section>`:''}</div>`;
}
