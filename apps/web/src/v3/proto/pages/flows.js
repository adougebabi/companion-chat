// 跨页业务流程（模拟运行时）：页面与自测共用
import * as store from '../store.js';
export const T = { step: 700 }; // 模拟延迟（ms）
window.__flowsT = T;
const wait = ms => new Promise(r => setTimeout(r, ms));
let msgSeq = 0;
export function addMsg(m) { const id = 'msg_' + Date.now().toString(36) + (++msgSeq); store.update(d => { d.messages[id] = { id, charId: 'c1', at: new Date().toTimeString().slice(0, 5), ...m }; }, { identity: m.identity || '交流', summary: m.from === 'user' ? '用户发送消息' : m.from === 'sys' ? '系统事件：' + m.text : '小遥回复消息', target: id }); return id; }

/** 流程 B-1：行动请求「购买碎花裙」a_buy */
export async function requestBuy() {
  const a = store.get('actions', 'a_buy'); if (a.phase !== '待请求' && a.phase !== '已撤回') return;
  store.update(d => { const x = d.actions.a_buy; x.phase = '请求中'; x.state = '候选'; x.log.push('用户发出行动请求'); }, { identity: '行动请求', summary: '用户请求小遥：购买碎花裙（a_buy）', target: 'a_buy' });
  await wait(T.step); if (store.get('actions', 'a_buy').phase !== '请求中') return; // 已撤回
  store.update(d => { const x = d.actions.a_buy; x.phase = '已接受'; x.state = '计划'; x.decision = '“好呀！正好下午有空，我去布偶服装店看看～”'; x.log.push('小遥接受'); }, { identity: '行动请求', summary: '小遥接受行动请求 a_buy', target: 'a_buy' });
  addMsg({ from: 'char', text: '好呀！我下午三点半去布偶服装店看看那条碎花裙～', identity: '交流' });
  await wait(T.step);
  store.update(d => { d.actions.a_buy.phase = '执行中'; d.actions.a_buy.state = '计划'; d.items.it_dress.status = '入库中'; d.schedule.s_shop.state = '已确认'; d.characters.c1.locationId = 'p_shop'; d.characters.c1.doing = '在布偶服装店试穿碎花裙'; d.characters.c1.doingActionId = 'a_buy'; }, { identity: '系统运维', summary: '运行时：a_buy 执行中，it_dress 入库中', target: 'a_buy' });
  await wait(T.step);
  store.update(d => { const x = d.actions.a_buy; x.phase = '完成'; x.state = '已确认'; x.log.push('付款成功，物品入库'); d.items.it_dress.status = '拥有'; d.items.it_dress.source = '购买（行动 a_buy，日程 s_shop）'; d.goals.g_dress.state = '已确认'; d.characters.c1.locationId = 'p_home'; d.characters.c1.doing = '回到公寓整理新衣服'; d.characters.c1.doingActionId = null; }, { identity: '系统运维', summary: '运行时：a_buy 完成，it_dress 已入库', target: 'it_dress' });
  addMsg({ from: 'char', text: '买到啦！碎花裙已经挂进衣柜了，好开心～', identity: '交流', itemRef: 'it_dress' });
}
export function withdraw(actionId) {
  store.update(d => { d.actions[actionId].phase = '已撤回'; d.actions[actionId].state = '取消'; }, { identity: '行动请求', summary: '用户撤回行动请求 ' + actionId, target: actionId });
}
/** 流程 B-2：换装请求（交流身份提出，角色接受后才改变穿着） */
export async function requestOutfit(itemId, { refuse = false } = {}) {
  const it = store.get('items', itemId);
  store.update(d => { d.outfit.c1.pending = itemId; }, { identity: '行动请求', summary: `用户请求小遥换上 ${it.name}`, target: itemId });
  await wait(T.step);
  if (refuse) { store.update(d => { d.outfit.c1.pending = null; }, { identity: '行动请求', summary: '小遥婉拒换装', target: itemId }); return false; }
  store.update(d => { const o = d.outfit.c1; o.history.unshift({ at: new Date().toTimeString().slice(0, 5), was: o.current.slice() });
    const it2 = d.items[itemId]; const sameCat = it2.cat === '服装';
    // 碎花裙替换外套
    if (sameCat) o.current = o.current.filter(x => !(itemId === 'it_dress' && x === 'it_coat'));
    if (!o.current.includes(itemId)) o.current.push(itemId); o.pending = null; o.since = new Date().toTimeString().slice(0, 5); }, { identity: '行动请求', summary: `小遥接受换装：穿上 ${it.name}`, target: itemId });
  addMsg({ from: 'char', text: `换好啦～${it.name}好看吗？`, identity: '交流' });
  return true;
}
/** 流程 B-3：拍照 → 生成图·非事实（不改变任何事实） */
export function takePhoto({ itemId = null, prompt = '' } = {}) {
  const id = 'md_' + Date.now().toString(36);
  const outfit = store.get('outfit').c1.current.slice();
  const loc = store.get('characters', 'c1').locationId;
  const scene = store.get('places', loc)?.scene || '/v3/assets/scenes/apartment.svg';
  store.update(d => { d.media[id] = { id, charId: 'c1', img: scene, overlay: itemId ? d.items[itemId].img : null, title: prompt || '换装照片', nonFact: true, itemId, outfitSnapshot: outfit, placeId: loc, at: new Date().toTimeString().slice(0, 5) }; },
    { identity: '交流', summary: '生成照片（生成图·非事实，不改变事实）', target: id });
  return id;
}
