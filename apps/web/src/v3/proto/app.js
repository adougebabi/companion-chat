import * as store from './store.js';
import * as ui from './ui.js';
import { match, allRoutes } from './modules/registry.js';
import './modules/相处体验.js';
import './modules/生活成长.js';
import './modules/世界人物.js';
import './modules/创作治理.js';
const { icon, esc, badge } = ui;

const NAV = [
  { k: 'chat', label: '对话', icon: 'chat', path: 'chat' },
  { k: 'life', label: '生活', icon: 'life', path: 'life/now', subs: [['此刻', 'life/now', 'now'], ['日程', 'life/schedule', 'calendar'], ['目标', 'life/goals', 'target'], ['衣柜', 'life/wardrobe', 'shirt'], ['日记', 'life/diary', 'book']] },
  { k: 'world', label: '世界', icon: 'globe', path: 'world' },
  { k: 'people', label: '人物关系', icon: 'people', path: 'people' },
  { k: 'growth', label: '记忆成长', icon: 'brain', path: 'growth' },
  { k: 'media', label: '媒体', icon: 'image', path: 'media' },
  { k: 'profile', label: '角色资料', icon: 'user', path: 'profile' },
];
const GLOBAL = [['角色库', '#/library', 'home'], ['动态', '#/feed', 'feed'], ['创作中心', '#/studio', 'studio'], ['系统设置', '#/settings', 'settings'], ['运行诊断', '#/diag', 'diag'], ['通知', '#/inbox', 'inbox']];
const keptDrafts = {};
let current = null; const HKEY = 'fluctlight.backstack';
let back = []; let popping = false;
let root = document;

function parse() {
  const raw = location.hash.slice(1) || '/c/c1/chat';
  const [path, qs] = raw.split('?'); const query = Object.fromEntries(new URLSearchParams(qs || ''));
  return { raw, path, query };
}
export function navigate(h) { location.hash = h.startsWith('#') ? h : '#' + h; }
export function goBack() {
  if (back.length > 1) { back.pop(); const prev = back[back.length - 1]; popping = true; sessionStorage.setItem(HKEY, JSON.stringify(back)); location.hash = prev; }
  else { const q = parse().query; navigate(q.from ? q.from : '#/library'); }
}

function shell(host) {
  root = host || document.body;
  root.innerHTML = `
  <header class="topbar">
    <a class="brand" href="#/library" title="角色库"><span class="brand-dot"></span><span class="hide-n">摇光</span></a>
    <button class="charsw" data-test="charsw" id="charsw"></button>
    <div class="scopebar" id="scopebar"></div>
    <span class="spacer"></span>
    <a class="btn ghost icon-only bell" href="#/inbox" aria-label="通知" data-test="bell">${icon('bell')}<span class="count" id="bellc"></span></a>
    <button class="btn ghost icon-only" id="moreBtn" aria-label="更多">${icon('more')}</button>
  </header>
  <div class="layout"><nav class="sidenav" id="sidenav" aria-label="角色工作区导航"></nav><main class="main" id="main"></main></div>
  <nav class="bottombar" id="bottombar" aria-label="底部导航"></nav>`;
  ui.$('#charsw').onclick = e => charMenu(e.currentTarget);
  ui.$('#moreBtn').onclick = e => moreMenu(e.currentTarget);
  document.addEventListener('click', e => {
    const kd = e.target.closest('[data-keep-draft]'); if (kd && current) { keptDrafts[current.def.key] = true; ui.toast('草稿已保留（未提交）', 'ok'); renderPage(); return; }
    if (e.target.closest('[data-back]')) { e.preventDefault(); goBack(); return; }
    const src = e.target.closest('[data-source]'); if (src) { e.preventDefault(); ui.sourceDrawer(src.dataset.source); }
  });
}
function popmenu(anchor, items) {
  document.querySelector('.popmenu')?.remove();
  const m = document.createElement('div'); m.className = 'popmenu';
  const r = anchor.getBoundingClientRect(); m.style.top = (r.bottom + 6 + scrollY) + 'px'; m.style.left = Math.max(8, Math.min(r.left, innerWidth - 260)) + 'px';
  m.innerHTML = items.map((it, i) => it === '-' ? '<hr style="border:0;border-top:1px solid var(--line)">' : `<button data-i="${i}" ${it.test ? `data-test="${it.test}"` : ''}>${it.html}</button>`).join('');
  document.body.append(m);
  m.onclick = e => { const b = e.target.closest('[data-i]'); if (b) { m.remove(); items[b.dataset.i].run(); } };
  setTimeout(() => document.addEventListener('click', function off(ev) { if (!m.contains(ev.target)) { m.remove(); document.removeEventListener('click', off); } }), 0);
}
function charMenu(a) {
  const groups = store.list('groups');
  const items = [];
  groups.forEach(g => store.list('characters', c => c.groupId === g.id).forEach(c => items.push({ test: 'switch-' + c.id, html: `<img class="av sm" src="${c.avatar}" alt=""> <span class="grow">${esc(c.name)} <span class="small muted">· ${g.name}</span></span>${badge(c.status, c.status === '正常' ? '已确认' : c.status === '异常' ? '失败' : '未知')}`,
    run: () => { if (c.status === '已退休') ui.toast('星野已退休，仅可在角色库查看归档'); store.setUi({ charId: c.id }); navigate(`#/c/${c.id}/chat`); } })));
  items.push('-', { html: `${icon('home')} 角色库`, run: () => navigate('#/library') });
  popmenu(a, items);
}
function moreMenu(a) {
  popmenu(a, GLOBAL.map(([l, h, i]) => ({ html: `${icon(i)} ${l}`, run: () => navigate(h) })));
}
function renderChrome(ctx) {
  const st = store.get('ui'); const c = store.get('characters', ctx.charId) || store.get('characters', 'c1') || { id: ctx.charId || 'c1', name: '尚未选择角色', avatar: '/v3/assets/avatars/user.svg', status: '未知', worldId: null }; const w = c.worldId ? store.get('worlds', c.worldId) : null;
  ui.$('#charsw').innerHTML = `<img class="av" src="${c.avatar}" alt=""><b>${esc(c.name)}</b>${icon('more')}`;
  ui.$('#scopebar').innerHTML = `<span class="hide-n">${badge(ctx.def?.scope || '角色范围')}</span><span class="hide-n">${esc(c.name)} · ${esc(w?.name || '')}</span>
    <span class="ident" data-test="ident">${['交流', '管理'].map(v => `<button data-v="${v}" class="${st.identity === v ? 'on' : ''}">${v}</button>`).join('')}</span>`;
  ui.$$('#scopebar .ident button').forEach(b => b.onclick = () => { store.setUi({ identity: b.dataset.v }); ui.toast(`已切换为「${b.dataset.v}」视图，这不会改变服务器权限`); });
  const unread = store.bellCount();
  ui.$('#bellc').textContent = unread ?? ''; ui.$('#bellc').style.display = unread ? '' : 'none';
  const p = ctx.path; const base = `#/c/${c.id}/`;
  const on = path => p === '/c/' + c.id + '/' + path || p.startsWith('/c/' + c.id + '/' + path + '/');
  ui.$('#sidenav').innerHTML = `<div class="sec">角色工作区 · ${esc(c.name)}</div>` + NAV.map(n => `<a class="navlink ${n.subs ? (p.includes('/life/') ? 'on' : '') : on(n.path) ? 'on' : ''}" href="${base}${n.path}">${icon(n.icon)} ${n.label}</a>` +
    (n.subs ? n.subs.map(([l, sp, ic]) => `<a class="navlink sub ${on(sp) ? 'on' : ''}" href="${base}${sp}">${icon(ic)} ${l}</a>`).join('') : '')).join('') +
    `<div class="sec">全局</div>` + GLOBAL.map(([l, h, i]) => `<a class="navlink ${('#' + p).startsWith(h) ? 'on' : ''}" href="${h}">${icon(i)} ${l}</a>`).join('');
  const bb = [['对话', 'chat', 'chat'], ['此刻', 'life/now', 'now'], ['衣柜', 'life/wardrobe', 'shirt'], ['记忆', 'growth', 'brain']];
  ui.$('#bottombar').innerHTML = bb.map(([l, sp, i]) => `<a href="${base}${sp}" class="${on(sp) ? 'on' : ''}">${icon(i)}${l}</a>`).join('') + `<button id="navDrawer" data-test="nav-drawer">${icon('menu')}全部</button>`;
  ui.$('#navDrawer').onclick = () => { const d = ui.drawer({ title: '导航', body: ui.$('#sidenav').innerHTML }); d.body.onclick = e => { if (e.target.closest('a')) d.close(); }; };
}
function renderPage() {
  const { def, ctx } = current; const main = ui.$('#main');
  const state = '正常';
  ctx.state = state; renderChrome(ctx);
  const showBack = back.length > 1 || ctx.query.from;
  main.innerHTML = `<div class="pagehead">${showBack ? `<button class="btn sm" data-back data-test="back">${icon('back')} 返回</button>` : ''}<h1>${def.title}</h1>${badge(def.scope || '角色范围')}<span class="spacer"></span></div>
    <div id="page" data-page="${def.key}"></div>`;
  const el = ui.$('#page');
  try { def.render(el, ctx); }
  catch (e) { console.error(e); el.innerHTML = `<div class="banner err">页面渲染出错：${esc(e.message)}</div>`; }
}
function route() {
  const { raw, path, query } = parse();
  if (!location.hash) { history.replaceState(null, '', '#/c/c1/chat'); }
  if (!popping) { if (back[back.length - 1] !== '#' + raw) back.push('#' + raw); if (back.length > 40) back.shift(); sessionStorage.setItem(HKEY, JSON.stringify(back)); }
  popping = false;
  let m = match(path);
  if (!m) m = { def: { key: 'sys.notfound', title: '页面不存在', zone: '总设计', coverage: '未完成', bp: '—', render: el => el.innerHTML = `<div class="placeholder"><div class="card"><div style="font-size:42px">🧭</div><h2>未完成：没有这个页面</h2><p class="muted">路由 <code>${esc(path)}</code> 尚未在任何分区注册。</p><button class="btn" data-back>返回</button></div></div>` }, params: {} };
  const charId = m.params.charId || store.get('ui').charId || 'c1';
  if (m.params.charId && store.get('ui').charId !== charId && store.get('characters', charId)) store.setUi({ charId });
  current = { ...m, ctx: { params: m.params, query, path, charId, def: m.def, navigate, goBack, refresh: () => renderPage(), store, ui } };
  document.querySelectorAll('.drawer,.modal,.scrim,.popmenu').forEach(x => x.remove());
  renderPage(); window.scrollTo(0, 0);
}
function adoptLocation() {
  try { back = JSON.parse(sessionStorage.getItem(HKEY) || '[]'); } catch { back = []; }
  if (location.hash) return;
  const path = location.pathname.replace(/\/+$/, '') || '/';
  const legacy = { '/instances': '/library', '/moments': '/feed', '/settings/diagnostics': '/diagnostics' };
  const charId = store.get('ui').charId || 'c1';
  location.hash = path === '/' ? `#/c/${charId}/chat` : `#${legacy[path] || path}${location.search}`;
}
export async function mountProduct(host) {
  shell(host);
  ui.$('#main').innerHTML = `<div class="card"><div class="skel" style="width:40%"></div><div class="skel"></div><p class="small muted">首次加载中…</p></div>`;
  try { await store.hydrate(); } catch (error) { ui.$('#main').innerHTML = `<div class="banner err">${esc(error.message || '加载失败')}</div>`; }
  store.subscribe(evt => { if (!current) return; if (evt.type === 'ui' || evt.type === 'reset' || !current.def.manualRefresh) renderPage(); else current.def.onStore?.(evt, current.ctx); });
  addEventListener('hashchange', route);
  adoptLocation();
  if (location.hash) route();
  window.__app = { navigate, goBack, routes: allRoutes, get current() { return current; } };
}
