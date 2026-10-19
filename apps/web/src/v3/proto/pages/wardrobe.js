import { OutfitStrip, ImageCard } from '../components/cards.js';
// items.wardrobe.view —— 衣柜与物品（生活成长 BP-08）
import * as store from '../store.js';
import { esc, icon, badge, objLink, drawer, modal, toast, sourceDrawer } from '../ui.js';
import { KeyValueList } from '../components/forms.js';
import { requestOutfit, takePhoto } from './flows.js';
import { genImg } from './chat.js';

const FILTERS = ['全部', '穿着', '拥有', '借用', '使用', '入库中'];
let filter = '全部'; let openedFor = '';
const statusOf = it => store.get('outfit').c1.current.includes(it.id) ? '穿着' : it.status;
function itemDrawer(id) {
  const it = store.get('items', id); const st = statusOf(it); const o = store.get('outfit').c1;
  const photos = store.list('media', m => m.itemId === id);
  const d = drawer({ title: `${icon('shirt')} ${esc(it.name)}`, body: `<p class="small muted">page key items.item.detail · ${badge('角色范围')}</p>
    <img src="${it.img}" alt="" style="width:160px;border-radius:16px;display:block;margin:0 auto 12px">
    <div class="kv"><dt>ID</dt><dd><code>${it.id}</code></dd><dt>状态</dt><dd><span class="badge st-${st}" data-test="item-status">${st}</span></dd><dt>分类</dt><dd>${it.cat}</dd><dt>来源</dt><dd>${esc(it.source)}</dd>
    ${it.actionId ? `<dt>来源行动</dt><dd>${objLink(it.actionId)}</dd>` : ''}${it.goalId ? `<dt>目标</dt><dd>${objLink(it.goalId)}</dd>` : ''}${it.owner ? `<dt>借自</dt><dd>${objLink(it.owner)}</dd>` : ''}${it.sourceEvent ? `<dt>事件</dt><dd>${objLink(it.sourceEvent)}</dd>` : ''}</div>
    <h3 style="margin-top:14px">相关照片 ${badge('生成图·非事实')}</h3>${photos.length ? `<div class="row">${photos.map(m => `<a href="#/c/c1/media?md=${m.id}" title="${m.id}">${genImg(m)}</a>`).join('')}</div>` : '<p class="small muted">当前范围未找到照片</p>'}
    ${o.pending === id ? `<div class="banner info typing" data-test="outfit-pending">已发出换装请求，等待小遥接受</div>` : ''}`,
    footer: `${it.cat === '服装' && st !== '穿着' && it.status === '拥有' ? `<button class="btn primary" data-dress data-test="outfit-request">${icon('swap')} 换装请求</button>` : ''}<button class="btn" data-photo data-test="photo">${icon('camera')} 拍照</button><button class="btn ghost" data-src>来源</button>` });
  d.el.querySelector('[data-dress]')?.addEventListener('click', async () => { toast('已发出换装请求（行动请求），等待小遥接受'); d.close(); const ok = await requestOutfit(id); toast(ok ? '小遥换上了 ' + it.name : '小遥婉拒了', ok ? 'ok' : 'err'); });
  d.el.querySelector('[data-photo]').onclick = () => { d.close(); photoModal(id); };
  d.el.querySelector('[data-src]').onclick = () => sourceDrawer(id);
}
export function photoModal(itemId) {
  const before = JSON.stringify(store.get('outfit').c1.current);
  const m = modal({ title: `${icon('camera')} 拍照 · media.item.generate`, body: `<div class="banner warn">${badge('生成图·非事实')} 生成的照片只是画面，不会改变穿着、位置或任何事实。</div>
    <label class="field">画面描述<input class="input" data-p value="穿着${esc(store.get('items', itemId)?.name || '今日穿搭')}在公寓窗边"></label><div data-out style="margin-top:12px"></div>`,
    footer: `<button class="btn" data-c>关闭</button><button class="btn primary" data-g data-test="gen-go">${icon('sparkle')} 生成</button>` });
  m.el.querySelector('[data-c]').onclick = () => m.close();
  m.el.querySelector('[data-g]').onclick = () => {
    const out = m.el.querySelector('[data-out]'); out.innerHTML = '<div class="skel" style="height:140px"></div><p class="small muted">生成中…</p>';
    setTimeout(() => { const id = takePhoto({ itemId, prompt: m.el.querySelector('[data-p]').value }); const md = store.get('media', id);
      const same = JSON.stringify(store.get('outfit').c1.current) === before;
      out.innerHTML = `${genImg(md)}<p class="small" data-test="gen-result">已存入媒体库 ${objLink(id, id)} · 关联物品 ${objLink(itemId)} · 穿搭快照 ${md.outfitSnapshot.join(',')}<br>${same ? '✓ 事实未改变（穿着保持不变）' : '!'}</p>`; }, 600);
  };
}
export function renderWardrobe(el, ctx) {
  const items = store.list('items', i => i.charId === 'c1' && i.status !== '未拥有');
  const o = store.get('outfit').c1;
  const shown = items.filter(i => filter === '全部' || statusOf(i) === filter);
  el.innerHTML = `<div class="grid wtop">
    <div class="card tint-primary"><h3>${icon('shirt')} 当前穿着 ${badge('已确认')} <span class="small muted">自 ${o.since}</span></h3>${OutfitStrip({ items: o.current.map(id => store.get('items', id)), attrs: { 'data-test': 'outfit' } })}
      <div class="small" style="margin-top:6px" data-test="outfit-names">${o.current.map(i => esc(store.get('items', i).name)).join('、')}</div>
      ${o.pending ? `<div class="banner info small typing" style="margin-top:8px">换装请求：${esc(store.get('items', o.pending).name)}，等待小遥接受</div>` : ''}</div>
    <div class="card"><h3>${icon('camera')} 拍一张</h3><p class="small muted">生成当前穿搭照片，${badge('生成图·非事实')}</p><button class="btn primary" data-photoall data-test="photo-now">${icon('camera')} 拍照</button></div></div>
  <div class="row" style="margin-bottom:12px" data-test="filters">${FILTERS.map(f => `<button class="chip ${f === filter ? 'on' : ''}" style="${f === filter ? 'background:var(--primary-soft);color:var(--primary-strong);border-color:var(--primary-line)' : ''}" data-f="${f}">${f} ${f === '全部' ? items.length : items.filter(i => statusOf(i) === f).length}</button>`).join('')}</div>
  ${shown.length ? `<div class="wgrid" data-test="wgrid">${shown.map(i => { const st = statusOf(i); return ImageCard({ as: 'button', className: `icard ${st === '穿着' ? 'wearing' : ''}`, img: i.img, attrs: { 'data-item': i.id, 'data-test': `item-${i.id}` }, caption: `<b>${esc(i.name)}</b><div class="row"><span class="badge st-${st}">${st}</span><span class="small muted">${i.cat}</span></div>` }); }).join('')}</div>`
    : `<div class="statebox"><h2>当前筛选「${filter}」未找到物品</h2><button class="btn" data-f="全部">显示全部</button></div>`}
  <p class="small muted" style="margin-top:12px">图例：<span class="badge st-拥有">拥有</span> <span class="badge st-穿着">穿着</span> <span class="badge st-借用">借用</span>（需归还）<span class="badge st-使用">使用</span>（正在用）<span class="badge st-入库中">入库中</span>（异步，可能失败）</p>`;
  el.querySelectorAll('[data-f]').forEach(b => b.onclick = () => { filter = b.dataset.f; renderWardrobe(el, ctx); });
  el.querySelectorAll('[data-item]').forEach(b => b.onclick = () => itemDrawer(b.dataset.item));
  el.querySelector('[data-photoall]').onclick = () => photoModal(o.current.includes('it_dress') ? 'it_dress' : o.current[0]);
  if (ctx.query.item && openedFor !== location.hash && store.get('items', ctx.query.item)) { openedFor = location.hash; itemDrawer(ctx.query.item); }
}
