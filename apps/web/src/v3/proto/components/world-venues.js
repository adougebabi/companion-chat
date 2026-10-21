import {esc,badge} from './controls.js';
import {JsonDetails} from './records.js';
function furniture(value){if(value==null||value==='')return '尚未补充';if(typeof value==='string')return esc(value);if(Array.isArray(value))return value.map(v=>typeof v==='string'?esc(v):v&&typeof v==='object'?esc(v.name||v.description||v.label||'陈设详情')+JsonDetails(v,'查看陈设详情'):esc(String(v))).join('、');return JsonDetails(value,'查看陈设详情');}

export function WorldVenueDirectory({categories=[],places=[],renderPlace,createHref}){
 const sections=categories.map(category=>({category,places:places.filter(p=>(p.venueCategoryId||p.venue_category_id)===category.id)}));
 const legacy=places.filter(p=>!categories.some(c=>c.id===(p.venueCategoryId||p.venue_category_id)));
 if(legacy.length)sections.push({category:{id:'',name:'原有未归类地点'},places:legacy});
 return `<div class="stack world-venue-directory">${sections.map(({category,places})=>`<section class="card stack"><div class="row"><h3 class="grow">${esc(category.name)}</h3>${category.id?badge('固定场地分类','plain'):''}${createHref&&category.id?`<a class="btn sm" href="${esc(createHref(category.id))}">添加具体地点</a>`:''}</div>${places.length?`<div class="world-venue-places">${places.map(renderPlace).join('')}</div>`:'<p class="small muted">这个分类还没有具体地点，可以添加。</p>'}</section>`).join('')||'<section class="statebox"><h3>还没有基础场地与地点</h3><p>先描述世界主题，再在世界编辑中让创作助手扩展基础框架，或手动添加地点。</p></section>'}</div>`;
}

export function WorldFrameworkPreview(framework){
 if(!framework?.venues?.length)return '<p class="small muted">尚未准备基础场地与地点。</p>';
 return `<div class="stack world-framework-preview">${framework.venues.map(venue=>`<section class="card stack"><h3>${esc(venue.name)}</h3>${(venue.places||[]).map(place=>`<article class="world-framework-point"><h4>${esc(place.name)}${place.alias?` <span class="small muted">· ${esc(place.alias)}</span>`:''}</h4>${place.floor?`<p class="small muted">楼层：${esc(place.floor)}</p>`:''}<p>${esc(place.description||'描述尚未补充')}</p><dl><dt>氛围</dt><dd>${esc(place.atmosphere||'尚未补充')}</dd><dt>固定陈设</dt><dd>${furniture(place.furniture)}</dd><dt>布局</dt><dd>${esc(place.layout||'尚未补充')}</dd></dl></article>`).join('')}</section>`).join('')}</div>`;
}
