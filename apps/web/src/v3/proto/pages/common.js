// 扩展页面共用小工具
import * as store from '../store.js';
import { esc, badge, icon } from '../ui.js';
export const statusKind = s => s === '正常' ? '已确认' : s === '异常' ? '失败' : '未知';
export const wait = ms => new Promise(r => setTimeout(r, ms));
export const hm = () => new Date().toTimeString().slice(0, 5);
const STATUS_TEXT = {
  '异常': ['运行世界同步失败，这里的内容暂时读不到。', '<a class="btn primary" href="#/diag">去运行诊断</a>'],
  '已暂停': ['所有者暂停了她的自治，生活停在暂停那一刻，不会产生新内容。', '<a class="btn" href="#/c/{id}/settings/autonomy">查看自治设置</a>'],
  '已退休': ['已退休，只读归档；不再产生新的生活内容。', '<a class="btn" href="#/library">回角色库</a>'],
  '未启动': ['刚创建，运行世界还没启动，所以暂时没有生活内容。', '<a class="btn" href="#/studio">去创作中心</a>'],
};
/** 非小遥角色：显示她自己的状态，不回落到小遥数据。返回 true 表示已渲染。 */
export function charStatus(el, ctx, what = '这一页') {
  const c = store.get('characters', ctx.charId);
  if (!c || c.id === 'c1' || (!store.isMock && c.status === '正常')) return false;
  const [txt, btn] = STATUS_TEXT[c.status] || ['当前范围未找到内容。', ''];
  const w = store.get('worlds', c.worldId);
  el.innerHTML = `<div class="card" data-test="char-status-${c.id}"><div class="row"><img class="av lg" src="${c.avatar}" alt=""><div class="grow"><h2 style="margin:0">${esc(c.name)}</h2><div class="small muted">${esc(w?.name || '运行世界未创建')}</div></div>${badge(c.status, statusKind(c.status))}</div>
   <p>${esc(c.bio || '')}</p>${c.statusNote ? `<div class="banner ${c.status === '异常' ? 'err' : 'warn'}">${icon('warn')} ${esc(c.statusNote)}</div>` : ''}
   <div class="statebox" style="margin-top:12px"><h2>${esc(c.name)}的${esc(what)}</h2><p class="muted">${esc(txt)}</p><div class="row" style="justify-content:center">${btn.replace('{id}', c.id)}<a class="btn" href="#/c/${c.id}/profile">她的资料</a><a class="btn ghost" href="#/c/c1/chat">切换到小遥</a></div></div></div>`;
  return true;
}
export const guard = (fn, what) => (el, ctx) => charStatus(el, ctx, what) || fn(el, ctx);
export const hl = (t, kw) => kw ? esc(t).split(esc(kw)).join(`<mark>${esc(kw)}</mark>`) : esc(t);
