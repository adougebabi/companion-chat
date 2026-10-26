import {esc,badge} from './controls.js';
import {JsonDetails} from './records.js';
function furniture(value){if(value==null||value==='')return '尚未补充';if(typeof value==='string')return esc(value);if(Array.isArray(value))return value.map(v=>typeof v==='string'?esc(v):v&&typeof v==='object'?esc(v.name||v.description||v.label||'陈设详情')+JsonDetails(v,'查看陈设详情'):esc(String(v))).join('、');return JsonDetails(value,'查看陈设详情');}

export function WorldVenueDirectory({categories=[],places=[],renderPlace,createHref}){
 const sections=categories.map(category=>({category,places:places.filter(p=>(p.venueCategoryId||p.venue_category_id)===category.id)}));
 const legacy=places.filter(p=>!categories.some(c=>c.id===(p.venueCategoryId||p.venue_category_id)));
 if(legacy.length)sections.push({category:{id:'',name:'原有未归类地点'},places:legacy});
 return `<div class="stack world-venue-directory">${sections.map(({category,places})=>`<section data-domain-ui="card" class="card stack"><div class="row"><h3 class="grow">${esc(category.name)}</h3>${category.id?badge('固定场地分类','plain'):''}${createHref&&category.id?`<a data-domain-ui="button" class="btn sm" href="${esc(createHref(category.id))}">添加具体地点</a>`:''}</div>${places.length?`<div class="world-venue-places">${places.map(renderPlace).join('')}</div>`:'<p class="small muted">这个分类还没有具体地点，可以添加。</p>'}</section>`).join('')||'<section data-domain-ui="empty" class="statebox"><h3>还没有基础场地与地点</h3><p>先描述世界主题，再在世界编辑中让创作助手扩展基础框架，或手动添加地点。</p></section>'}</div>`;
}

export function WorldFrameworkPreview(framework){
 if(!framework?.venues?.length)return '<p class="small muted">尚未准备基础场地与地点。</p>';
 return `<div class="stack world-framework-preview">${framework.venues.map(venue=>`<section data-domain-ui="card" class="card stack"><h3>${esc(venue.name)}</h3>${(venue.places||[]).map(place=>`<article class="world-framework-point"><h4>${esc(place.name)}${place.alias?` <span class="small muted">· ${esc(place.alias)}</span>`:''}</h4>${place.floor?`<p class="small muted">楼层：${esc(place.floor)}</p>`:''}<p>${esc(place.description||'描述尚未补充')}</p><dl><dt>氛围</dt><dd>${esc(place.atmosphere||'尚未补充')}</dd><dt>固定陈设</dt><dd>${furniture(place.furniture)}</dd><dt>布局</dt><dd>${esc(place.layout||'尚未补充')}</dd></dl></article>`).join('')}</section>`).join('')}</div>`;
}

export function WorldEditorTree({categories=[],places=[],query='',closed=new Set(),selected='world',worldHref,placeHref}) {
 const keyword=query.trim().toLocaleLowerCase(),matches=place=>[place.name,place.alias,place.floor].some(v=>String(v||'').toLocaleLowerCase().includes(keyword));
 const groups=categories.map(category=>({category,rows:places.filter(p=>(p.venueCategoryId||p.venue_category_id)===category.id)}));
 const legacy=places.filter(p=>!categories.some(c=>c.id===(p.venueCategoryId||p.venue_category_id)));
 if(legacy.length)groups.push({category:{id:'legacy',name:'原有未归类地点'},rows:legacy});
 let count=0;
 const content=groups.map(({category,rows})=>{const visible=keyword&&!category.name.toLocaleLowerCase().includes(keyword)?rows.filter(matches):rows;if(keyword&&!visible.length&&!category.name.toLocaleLowerCase().includes(keyword))return '';count+=visible.length;return `<details data-domain-ui="disclosure" class="world-tree-group" data-tree-category="${esc(category.id)}" ${keyword||!closed.has(category.id)?'open':''}><summary>${esc(category.name)} <span class="small muted">· ${rows.length}</span></summary>${visible.map(p=>`<a class="world-tree-place ${selected===p.id?'on':''}" href="${esc(placeHref(p))}" ${selected===p.id?'aria-current="page"':''}>${esc(p.alias||p.name)}${p.floor?`<span class="small muted">${esc(p.floor)}</span>`:''}</a>`).join('')||'<p class="small muted world-tree-empty">暂无具体地点</p>'}</details>`;}).join('');
 return `<a class="world-tree-root ${selected==='world'?'on':''}" href="${esc(worldHref)}" ${selected==='world'?'aria-current="page"':''}>世界本身</a>${content}${keyword&&!count?'<p class="small muted world-tree-empty">没有匹配的地点</p>':''}`;
}
