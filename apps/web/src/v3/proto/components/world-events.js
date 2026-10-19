import {esc,badge} from '../ui.js';
import {instant} from './async-page.js';
import {JsonDetails} from './records.js';
export function WorldEventCard(event,{fid,zone}={}){
 return `<a class="card lrow" href="#/c/${encodeURIComponent(fid)}/world/event/${encodeURIComponent(event.id)}"><b class="small">${esc(instant(event.start_at,zone))}</b><span class="grow"><b>${esc(event.title)}</b><p class="small muted">${event.context_scope==='ambient'?'环境或身体事实':'生活事件'} · ${event.world_id?'已关联运行世界':'历史世界归属未记录'}</p></span>${badge(event.status,'plain')}</a>`;
}
export function ActiveWorldEffects(effects,{fid,zone}={}){
 if(!effects?.length)return '';
 return `<div class="card"><h3>当前有效的世界变化</h3>${effects.map(e=>`<div class="lrow"><span class="grow"><a href="#/c/${encodeURIComponent(fid)}/world/event/${encodeURIComponent(e.event_id)}">${esc(e.description)}</a><p class="small muted">虚拟世界事件 · 有效至 ${esc(instant(e.end_at,zone))}</p>${JsonDetails(e.effects,'当前范围效果')}</span></div>`).join('')}</div>`;
}
