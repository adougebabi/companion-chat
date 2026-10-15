// 摇光原型 · 共享 UI 组件
import * as store from './store.js';

export const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
export const icon = (name, cls = '') => `<svg class="icon ${cls}" aria-hidden="true"><use href="/v3/assets/icons.svg#i-${name}"></use></svg>`;
export const $ = (sel, root = document) => root.querySelector(sel);
export const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];

const BADGE_CLASS = { '生成图·非事实': 'b-nonfact', '创作预览·不进入生活': 'b-preview' };
/** badge('已确认') / badge('交流') / badge('生成图·非事实') / badge('角色范围') / badge('任意文字','plain') */
export function badge(label, kind) {
  const cls = kind ? 'b-' + kind : (BADGE_CLASS[label] || 'b-' + label);
  return `<span class="badge ${esc(cls)}" data-badge="${esc(label)}">${esc(label)}</span>`;
}
export const LABELS = {
  identity: ['交流', '行动请求', '所有者修改', '系统运维'],
  fact: ['计划', '候选', '推测', '已确认', '失败', '取消', '未知'],
  nonfact: ['生成图·非事实', '创作预览·不进入生活'],
  scope: ['全局定义', '角色范围', '运行世界'],
};

let layer = [];
function closeTop() { const t = layer.pop(); if (t) t.close(true); }
document.addEventListener('keydown', e => { if (e.key === 'Escape' && layer.length) { e.preventDefault(); closeTop(); } });

function overlay(kind, { title, body = '', footer = '', onClose, wide } = {}) {
  const scrim = document.createElement('div'); scrim.className = 'scrim';
  const box = document.createElement('div'); box.className = kind; box.setAttribute('role', 'dialog'); box.setAttribute('aria-modal', 'true');
  if (wide) box.style.width = 'min(720px,calc(100vw - 24px))';
  box.innerHTML = `<header><h2>${title}</h2><button class="btn ghost icon-only" data-x aria-label="关闭">${icon('close')}</button></header><div class="body">${body}</div>${footer ? `<footer>${footer}</footer>` : ''}`;
  document.body.append(scrim, box);
  const api = {
    el: box, body: box.querySelector('.body'),
    close(fromEsc) { scrim.remove(); box.remove(); layer = layer.filter(x => x !== api); onClose?.(fromEsc); },
    setBody(html) { api.body.innerHTML = html; },
    setFooter(html) { let f = box.querySelector('footer'); if (!f) { f = document.createElement('footer'); box.append(f); } f.innerHTML = html; },
  };
  scrim.onclick = () => api.close(); box.querySelector('[data-x]').onclick = () => api.close();
  layer.push(api);
  setTimeout(() => box.querySelector('[data-x]')?.focus(), 0);
  return api;
}
/** 右侧抽屉：返回 {el, body, close, setBody, setFooter}，Esc/遮罩/× 可关 */
export const drawer = opts => overlay('drawer', opts);
/** 居中弹窗 */
export const modal = opts => overlay('modal', opts);
/** 二次确认：返回 Promise<boolean> */
export function confirm({ title = '确认', text, okText = '确认', identity = '所有者修改' }) {
  return new Promise(res => {
    let done = false;
    const m = modal({ title: `${badge(identity)} ${esc(title)}`, body: `<p>${text}</p><p class="small muted">${identity === '所有者修改' ? '此操作将记入运行诊断 · 操作审计（diag.audit）。' : ''}</p>`,
      footer: `<button class="btn" data-no>取消</button><button class="btn accent" data-ok data-test="confirm-ok">${esc(okText)}</button>`, onClose: () => { if (!done) res(false); } });
    m.el.querySelector('[data-no]').onclick = () => m.close();
    m.el.querySelector('[data-ok]').onclick = () => { done = true; m.close(); res(true); };
  });
}
/** 所有者操作前确保处于“管理”身份 */
export async function requireOwner() {
  if (store.get('ui').identity === '管理') return true;
  const ok = await confirm({ title: '切换到管理身份', text: '当前为“交流”身份。所有者修改需要切换到“管理”身份后执行。', okText: '切换并继续', identity: '所有者修改' });
  if (ok) store.setUi({ identity: '管理' });
  return ok;
}
let tbox;
export function toast(text, type = '') {
  const guard = store.get('meta')?.toastGuard;
  if (guard && Date.now() < guard && type !== 'err' && !/处理中|尚未|冲突|草稿|未定义|不会|失败|preference_only|状态 |未完成|所有者修改|她刚更新过|候选/.test(text)) return;
  if (!tbox) { tbox = document.createElement('div'); tbox.className = 'toasts'; document.body.append(tbox); }
  const t = document.createElement('div'); t.className = 'toast ' + type; t.textContent = text; tbox.append(t);
  setTimeout(() => t.remove(), 2600);
}

export const STATES = ['正常', '空', '加载', '失败', '未接入', '冲突', '部分成功'];
/** 页内演示状态切换器（HTML），点击由 app 统一处理 */
export function stateSwitcher(cur) {
  return `<div class="statesw" data-test="statesw"><span>演示状态：</span>${STATES.map(s => `<button data-demo-state="${s}" class="${s === cur ? 'on' : ''}">${s}</button>`).join('')}</div>`;
}
/** 通用非正常态渲染 */
export function stateView(state, { zone = '', retry = true, emptyText = '当前范围未找到相关内容', kept = false } = {}) {
  const M = {
    '空': ['🌱', emptyText, '这是空态：不写“没有记录”，而说明范围。'],
    '加载': ['', '', ''],
    '失败': ['⚠️', '加载失败', '显示上次快照时间：10-15 09:40（可能过时）。'],
    '未接入': ['🔌', '该能力尚未接入', `接口状态：待核验。归属：${zone || '—'}。`],
    '冲突': ['🔀', '数据版本冲突', '他处已修改此对象。可查看新版本、比较差异、保留草稿或刷新后再提交。'],
    '部分成功': ['🧩', '部分成功', '4 项中 3 项已完成，1 项失败；已成功的不会重复执行。'],
  };
  if (state === '加载') return `<div class="card" data-test="state-加载"><div class="skel" style="width:40%"></div><div class="skel"></div><div class="skel" style="width:80%"></div><div class="skel" style="width:60%"></div><p class="small muted">首次加载中…</p></div>`;
  const [big, t, d] = M[state] || ['', state, ''];
  const btns = state === '冲突' ? `<button class="btn" data-demo-state="正常">查看新版本</button><button class="btn" data-keep-draft data-test="keep-draft">保留草稿</button><button class="btn primary" data-demo-state="正常">刷新再提交</button>`
    : state === '失败' || state === '部分成功' ? (retry ? `<button class="btn primary" data-demo-state="正常">重试（幂等）</button><a class="btn" href="#/diag/task/t88">查看诊断</a>` : '')
    : state === '空' ? `<button class="btn" data-demo-state="正常">回到正常态</button>` : `<a class="btn" href="#/settings">去系统设置</a>`;
  const keptHtml = state === '冲突' && kept ? `<div class="banner ok" data-test="draft-kept">${icon('check')} 草稿已保留在本地（未提交）。可稍后「刷新再提交」，他处的新版本不会被覆盖。</div>` : '';
  return `<div class="statebox" data-test="state-${state}">${keptHtml}<div class="big">${big}</div><h2>${t}</h2><p class="muted">${d}</p><div class="row" style="justify-content:center">${btns}</div></div>`;
}
/** 原型说明面板（顶栏“原型说明”切换显示） */
export function notesPanel(meta) {
  const iface = (meta.iface || []).map(([n, s]) => `${esc(n)} <b>${esc(s)}</b>`).join('；');
  return `<div class="notes" data-test="notes"><table>
  <tr><td>page key</td><td><code>${esc(meta.key)}</code>　${badge(meta.coverage, 'cov-' + meta.coverage)}　分区：${esc(meta.zone || '')}</td></tr>
  <tr><td>BP</td><td>${esc(meta.bp || '')}</td></tr>
  <tr><td>来源/触发</td><td>${esc(meta.source || '—')}</td></tr>
  <tr><td>接口状态</td><td>${iface || '—'}（现有/补齐/新增/待核验）</td></tr></table></div>`;
}
const ROUTE_OF = {
  messages: o => `#/c/${o.charId}/chat`, memories: o => `#/c/${o.charId}/growth/memory/${o.id}`, goals: o => `#/c/${o.charId}/life/goals`,
  schedule: o => `#/c/${o.charId}/life/schedule`, actions: o => `#/c/${o.charId}/life/now`, items: o => `#/c/${o.charId}/life/wardrobe?item=${o.id}`,
  media: o => `#/c/${o.charId}/media/${o.id}`, places: o => `#/c/c1/world/place/${o.id}`, npcs: o => `#/c/c1/people/npc/${o.id}`, events: o => `#/c/c1/world/event/${o.id}`,
  relations: o => `#/c/${o.charId}/people/relation/${o.id}`, tasks: o => `#/diag/task/${o.id}`, resources: o => `#/studio/edit/${o.id}`, characters: o => `#/c/${o.id}/profile`, proposals: o => `#/c/${o.charId}/growth/self`,
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
  for (const k of ['sourceMsg', 'sourceEvent', 'goalId', 'scheduleId', 'actionId', 'itemId', 'placeId', 'mediaId', 'memoryRef', 'goalRef', 'taskId', 'relatedPlace', 'relatedNpc', 'npcId', 'adjustedBy', 'owner', 'resourceId']) if (o[k]) refs.push([k, o[k]]);
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
export const mgmt = (html, label = '管理信息') => `<details class="mgmt small muted" data-test="mgmt"><summary>${label}</summary><div style="margin-top:4px">${html}</div></details>`;
