import { randomId } from '../../random-id.mjs';
// Shared server snapshot facade. The isolated fixture store is build-time opt-in.
export const isMock = typeof import.meta.env !== 'undefined' && import.meta.env.VITE_PRODUCT_MODE === 'mock';
const fixture = isMock ? await import('./mock/store.js') : null;
export const IDENTITIES = ['交流', '行动请求', '所有者修改', '系统运维'];
const tables = ['groups','characters','worlds','places','npcs','relations','messages','memories','goals','schedule','actions','items','outfit','media','events','diary','tasks','proposals','resources','instances','posts','postSuggestions','reflections','worldSettings','ownerRequests'];
let data = { ...Object.fromEntries(tables.map(key => [key, {}])), meta: { surfaces: {}, unwired: {} }, ui: { charId: '', identity: '交流', notes: false }, user: { id: '', name: '我', background: {} }, prefs: { global: {}, autonomy: {} }, session: null, notices: [], audit: [], studioDrafts: [] };
const subs = new Set();
const projections = new Map();
export function configureReadProjection(col, read) { projections.set(col,read); }
function emit(event) { for (const fn of subs) fn(event); }
export function replaceSnapshot(snapshot) {
  const ui = data.ui;
  data = snapshot;
  data.ui = { ...snapshot.ui, ...ui, charId: snapshot.characters[ui.charId] ? ui.charId : snapshot.ui.charId };
  emit({ type: 'snapshot' });
}
export function get(col, id) { if (fixture) return fixture.get(col,id); const value = col ? (projections.has(col) ? projections.get(col)(data[col]) : data[col]) : data; return id === undefined ? value : value?.[id]; }
export function list(col, predicate = () => true) { if (fixture) return fixture.list(col,predicate); const value = get(col); return (Array.isArray(value) ? value : Object.values(value || {})).filter(predicate); }
export function update(...args) { if (fixture) return fixture.update(...args); throw new Error('此操作尚未连接真实命令，未保存任何修改'); }
export function setUi(patch) { if (fixture) return fixture.setUi(patch); Object.assign(data.ui,patch); emit({type:'ui',patch}); }
export function subscribe(fn) { if (fixture) return fixture.subscribe(fn); subs.add(fn); return () => subs.delete(fn); }
export function uid(prefix) { return prefix + randomId(); }
export function resetDemo() { if (fixture) return fixture.resetDemo(); throw new Error('真实模式不提供演示重置'); }
export function auditLog() { return get('audit'); }
export function find(id) { for (const col of tables) { const obj = get(col,id); if (obj) return {col,obj}; } return null; }
export function pending(charId) {
  if(fixture)return fixture.pending(charId);
  const groups=data.meta.pendingGroups;
  if(!groups || groups.some(group=>group.status!=='ok'))return null;
  return groups.flatMap(group=>group.items).filter(item=>!charId || !item.charId || item.charId===charId);
}
export const _debug = {get data() {return get();}};
if (isMock) window.__store = { get,list,update,setUi,subscribe,auditLog,resetDemo,find,pending,_debug };
