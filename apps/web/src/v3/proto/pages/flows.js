// 跨页业务流程（模拟运行时）：页面与自测共用
import * as store from '../store.js';
import { toast } from '../ui.js';
export const T = { step: 700 }; // 模拟延迟（ms）
window.__flowsT = T;
const wait = ms => new Promise(r => setTimeout(r, ms));
let msgSeq = 0;
export function addMsg(m) { const id = 'msg_' + Date.now().toString(36) + (++msgSeq); store.update(d => { d.messages[id] = { id, charId: 'c1', at: new Date().toTimeString().slice(0, 5), ...m }; }, { identity: m.identity || '交流', summary: m.from === 'user' ? '用户发送消息' : m.from === 'sys' ? '系统事件：' + m.text : '小遥回复消息', target: id }); return id; }

/** 流程 B-1：行动请求「购买碎花裙」a_buy */
export async function requestBuy() {
  const a = store.get('actions', 'a_buy'); if (!a || (a.phase !== '待请求' && a.phase !== '已撤回')) return;
  store.update(d => { const x = d.actions.a_buy; x.phase = '请求中'; x.state = '候选'; x.log = [...(x.log || []), '用户发出行动请求']; }, { identity: '行动请求', summary: '用户请求小遥：购买碎花裙（a_buy）', target: 'a_buy' });
}
export function withdraw(actionId) {
  store.update(d => { d.actions[actionId].phase = '已撤回'; d.actions[actionId].state = '取消'; }, { identity: '行动请求', summary: '用户撤回行动请求 ' + actionId, target: actionId });
}
/** 流程 B-2：换装请求（交流身份提出，角色接受后才改变穿着） */
export async function requestOutfit(itemId) {
  const it = store.get('items', itemId);
  store.update(() => {}, { identity: '行动请求', summary: `用户请求小遥换上 ${it?.name || itemId}`, target: itemId });
  return false;
}
/** 流程 B-3：拍照 → 生成图·非事实（不改变任何事实） */
export function takePhoto({ itemId = null, prompt = '' } = {}) {
  if (!itemId) { toast('不绑定物品的拍照尚未接入', 'err'); return null; }
  store.update(() => {}, { identity: '交流', summary: `物品拍照 ${prompt || ''}`.trim(), target: itemId });
  return null;
}
