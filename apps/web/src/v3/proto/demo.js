// 演示流程引导（A/B/C），步骤完成由 store 状态自动判断
import * as store from './store.js';
import { icon, esc } from './ui.js';
const g = (c, id) => store.get(c, id) || {};
export const FLOWS = [
  { id: 'A', title: '聊天纠错（#m31）', steps: [
    { t: '打开对话，找到小遥说“海边书店已经停业了呢”', href: '#/c/c1/chat', done: () => true },
    { t: '悬停该消息点「这条不对」，打开纠错抽屉', href: '#/c/c1/chat?correct=msg_007', done: () => g('memories', 'm31').version > 1 || g('messages', 'msg_007').corrected },
    { t: '选「所有者修改」，查看差异并保存（可先试“模拟他处修改”看冲突）', href: '#/c/c1/chat?correct=msg_007', done: () => g('memories', 'm31').version > 1 },
    { t: '去记忆 #m31 查看修订记录', href: '#/c/c1/growth/memory/m31', done: () => g('memories', 'm31').revisions?.length > 1 },
  ] },
  { id: 'B', title: '愿望 → 购物 → 入库 → 换装 → 拍照', steps: [
    { t: '目标页看到意向 g_dress「想要一条碎花裙」', href: '#/c/c1/life/goals', done: () => true },
    { t: '在对话里对行动卡 a_buy 发出「行动请求」', href: '#/c/c1/chat', done: () => g('actions', 'a_buy').phase !== '待请求' },
    { t: '等小遥接受并执行，碎花裙 it_dress 入库', href: '#/c/c1/chat', done: () => g('items', 'it_dress').status === '拥有' || g('items', 'it_dress').status === '穿着' },
    { t: '衣柜里对碎花裙发「换装请求」，等她接受', href: '#/c/c1/life/wardrobe?item=it_dress', done: () => store.get('outfit').c1.current.includes('it_dress') },
    { t: '点「拍照」生成图片（生成图·非事实）', href: '#/c/c1/life/wardrobe', done: () => store.list('media', m => m.itemId === 'it_dress').length > 0 },
    { t: '媒体库查看照片与关联物品', href: '#/c/c1/media', done: () => store.list('media', m => m.itemId === 'it_dress').length > 0 },
  ] },
  { id: 'C', title: '创作预览与应用', steps: [
    { t: '打开资源「海边书店」r_book 编辑', href: '#/studio/edit/r_book', done: () => true },
    { t: '用右侧助手修改草稿，点「保存草稿」', href: '#/studio/edit/r_book', done: () => !!g('resources', 'r_book').draft },
    { t: '在「创作预览·不进入生活」沙盒试生成', href: '#/studio/edit/r_book', done: () => store.get('studioDrafts').length > 0 },
    { t: '「应用到实例」小遥：处理与实例修正的冲突', href: '#/studio/edit/r_book', done: () => store.get('instances').c1.r_book.applied !== 'v3' },
  ] },
];
let active = sessionStorage.getItem('fluctlight.guide') || '';
export function openGuide(id) { active = id; sessionStorage.setItem('fluctlight.guide', id); renderGuide(); }
export function renderGuide() {
  document.querySelector('.guide')?.remove(); if (!active) return;
  const f = FLOWS.find(x => x.id === active); if (!f) return;
  const firstOpen = f.steps.findIndex(s => !s.done());
  const box = document.createElement('div'); box.className = 'guide'; box.dataset.test = 'guide';
  box.innerHTML = `<div class="row"><b class="grow">${icon('route')} 演示流程 ${f.id}：${esc(f.title)}</b><button class="btn ghost icon-only sm" data-close aria-label="关闭引导">${icon('close')}</button></div>
  <ol>${f.steps.map((s, i) => `<li class="${s.done() && i !== firstOpen ? 'done' : ''} ${i === firstOpen ? 'cur' : ''}"><a href="${s.href}">${esc(s.t)}</a></li>`).join('')}</ol>
  <div class="small muted">${firstOpen < 0 ? '✓ 全部完成' : `当前第 ${firstOpen + 1} 步`}</div>`;
  box.querySelector('[data-close]').onclick = () => { active = ''; sessionStorage.removeItem('fluctlight.guide'); box.remove(); };
  document.body.append(box);
}
store.subscribe(() => setTimeout(renderGuide, 0));
