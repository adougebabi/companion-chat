import {esc} from './controls.js';
import {eventEffectLabels} from './event-definition.js';

export const worldRecordLabel=value=>({confirmed:'已确认发生',planned:'尚在计划',cancelled:'已取消',not_required:'无需恢复',pending:'等待处理',queued:'已排队',applied:'已应用',completed:'已完成',failed:'失败',filtered:'条件未满足，未应用',recovered:'已恢复',scheduled:'已安排恢复',world:'整个世界',place:'指定地点',character:'当前角色',open:'开放',closed:'关闭',daily:'每天',weekday:'工作日',weekend:'周末'}[value]||(/[\u3400-\u9fff]/.test(String(value||''))?value:'尚未确认'));
export function MissingWorld(fid){return `<div data-domain-ui="empty" class="statebox"><h2>尚未选择运行世界</h2><p>先创建世界，或从世界目录明确选择已有世界，再查看事件与随机事件设定。浏览不会创建或切换世界。</p><a data-domain-ui="button" class="btn primary" href="#/c/${encodeURIComponent(fid)}/worlds">创建或选择世界</a><a data-domain-ui="button" class="btn" href="#/c/${encodeURIComponent(fid)}/world">返回世界目录</a></div>`;}
export function EffectSummary(effects){
 const rows=Array.isArray(effects)?effects:effects?.effects;
 if(!Array.isArray(rows)||!rows.length)return '<p class="small muted">尚无可读的效果记录，可展开来源核对原始内容。</p>';
 return `<ul>${rows.map(e=>`<li>${esc(eventEffectLabels[e.kind]||'世界变化')} · ${esc(worldRecordLabel(e.scope))}：${esc(e.value==='open'||e.value==='closed'?worldRecordLabel(e.value):typeof e.value==='string'?e.value:e.description||'详见来源记录')}</li>`).join('')}</ul>`;
}
export function definitionCondition(d={}){
 const c=d.condition||{},p=d.trigger?.probability;
 const probability=typeof p==='number'&&Number.isFinite(p)?`${Math.round(p*10000)/100}%`:'尚未配置';
 const time=c.start&&c.end?(c.start===c.end?'全天':`${c.start}—${c.end}${c.end<c.start?'（跨夜）':''}`):'时间尚未配置';
 return {probability,time,day:worldRecordLabel(c.day)};
}
