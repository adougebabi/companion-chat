import * as store from '../store.js';
import { esc, icon, badge } from './controls.js';
import { drawer } from './overlay.js';
const ROUTE_OF = {
  messages: o => `#/c/${o.charId}/chat`, memories: o => `#/c/${o.charId}/growth/memory/${o.id}`, goals: o => `#/c/${o.charId}/life/goals`,
  schedule: o => `#/c/${o.charId}/life/schedule`, actions: o => `#/c/${o.charId}/life/now`, items: o => `#/c/${o.charId}/life/wardrobe?item=${o.id}`,
  media: o => `#/c/${o.charId}/media/${o.id}`, places: o => `#/c/${o.charId || store.get('ui').charId}/world/place/${o.id}`, npcs: o => `#/c/${o.charId || store.get('ui').charId}/people/npc/${o.id}`, events: o => `#/c/${o.charId || store.get('ui').charId}/world/event/${o.id}`,
  relations: o => `#/c/${o.charId}/people/relation/${o.id}`, tasks: o => `#/diag/task/${o.id}`, resources: o => `#/studio/${o.type === 'NPC' ? 'npc' : 'edit'}/${o.id}`, characters: o => `#/c/${o.id}/profile`, proposals: o => `#/c/${o.charId}/growth/self`,
  posts: o => `#/feed/${o.id}`, postSuggestions: o => `#/feed`, reflections: o => `#/c/${o.charId}/growth/reflection/${o.id}`, diary: o => `#/c/${o.charId}/life/diary/${o.id}`,
};
export const routeOf = id => { const f = store.find(id); return f ? ROUTE_OF[f.col]?.(f.obj) : null; };
export const nameOf = id => { const f = store.find(id); if (!f) return id; const o = f.obj; return o.name || o.title || o.content || o.text || id; };
/** 对象链接：只有真实 ID 才可跳转 */
export function objLink(id, label) {
  const r = routeOf(id);
  return r ? `<a href="${r}" data-obj="${esc(id)}">${esc(label ?? nameOf(id))}</a>` : `<span class="muted">${esc(label ?? id)}（无真实 ID，不可跳转）</span>`;
}
/** 来源抽屉 */
export function sourceDrawer(objId) {
  const f = store.find(objId);
  if (!f) return drawer({ title: '来源', body: `<p class="muted">对象 ${esc(objId)} 不存在或无真实 ID。</p>` });
  const o = f.obj; const refs = [];
  for (const k of ['sourceMsg', 'sourceEvent', 'goalId', 'scheduleId', 'actionId', 'itemId', 'placeId', 'mediaId', 'memoryRef', 'goalRef', 'taskId', 'relatedPlace', 'relatedNpc', 'npcId', 'adjustedBy', 'owner', 'resourceId', 'sourceRef', 'conversationId', 'turnId']) if (o[k]) refs.push([k, o[k]]);
  (o.evidence || o.mood?.evidence || []).forEach(e => refs.push(['依据', e]));
  (o.refs || []).forEach(e => refs.push(['引用', e]));
  (o.impacts || []).forEach(i => refs.push(['影响', i.id]));
  const audits = store.auditLog().filter(a => a.target === objId).slice(0, 5);
  return drawer({ title: `${icon('link')} 来源 · ${esc(objId)}`, body: `
    <div class="stack"><div class="card tint-sun"><b>${esc(nameOf(objId))}</b><div class="small muted">类型：${f.col} · ID：<code>${esc(objId)}</code></div></div>
    <h3>关联对象</h3><div class="list">${refs.length ? refs.map(([k, v]) => `<div class="lrow"><span class="small muted" style="width:90px">${k}</span>${objLink(v, `${nameOf(v)} (${v})`)}</div>`).join('') : '<p class="muted">无关联</p>'}</div>
    <h3>最近审计</h3>${audits.length ? audits.map(a => `<div class="lrow small">${badge(a.identity)} ${esc(a.summary)} <span class="muted">${a.at}</span></div>`).join('') : '<p class="small muted">无</p>'}</div>` });
}
export function avatarOf(id) { const f = store.find(id); return f?.obj.avatar || '/v3/assets/avatars/user.svg'; }
/** 管理信息折叠区：ID、page key、来源等放这里，日常视图不直接露出 */
export const mgmt = (html, label = '管理信息') => `<details data-domain-ui="disclosure" class="mgmt small muted" data-test="mgmt"><summary>${label}</summary><div style="margin-top:4px">${html}</div></details>`;
