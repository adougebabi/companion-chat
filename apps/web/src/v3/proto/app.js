import {exchangeNavigation,managementNavigation,globalNavigation} from '../workspace-navigation.mjs';
import { ApplicationShell } from './components/shell.js';
import * as store from './store.js';
import * as ui from './ui.js';
import { match, allRoutes } from './modules/registry.js';
const demo = store.isMock ? await import('./demo.js') : { FLOWS: [], openGuide() {}, renderGuide() {} };
const { FLOWS, openGuide, renderGuide } = demo;
const real = store.isMock ? null : await import('../providers/real.ts');
import './modules/相处体验.js';
import './modules/生活成长.js';
import './modules/世界人物.js';
import './modules/创作治理.js';
const { icon, esc, badge } = ui;

const demoStates = {}; const keptDrafts = {}; // route key → 演示状态
let disposePage = null;
let current = null; const HKEY = 'fluctlight.backstack';
let back = JSON.parse(sessionStorage.getItem(HKEY) || '[]'); let popping = false;

function parse() {
  const raw = location.hash.slice(1) || '/library';
  const [path, qs] = raw.split('?'); const query = Object.fromEntries(new URLSearchParams(qs || ''));
  return { raw, path, query };
}
export function navigate(h) { location.hash = h.startsWith('#') ? h : '#' + h; }
export function goBack() {
  if (back.length > 1) { back.pop(); const prev = back[back.length - 1]; popping = true; sessionStorage.setItem(HKEY, JSON.stringify(back)); location.hash = prev; }
  else { const q = parse().query; navigate(q.from ? q.from : '#/library'); }
}

function shell(host) {
  host.innerHTML = ApplicationShell();
  ui.$('#notesT').onchange = e => store.setUi({ notes: e.target.checked });
  ui.$('#charsw').onclick = e => charMenu(e.currentTarget);
  if (store.isMock) ui.$('#demoBtn').onclick = e => demoMenu(e.currentTarget);
  ui.$('#moreBtn').onclick = e => moreMenu(e.currentTarget);
  const onDocumentClick = e => {
    const kd = e.target.closest('[data-keep-draft]'); if (kd && current) { keptDrafts[current.def.key] = true; ui.toast('草稿已保留（未提交）', 'ok'); renderPage(); return; }
    const s = e.target.closest('[data-demo-state]'); if (s) { demoStates[current.def.key] = s.dataset.demoState; renderPage(); return; }
    if (e.target.closest('[data-back]')) { e.preventDefault(); goBack(); return; }
    const src = e.target.closest('[data-source]'); if (src) { e.preventDefault(); ui.sourceDrawer(src.dataset.source); }
  };
  document.addEventListener('click', onDocumentClick);
  return () => document.removeEventListener('click', onDocumentClick);
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
  groups.forEach(g => store.list('characters', c => c.groupId === g.id).forEach(c => items.push({ test: 'switch-' + c.id, html: `<img class="av sm" src="${esc(c.avatar)}" alt=""> <span class="grow">${esc(c.name)} <span class="small muted">· ${esc(g.name)}</span></span>${badge(c.status, c.status === '正常' ? '已确认' : c.status === '异常' ? '失败' : '未知')}`,
    run: () => { if (c.status === '已退休') ui.toast('星野已退休，仅可在角色库查看归档'); store.setUi({ charId: c.id }); navigate(`#/c/${c.id}/chat`); } })));
  items.push('-', { html: `${icon('home')} 角色库`, run: () => navigate('#/library') });
  popmenu(a, items);
}
function demoMenu(a) {
  popmenu(a, [...FLOWS.map(f => ({ test: 'flow-' + f.id, html: `${icon('route')} <span class="grow"><b>流程 ${f.id}</b> ${esc(f.title)}</span>`, run: () => openGuide(f.id) })), '-',
    { test: 'reset-demo', html: `${icon('reset')} 重置演示数据（仅原型，不是时间线回档）`, run: async () => { if (await ui.confirm({ title: '重置演示数据', text: '恢复原型种子数据（仅原型，不是时间线回档）。', identity: '系统运维', okText: '重置' })) { store.resetDemo(); ui.toast('已重置演示数据', 'ok'); } } }]);
}
function moreMenu(a) {
  popmenu(a, [...globalNavigation(store.get('ui').identity).map(([l, h, i]) => ({ html: `${icon(i)} ${l}`, run: () => navigate(h) })), ...(store.isMock ? ['-',
    { html: `${icon('route')} 演示流程…`, run: () => demoMenu(a) },
    { html: `${icon('info')} ${store.get('ui').notes ? '隐藏' : '显示'}原型说明`, run: () => store.setUi({ notes: !store.get('ui').notes }) }] : [])]);
}
function renderChrome(ctx) {
  const st = store.get('ui'); const c = store.get('characters', ctx.charId) || { id: ctx.charId||'', name: ctx.charId?'角色资料待读取':'角色库', avatar: '', worldId: null }; const w = store.get('worlds', c.worldId);
  ui.$('#charsw').innerHTML = `${c.avatar?`<img class="av" src="${esc(c.avatar)}" alt="">`:`<span class="av" aria-hidden="true" style="display:grid;place-items:center;background:var(--primary-soft)">${esc(c.name.slice(0,1))}</span>`}<b>${esc(c.name)}</b>${icon('more')}`;
  ui.$('#scopebar').innerHTML = `<span class="hide-n">${badge(ctx.def?.scope || '角色范围')}</span><span class="hide-n">${esc(c.name)} · ${esc(w?.name || '')}</span>
    <span class="ident" data-test="ident">${['交流', '管理'].map(v => `<button data-v="${v}" class="${st.identity === v ? 'on' : ''}">${v}</button>`).join('')}</span>`;
  ui.$$('#scopebar .ident button').forEach(b => b.onclick = () => { store.setUi({ identity: b.dataset.v }); navigate(b.dataset.v==='管理'?'#/settings':c.id?`#/c/${c.id}/chat`:'#/library'); });
  const pending=store.pending(ctx.charId);
  const meta=store.get('meta');
  const unread = store.isMock ? (pending===null ? null : store.get('notices').filter(n => !n.read).length + pending.length) : (typeof meta.alertCount==='number' && typeof meta.pendingCount==='number' && meta.pendingPartial===false ? meta.alertCount+meta.pendingCount : null);
  ui.$('#bellc').textContent = unread===null?'?':unread; ui.$('#bellc').style.display = unread===null || unread ? '' : 'none';
  ui.$('#bellc').title=unread===null?'部分待处理来源尚未读取，不能判断是否全部处理完':'';
  ui.$('#notesT').checked = store.isMock && st.notes; document.body.classList.toggle('show-notes', store.isMock && !!st.notes);
  const p = ctx.path; const base = `#/c/${c.id}/`;
  const on = path => p === '/c/' + c.id + '/' + path || p.startsWith('/c/' + c.id + '/' + path + '/');
  ui.$('#sidenav').innerHTML = `<div class="sec">角色工作区 · ${esc(c.name)}</div>` + (c.id ? (st.identity==='管理'?managementNavigation:exchangeNavigation) : []).map(n => `<a class="navlink ${n.subs ? (p.includes('/life/') ? 'on' : '') : on(n.path) ? 'on' : ''}" href="${base}${n.path}">${icon(n.icon)} ${n.label}</a>` +
    (n.subs ? n.subs.map(([l, sp, ic]) => `<a class="navlink sub ${on(sp) ? 'on' : ''}" href="${base}${sp}">${icon(ic)} ${l}</a>`).join('') : '')).join('') +
    `<div class="sec">全局</div>` + globalNavigation(st.identity).map(([l, h, i]) => `<a class="navlink ${('#' + p).startsWith(h) ? 'on' : ''}" href="${h}">${icon(i)} ${l}</a>`).join('');
  const bb = st.identity==='管理' ? [['角色资料','profile','user'],['世界管理','worlds','globe'],['自治设置','settings/autonomy','settings']] : [['对话', 'chat', 'chat'], ['此刻', 'life/now', 'now'], ['衣柜', 'life/wardrobe', 'shirt'], ['记忆', 'growth', 'brain']];
  ui.$('#bottombar').innerHTML = (c.id ? bb : []).map(([l, sp, i]) => `<a href="${base}${sp}" class="${on(sp) ? 'on' : ''}">${icon(i)}${l}</a>`).join('') + `<button id="navDrawer" data-test="nav-drawer">${icon('menu')}全部</button>`;
  ui.$('#navDrawer').onclick = () => { const d = ui.drawer({ title: '导航', body: ui.$('#sidenav').innerHTML }); d.body.onclick = e => { if (e.target.closest('a')) d.close(); }; };
}
function renderPage() {
  disposePage?.(); disposePage = null;
  const { def, params, ctx } = current; const main = ui.$('#main');
  const state = demoStates[def.key] || '正常';
  ctx.state = state; renderChrome(ctx);
  const showBack = back.length > 1 || ctx.query.from;
  const backControl=def.backHref?`<a class="btn sm" href="${esc(def.backHref)}" data-test="back">${icon('back')} ${esc(def.backLabel||'返回')}</a>`:showBack?`<button class="btn sm" data-back data-test="back">${icon('back')} 返回</button>`:'';
  main.innerHTML = `<div class="pagehead">${backControl}<h1>${def.title}</h1>${badge(def.scope || '角色范围')}<span class="spacer"></span>${store.isMock ? ui.stateSwitcher(state) : ''}</div>
    ${ui.notesPanel(def)}<div id="page" data-page="${def.key}"></div>`;
  const el = ui.$('#page');
  const bootstrapWarning=store.get('meta')?.bootstrapWarning;if(bootstrapWarning)el.insertAdjacentHTML('beforebegin',`<div class="banner info">${esc(bootstrapWarning)}</div>`);
  if (!store.isMock) {
    const section = ctx.path.split('/').slice(3).join('/');
    const reads = ctx.path === '/library' ? ['global:groups'] : ctx.path === '/inbox' ? ['global:notifications'] : ctx.path.startsWith('/studio') ? ['global:resources'] : section.startsWith('world') ? [`${ctx.charId}:worlds`] : section.startsWith('life/wardrobe') ? [`${ctx.charId}:wardrobes`] : section.startsWith('media') ? [`${ctx.charId}:media`] : section.startsWith('life/diary') ? [`${ctx.charId}:diaries`] : [];
    const failures = reads.map(key => store.get('meta').surfaces?.[key]).filter(value => value && value.status !== 'ready');
    if (failures.length) {
      el.insertAdjacentHTML('beforebegin',`<div class="banner info" role="status">${failures.map(value => esc(value.message || '部分资料暂未读到')).join('；')}。其他入口仍可使用。 <button class="btn sm" data-read-retry>重新读取</button></div>`);
      main.querySelector('[data-read-retry]').onclick = async () => { try { await real.refreshProduct(); } catch(error) { ui.toast(error.message,'err'); } };
    }
  }
  try {
    if (state === '正常') { const cleanup = def.render(el, ctx); if (typeof cleanup === 'function') disposePage = cleanup; }
    else if (def.renderState && def.renderState(el, ctx, state) !== false) { /* 页面自定义 */ }
    else el.innerHTML = ui.stateView(state, { zone: def.zone, kept: !!keptDrafts[def.key] });
  } catch (e) { console.error(e); el.innerHTML = `<div class="banner err">页面渲染出错：${esc(e.message)}</div>`; }
  renderGuide();
}
function route() {
  // Resolve the initial route before parsing. Replacing an empty URL with
  // /auth does not emit hashchange; parsing /library first leaves main blank.
  if (!location.hash) { history.replaceState(null, '', store.get('session') || store.isMock ? '#/library' : '#/auth'); }
  const { raw, path, query } = parse();
  if (!popping) { if (back[back.length - 1] !== '#' + raw) back.push('#' + raw); if (back.length > 40) back.shift(); sessionStorage.setItem(HKEY, JSON.stringify(back)); }
  popping = false;
  let m = match(path);
  if (!m) m = { def: { key: 'sys.notfound', title: '页面不存在', zone: '总设计', coverage: '未完成', bp: '—', render: el => el.innerHTML = `<div class="placeholder"><div class="card"><div style="font-size:42px">🧭</div><h2>未完成：没有这个页面</h2><p class="muted">路由 <code>${esc(path)}</code> 尚未在任何分区注册。</p><button class="btn" data-back>返回</button></div></div>` }, params: {} };
  const charId = m.params.charId || store.get('ui').charId || '';
  if (m.params.charId && store.get('ui').charId !== charId && store.get('characters', charId)) store.setUi({ charId });
  current = { ...m, ctx: { params: m.params, query, path, charId, def: m.def, navigate, goBack, refresh: () => renderPage(), store, ui } };
  if(m.def.workspace&&store.get('ui').identity!==m.def.workspace)store.setUi({identity:m.def.workspace});
  ui.closeOverlays(); document.querySelectorAll('.popmenu').forEach(x => x.remove());
  if (!store.isMock && path !== '/auth' && !store.get('session')) { navigate('#/auth'); return; }
  // A stale directory is not an authorization decision. The page's scoped API
  // reads the actual character and supplies the real not-found/permission state.
  renderPage(); window.scrollTo(0, 0);
}
export async function mountProduct(host) {
if (!host) return;
let stopped=false, cleanup=null;
async function start(){
if(stopped||!host.isConnected)return;
cleanup?.();cleanup=null;
document.title = store.isMock ? '摇光 · 隔离演示' : '摇光';
if (real) {
  host.innerHTML = '<div class="statebox" role="status">正在验证所有者会话与读取角色目录…</div>';
  try { real.registerProviders(); await real.refreshProduct({initialOnly:true}); } catch (error) { host.innerHTML = `<div class="statebox" role="alert"><h2>服务读取失败</h2><p>${esc(error.message)}</p><button class="btn" data-boot-retry>重新读取</button><a class="btn" href="/">重新打开首页</a></div>`; host.querySelector('[data-boot-retry]').onclick=()=>void start(); return; }
}
if(stopped||!host.isConnected)return;
const disposeShell = shell(host);
if (!store.isMock) { ui.$('#demoBtn').remove(); ui.$('#notesT').closest('label').hidden = true; }
const unsubscribe = store.subscribe(evt => { if (!current || !host.isConnected) return; if (!store.isMock && evt.type === 'snapshot' && ['auth.signin','library.char.create'].includes(current.def.key)) { renderChrome(current.ctx); return; } if (evt.type === 'snapshot') renderChrome(current.ctx); if (evt.type === 'ui') renderChrome(current.ctx); else if (evt.type === 'reset' || (evt.type !== 'conversation' && !current.def.manualRefresh)) renderPage(); else current.def.onStore?.(evt, current.ctx); });
addEventListener('hashchange', route);
route();
if(real&&store.get('session'))void real.refreshProduct().catch(error=>ui.toast(error.message,'err'));
if (store.isMock) window.__app = { navigate, goBack, routes: allRoutes, get current() { return current; }, demoStates };
cleanup = () => { unsubscribe(); disposeShell(); removeEventListener('hashchange', route); disposePage?.(); disposePage = null; current = null; ui.closeOverlays(); };
}
await start();
return()=>{stopped=true;cleanup?.();};
}
