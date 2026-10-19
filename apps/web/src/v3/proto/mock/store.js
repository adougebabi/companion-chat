// 摇光原型 · 唯一 mock 数据仓库（localStorage 持久化）
// 规则：所有对象 ID 全局唯一；跨对象只用 ID 引用；任何修改必须走 update() 并带操作身份，自动写审计。
import { createMockSeed as seed } from './seed.js';

const KEY = 'fluctlight.v3.acceptance.mock.v1';
export const IDENTITIES = ['交流', '行动请求', '所有者修改', '系统运维'];


let data = load();
const subs = new Set();
function load() {
  try { const s = JSON.parse(localStorage.getItem(KEY)); if (s && s.meta?.seedVersion === 4) return s; } catch (e) { /* ignore */ }
  return seed();
}
function persist() { try { localStorage.setItem(KEY, JSON.stringify(data)); } catch { throw new Error('Mock 保存失败：浏览器存储不可用或已满，修改尚未保存'); } }
function emit(evt) { subs.forEach(fn => { try { fn(evt); } catch (e) { console.error(e); } }); }
function now() { const d = new Date(); return d.toTimeString().slice(0, 5); }

/** 读取：get('memories','m31') 或 get('memories') 得到对象表；get() 返回整个快照（只读） */
export function get(col, id) { if (!col) return data; const c = data[col]; return id === undefined ? c : c?.[id]; }
export function list(col, pred = () => true) { const c = data[col]; return (Array.isArray(c) ? c : Object.values(c || {})).filter(pred); }
/** 写入：update(draft => {...}, {identity, summary, target}) —— identity 必须是四种操作身份之一 */
export function update(mutator, { identity, summary, target = null, silent = false } = {}) {
  if (!IDENTITIES.includes(identity)) throw new Error('update() 需要合法的操作身份: ' + identity);
  const before = structuredClone(data); let res;
  try { res = mutator(data);
  const entry = { id: 'au_' + (++data.meta.seq), at: now(), identity, summary, target, charId: data.ui.charId };
  data.audit.unshift(entry);
  persist(); } catch (error) { data = before; throw error; }
  const entry = data.audit[0]; if (!silent) emit({ type: 'update', entry });
  return res;
}
/** UI 偏好（不算业务事实，不审计） */
export function setUi(patch) { Object.assign(data.ui, patch); persist(); emit({ type: 'ui', patch }); }
export function subscribe(fn) { subs.add(fn); return () => subs.delete(fn); }
export function auditLog() { return data.audit; }
export function uid(prefix) { data.meta.seq++; persist(); return prefix + data.meta.seq; }
/** 重置演示数据（仅原型，不是时间线回档） */
export function resetDemo() { const ui = { ...data.ui }; data = seed(); data.ui.notes = ui.notes; persist(); emit({ type: 'reset' }); }
/** 通用查找：任意 ID → {col, obj} */
const COLS = ['characters', 'messages', 'memories', 'goals', 'schedule', 'actions', 'items', 'media', 'places', 'npcs', 'events', 'relations', 'diary', 'tasks', 'resources', 'proposals', 'posts', 'postSuggestions', 'reflections'];
export function find(id) { for (const c of COLS) if (data[c]?.[id]) return { col: c, obj: data[c][id] }; return null; }
/** 待处理事项（派生自已有对象，不另存） */
export function pending(charId = 'c1') {
  const out = [];
  list('proposals', p => p.charId === charId && p.state === '候选').forEach(p => out.push({ id: p.id, text: p.title, link: '#/c/' + charId + '/profile' }));
  list('resources').forEach(r => { const inst = data.instances[charId]?.[r.id]; if (inst && inst.applied !== r.published) out.push({ id: r.id, text: `${r.name} 定义 ${r.published} 待应用（当前 ${inst.applied}）`, link: '#/studio/' + (r.type === 'NPC' ? 'npc/' : 'edit/') + r.id }); });
  list('tasks', t => t.charId === charId && t.state === '失败').forEach(t => out.push({ id: t.id, text: `任务 ${t.id} 失败：${t.title}`, link: '#/diag/task/' + t.id }));
  return out;
}
export const _debug = { get data() { return data; } };
window.__store = { get, list, update, setUi, subscribe, auditLog, resetDemo, find, pending, _debug };
