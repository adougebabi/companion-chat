import {esc,badge} from './controls.js';
import {wardrobeCategoryLabel,wardrobeSlotLabel,wardrobeOwnershipLabel,wardrobeAvailabilityLabel} from '../../../lib/wardrobe-form.ts';
function displayCategory(value){const label=wardrobeCategoryLabel(value);return label===value&&/^[a-z\d_.-]+$/i.test(label)?'自定义分类':label;}
function displaySlot(value){const label=wardrobeSlotLabel(value);return label===value&&/^[a-z\d_.-]+$/i.test(label)?'自定义穿着部位':label;}
/** Initial wardrobe choices and current effective items are distinct sources. */
export function ClothingRecords(values,{mode='initial'}={}){
 if(!Array.isArray(values)||!values.length)return '<span class="small muted">尚未记录</span>';
 return `<div class="stack clothing-records">${values.map(raw=>{if(typeof raw==='string')return `<p>${esc(raw)}</p>`;if(!raw||typeof raw!=='object')return '';const available=typeof raw.available==='boolean'?(raw.available?'可用':'不可用'):wardrobeAvailabilityLabel(raw.availability);const worn=mode==='initial'?(raw.currently_worn===true?'初始穿着':raw.currently_worn===false?'初始未穿着':'初始穿着未设定'):mode==='used'?'使用记录':'当前穿着';return `<div class="clothing-record"><b>${esc(raw.description||raw.name||'物品描述尚未记录')}</b><div class="row small muted"><span>分类：${esc(displayCategory(raw.category))}</span>${raw.slot?`<span>部位：${esc(displaySlot(raw.slot))}</span>`:''}<span>${esc(wardrobeOwnershipLabel(raw.ownership))}</span><span>${esc(available)}</span>${badge(worn,'plain')}</div></div>`;}).join('')}</div>`;
}
