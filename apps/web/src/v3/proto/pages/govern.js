import {renderRealStudioHome,renderRealStudioPreview} from './real-studio.js';
import { providerPage } from './provider-page.js';
import { npcApplyDrawer } from './npc-management.js';
// 创作治理扩展页：登录、角色库、新建向导、创作中心、沙盒、应用、模板、设置、诊断、数据工具
import * as store from '../store.js';
import { esc, icon, badge, modal, toast, objLink, mgmt, nameOf, confirm, requireOwner } from '../ui.js';
import { applyDrawer } from './studio-edit.js';
import { actionDrawer } from './chat.js';
import { PreferenceFields, Tabs, KeyValueList } from '../components/forms.js';
import { PreviewCard, ResourceCard } from '../components/cards.js';
import { statusKind, hm, charStatus } from './common.js';

export function renderAuth(el, ctx) {
  if (!store.isMock) return providerPage(() => import('./real-core.js'), 'renderRealAuth',el,ctx);
  const s = store.get('session'); const tab = ctx.query.tab || '登录';
  el.innerHTML = `<div class="card" style="max-width:420px;margin:0 auto">${s ? `<div class="banner ok" data-test="signed-in">${icon('check')} 已登录：${esc(s.user)}（${s.at}，原型模拟）</div><div class="row" style="margin-top:8px"><a class="btn primary" href="#/library">进入角色库</a><button class="btn" data-out data-test="signout">退出</button></div>` : `
   ${Tabs({ value: tab, items: ['登录', '注册'].map(value => ({ value, label: value, href: `#/auth?tab=${value}` })) })}
   <form class="stack" style="margin-top:12px" data-af><label class="field">邮箱<input class="input" name="email" type="email" value="me@example.com" data-test="auth-email"></label>
   <label class="field">密码<input class="input" name="pw" type="password" value="demo1234"></label>${tab === '注册' ? '<label class="field">怎么称呼你<input class="input" name="nick" value="我"></label>' : ''}
   <button class="btn primary" data-test="auth-submit">${tab}</button><p class="small muted">原型模拟，不连接真实账号，也不会发送邮件。</p></form>`}</div>`;
  el.querySelector('[data-af]')?.addEventListener('submit', e => { e.preventDefault(); const f = e.target; if (!/.+@.+/.test(f.email.value)) return toast('邮箱格式不对', 'err');
    store.update(d => { d.session = { user: f.email.value, at: hm(), mode: tab }; }, { identity: '系统运维', summary: `原型${tab}（模拟）`, target: null }); toast(tab + '成功（模拟）', 'ok'); });
  el.querySelector('[data-out]')?.addEventListener('click', () => store.update(d => { d.session = null; }, { identity: '系统运维', summary: '退出登录（模拟）', target: null }));
}
export function renderLibrary(el, ctx) {
  if (!store.isMock) return providerPage(() => import('./real-core.js'), 'renderRealLibrary',el,ctx);
  const f = ctx.query.s || '全部'; const groups = store.list('groups');
  const counts = s => store.list('characters', c => s === '全部' || c.status === s).length;
  el.innerHTML = `<div class="row" style="margin-bottom:12px"><div class="row grow">${['全部', '正常', '异常', '已暂停', '已退休', '未启动'].filter(s => counts(s)).map(s => `<a class="chip" style="${s === f ? 'background:var(--primary-soft);color:var(--primary-strong);border-color:var(--primary-line)' : ''}" href="#/library?s=${s}">${s} ${counts(s)}</a>`).join('')}</div><a class="btn primary" href="#/library/create" data-test="lib-create">${icon('sparkle')} 新建角色</a></div>
  ${groups.map(g => { const cs = store.list('characters', c => c.groupId === g.id && (f === '全部' || c.status === f)); return `<h3>${esc(g.name)} <span class="small muted">${cs.length}</span></h3><div class="wgrid" style="margin-bottom:16px">${cs.map(c => `<div class="icard" data-test="lib-${c.id}"><a href="#/c/${c.id}/${c.status === '正常' ? 'chat' : 'profile'}"><img src="${c.avatar}" alt=""></a><div class="cap"><a href="#/c/${c.id}/${c.status === '正常' ? 'chat' : 'profile'}"><b>${esc(c.name)}</b></a>${badge(c.status, statusKind(c.status))}<span class="small muted">${esc(c.statusNote || c.bio)}</span>
    <select class="select" data-grp="${c.id}" data-test="grp-${c.id}" ${c.status === '已退休' ? 'disabled title="已退休，只读"' : ''} aria-label="分组">${groups.map(x => `<option value="${x.id}" ${x.id === c.groupId ? 'selected' : ''}>${esc(x.name)}</option>`).join('')}</select></div></div>`).join('') || '<p class="small muted">这一组没有符合筛选的角色</p>'}</div>`; }).join('')}`;
  el.querySelectorAll('[data-grp]').forEach(s => s.onchange = () => store.update(d => { d.characters[s.dataset.grp].groupId = s.value; }, { identity: '所有者修改', summary: `移动 ${nameOf(s.dataset.grp)} 到分组 ${store.get('groups', s.value)?.name || s.value}`, target: s.dataset.grp }));
}
/* library.char.create —— 4 步向导（草稿保存在内存，取消即丢弃） */
let wiz = null;
const TRAITS = ['温柔', '好奇', '元气', '安静', '毒舌', '迷糊', '认真', '浪漫'];
export function renderCreate(el, ctx) {
  if (!store.isMock) return providerPage(() => import('./real-core.js'), 'renderRealCreate',el,ctx);
  if (!wiz) wiz = { step: 0, name: '', bio: '', traits: [], world: '海湾小城（新运行世界）', groupId: 'grp_none' };
  const S = ['基础', '性格', '世界', '确认'];
  const body = [
    `<label class="field">名字<input class="input" name="name" value="${esc(wiz.name)}" placeholder="例如：小满" data-test="wiz-name"></label><label class="field">一句话简介<input class="input" name="bio" value="${esc(wiz.bio)}" placeholder="她是谁、在做什么"></label>`,
    `<p class="small muted">选 1-3 个</p><div class="row">${TRAITS.map(t => `<button type="button" class="chip" data-t="${t}" style="${wiz.traits.includes(t) ? 'background:var(--primary-soft);color:var(--primary-strong);border-color:var(--primary-line)' : ''}">${t}</button>`).join('')}</div>`,
    `<label class="field">运行世界模板<select class="select" name="world">${['海湾小城（新运行世界）', '大学城（新运行世界）', '面包街（新运行世界）'].map(w => `<option ${w === wiz.world ? 'selected' : ''}>${w}</option>`).join('')}</select></label><label class="field">分组<select class="select" name="groupId">${store.list('groups').map(g => `<option value="${g.id}" ${g.id === wiz.groupId ? 'selected' : ''}>${esc(g.name)}</option>`).join('')}</select></label><p class="small muted">每个角色都有自己的运行世界，不会和小遥共用。</p>`,
    `<div class="kv"><dt>名字</dt><dd>${esc(wiz.name)}</dd><dt>简介</dt><dd>${esc(wiz.bio || '—')}</dd><dt>性格</dt><dd>${wiz.traits.map(t => badge(t, 'plain')).join('') || '—'}</dd><dt>世界</dt><dd>${esc(wiz.world)}</dd><dt>分组</dt><dd>${esc(store.get('groups', wiz.groupId)?.name)}</dd></div><p class="small muted">创建后状态为「未启动」，生活不会自动开始。</p>`,
  ][wiz.step];
  el.innerHTML = `<div class="card" style="max-width:640px;margin:0 auto"><div class="wizsteps">${S.map((s, i) => `<span class="${i === wiz.step ? 'on' : ''}">${i + 1}. ${s}</span>`).join('')}</div><form class="stack" data-wf>${body}</form>
   <div class="row" style="margin-top:16px"><button class="btn" data-cancel data-test="wiz-cancel">取消</button><span class="spacer"></span>${wiz.step ? '<button class="btn" data-prev>上一步</button>' : ''}${wiz.step < 3 ? '<button class="btn primary" data-next data-test="wiz-next">下一步</button>' : '<button class="btn accent" data-done data-test="wiz-done">创建角色</button>'}</div></div>`;
  const f = el.querySelector('[data-wf]');
  const keep = () => { ['name', 'bio', 'world', 'groupId'].forEach(k => { if (f[k]) wiz[k] = f[k].value; }); };
  el.querySelectorAll('[data-t]').forEach(b => b.onclick = () => { const t = b.dataset.t; wiz.traits = wiz.traits.includes(t) ? wiz.traits.filter(x => x !== t) : [...wiz.traits, t].slice(-3); renderCreate(el, ctx); });
  el.querySelector('[data-next]')?.addEventListener('click', () => { keep(); if (wiz.step === 0 && !wiz.name.trim()) return toast('请先填写名字', 'err'); wiz.step++; renderCreate(el, ctx); });
  el.querySelector('[data-prev]')?.addEventListener('click', () => { keep(); wiz.step--; renderCreate(el, ctx); });
  el.querySelector('[data-cancel]').onclick = async () => { if (wiz.name && !(await confirm({ title: '放弃新建', text: '向导里填的内容不会保存。', okText: '放弃', identity: '所有者修改' }))) return; wiz = null; ctx.navigate('#/library'); };
  el.querySelector('[data-done]')?.addEventListener('click', async () => { if (!(await requireOwner())) return; const id = store.uid('c'); const v = wiz;
    store.update(d => { d.characters[id] = { id, name: v.name.trim(), avatar: '/v3/assets/avatars/user.svg', status: '未启动', statusNote: '刚创建，运行世界尚未启动', groupId: v.groupId, worldId: null, worldTemplate: v.world, bio: v.bio || '（暂无简介）', traits: v.traits, locationId: null }; }, { identity: '所有者修改', summary: `新建角色 ${v.name}（未启动）`, target: id });
    wiz = null; toast('已创建 ' + v.name, 'ok'); ctx.navigate(`#/c/${id}/profile`); });
}
export function renderStudioHome(el, ctx) {
  if(!store.isMock)return renderRealStudioHome(el,ctx);
  const t = ctx.query.type || '全部'; const rs = store.list('resources', r => t === '全部' || r.type === t);
  el.innerHTML = `<div class="row" style="margin-bottom:12px">${Tabs({ value: t, className: 'tabs grow', items: ['全部', '地点', '物品', '事件模板', 'NPC'].map(value => ({ value, label: value, href: `#/studio?type=${value}` })) })}<button class="btn primary" data-new data-test="res-new">${icon('sparkle')} 新建资源</button></div>
  <div class="grid g3">${rs.map(r => { const pend = Object.values(store.get('instances')).some(i => i[r.id] && i[r.id].applied !== r.published); return ResourceCard(r, { pending: pend }); }).join('')}</div>
  <div class="row" style="margin-top:12px"><a class="btn" href="#/studio/preview">试生成沙盒</a><a class="btn" href="#/studio/apply">应用到实例</a><a class="btn" href="#/studio/template">事件模板</a></div>`;
  el.querySelector('[data-new]').onclick = () => { const m = modal({ title: '新建资源 ' + badge('全局定义'), body: `<form class="stack" data-nf><label class="field">名称<input class="input" name="name" value="街角面包房" data-test="res-name"></label><label class="field">类型<select class="select" name="type"><option>地点</option><option>物品</option><option>事件模板</option><option>NPC</option></select></label><p class="small muted">新资源从 v1 开始；不会自动加入任何角色的世界。</p></form>`, footer: '<button class="btn" data-c>取消</button><button class="btn primary" data-ok data-test="res-create">创建</button>' });
    m.el.querySelector('[data-c]').onclick = () => m.close();
    m.el.querySelector('[data-ok]').onclick = async () => { const f = m.el.querySelector('[data-nf]'); if (!f.name.value.trim()) return; if (f.type.value === 'NPC') { m.close(); ctx.navigate(`#/c/${store.get('ui').charId}/people/npc/new?name=${encodeURIComponent(f.name.value.trim())}`); return; } if (!(await requireOwner())) return; const id = store.uid('r_');
      store.update(d => { d.resources[id] = { id, type: f.type.value, name: f.name.value.trim(), published: 'v1', draft: null, versions: { v1: { 名称: f.name.value.trim(), 描述: '', 营业状态: '—', 标签: '' } } }; }, { identity: '所有者修改', summary: '新建资源 ' + f.name.value, target: id });
      m.close(); ctx.navigate('#/studio/edit/' + id); }; };
}
export function renderPreview(el, ctx) {
  if(!store.isMock)return renderRealStudioPreview(el,ctx);
  const rid = ctx.query.res || 'r_book'; const r = store.get('resources', rid) || store.get('resources', 'r_book');
  const drafts = store.get('studioDrafts');
  el.innerHTML = `<div class="grid g2" style="align-items:start"><form class="card sandbox" data-pf><div class="row"><b class="grow">试生成沙盒</b>${badge('创作预览·不进入生活')}</div><p class="small muted">结果只存在创作资源里，不进入任何角色的生活、媒体库、日记或记忆。</p>
   <label class="field">资源<select class="select" name="res">${store.list('resources').map(x => `<option value="${x.id}" ${x.id === r.id ? 'selected' : ''}>${esc(x.name)}（${x.type}）</option>`).join('')}</select></label>
   <label class="field">想看什么<input class="input" name="prompt" value="傍晚的${esc(r.name)}" data-test="pv-prompt"></label>
   <div class="row" style="margin-top:8px"><button type="reset" class="btn">重置</button><button class="btn accent" data-test="pv-gen">${icon('sparkle')} 试生成</button></div>
   <p class="small muted" data-test="pv-media-count">媒体库条目：${store.list('media').length}（试生成不会增加）</p></form>
  <div class="stack" data-test="pv-list">${drafts.slice().reverse().map(x => PreviewCard(x, { resourceName: nameOf(x.resourceId), includeResource: true })).join('') || '<div class="statebox"><h2>还没有试生成结果</h2></div>'}</div></div>`;
  el.querySelector('[data-pf]').onsubmit = e => { e.preventDefault(); const f = e.target; const rr = store.get('resources', f.res.value); const v = rr.draft || rr.versions[rr.published]; const id = 'sd_' + Date.now().toString(36);
    const img = store.list('places', p => p.resourceId === rr.id)[0]?.scene || '/v3/assets/scenes/shop.svg';
    store.update(d => { d.studioDrafts.push({ id, resourceId: rr.id, at: hm(), img, text: `【试生成】${f.prompt.value}：${v.描述 || v.名称}` }); }, { identity: '所有者修改', summary: '创作预览试生成（不进入生活）', target: rr.id }); };
  el.querySelectorAll('[data-del]').forEach(b => b.onclick = () => store.update(d => { d.studioDrafts = d.studioDrafts.filter(x => x.id !== b.dataset.del); }, { identity: '所有者修改', summary: '丢弃创作预览 ' + b.dataset.del, target: null }));
}
export function renderApply(el, ctx) {
  const rows = []; Object.entries(store.get('instances')).forEach(([cid, m]) => Object.entries(m).forEach(([rid, i]) => rows.push({ cid, rid, i, r: store.get('resources', rid) })));
  el.innerHTML = `<p class="small muted">${badge('全局定义')} → ${badge('角色范围')} 发布新版本不会自动改变运行中的角色，需要在这里逐个应用。</p>
  <div class="card"><div class="list">${rows.map(({ cid, rid, i, r }) => `<div class="lrow" data-test="apply-row-${rid}"><img class="av sm" src="${store.get('characters', cid).avatar}" alt=""><span class="grow"><b>${esc(r.name)}</b> → ${esc(nameOf(cid))}<div class="small muted">运行 ${i.applied} · 最新 ${r.published}${Object.keys(i.overrides || {}).length ? ' · 有实例修正' : ''}</div></span>${i.applied !== r.published ? `<button class="btn teal sm" data-apply="${rid}" data-c="${cid}" data-test="apply-open-${rid}">应用…</button>` : badge('已确认')}</div>`).join('') || '<p class="small muted">当前范围未找到实例引用</p>'}</div></div>`;
  el.querySelectorAll('[data-apply]').forEach(b => b.onclick = () => store.get('resources', b.dataset.apply).type === 'NPC' ? npcApplyDrawer(b.dataset.apply, b.dataset.c) : applyDrawer(b.dataset.apply, b.dataset.c));
}
export function renderTemplate(el, ctx) {
  if(!store.isMock)return renderRealStudioHome(el,{...ctx,query:{...ctx.query,type:'事件模板'}});
  const tpls = store.list('resources', r => r.type === '事件模板');
  el.innerHTML = tpls.map(r => { const v = r.versions[r.published]; return `<div class="card" style="margin-bottom:12px"><div class="row"><h3 class="grow" style="margin:0">${icon('sparkle')} ${esc(r.name)}</h3>${badge('全局定义')}<span class="small muted">${r.published}</span></div><p>${esc(v.描述)}</p>
    <div class="banner warn small">${icon('warn')} 所有事件模板都受系统规则约束：不得产生永久伤害（致残、死亡、永久失去重要关系），该规则不可在模板里关闭。</div>
    <div class="small">已触发的实例：${store.list('events', e => /封路/.test(e.title) && r.id === 'r_tpl_close').map(e => objLink(e.id)).join('、') || '—'}</div>
    <div class="row" style="margin-top:8px"><a class="btn" href="#/studio/edit/${r.id}">编辑模板</a><a class="btn accent" href="#/studio/preview?res=${r.id}">试生成</a></div></div>`; }).join('');
}
/* settings.global / settings.char.autonomy —— 仅展示：保存只记录偏好，不改变运行时权限 */
const PERMS = [['发起行动（购物、出门）', '需你发出行动请求'], ['修改自己的记忆', '可以（来自经历与交流）'], ['改动人格定义', '不可以，需所有者采纳提案'], ['发动态', '可以，以她自己的名义'], ['永久伤害类事件', '系统禁止，不可开启']];
function prefForm(el, obj, onSave, testKey) {
  el.querySelector('[data-prf]').onsubmit = e => { e.preventDefault(); const f = e.target; const v = {}; Object.keys(obj).filter(k => k !== 'savedAt').forEach(k => { v[k] = typeof obj[k] === 'boolean' ? f[k].checked : f[k].value; }); onSave(v); };
}
const fieldsOf = PreferenceFields;
export function renderSettings(el, ctx) {
  const g = store.get('prefs').global;
  el.innerHTML = `<div class="banner info">${icon('info')} 仅展示：这里的设置只记录为偏好；原型不会因此改变任何运行时权限或行为。</div>
  <div class="grid g2" style="margin-top:12px;align-items:start"><form class="card" data-prf><h3>${icon('settings')} 偏好</h3><div class="stack">${fieldsOf(g)}</div><div class="row" style="margin-top:12px"><button type="reset" class="btn" data-test="set-cancel">撤销更改</button><button class="btn primary" data-test="set-save">记录偏好</button></div><p class="small muted" data-test="set-saved">${g.savedAt ? '上次记录 ' + g.savedAt : ''}</p></form>
  <div class="card"><h3>账号</h3><p class="small">${store.get('session') ? '已登录（模拟）：' + esc(store.get('session').user) : '未登录（原型）'}</p><a class="btn" href="#/auth">登录/注册</a><h3 style="margin-top:12px">其他</h3><div class="list"><a class="lrow" href="#/c/c1/settings/autonomy">角色自治与触达</a><a class="lrow" href="#/c/c1/world/settings">随机事件设定</a><a class="lrow" href="#/data">数据工具</a></div></div></div>`;
  prefForm(el, g, v => store.update(d => { Object.assign(d.prefs.global, v, { savedAt: hm() }); }, { identity: '所有者修改', summary: '记录系统偏好（仅展示，不改变运行时权限）', target: null }));
}
export function renderAutonomy(el, ctx) {
  if (!store.isMock) return providerPage(() => import('./real-autonomy.js'), 'renderRealAutonomy',el,ctx);
  const c = store.get('characters', ctx.charId); const p = store.get('prefs').autonomy[c.id] || { 自治程度: '自主（重要事项告知我）', 主动触达: '适中', 免打扰: '23:00-08:00', 允许发动态: true };
  const head = document.createElement('div'); const top = c.id !== 'c1' && charStatus(head, ctx, '自治设置') ? head.innerHTML : '';
  el.innerHTML = `${top}<div class="banner info" style="margin-top:${top ? 12 : 0}px">${icon('info')} 仅展示：下面的「当前权限」是只读说明；偏好表单只做记录，不改变运行时权限。</div>
  <div class="grid g2" style="margin-top:12px;align-items:start"><form class="card" data-prf><h3>${esc(c.name)}的自治偏好</h3><div class="stack">${fieldsOf(p)}</div><div class="row" style="margin-top:12px"><button type="reset" class="btn">撤销更改</button><button class="btn primary" data-test="auto-save" ${c.status === '已退休' ? 'disabled' : ''}>记录偏好</button></div><p class="small muted" data-test="auto-saved">${p.savedAt ? '上次记录 ' + p.savedAt + '（未改变运行时权限）' : ''}</p></form>
  <div class="card"><h3>当前权限（只读）</h3>${PERMS.map(([a, b]) => `<div class="lrow small"><span class="grow">${a}</span><span class="muted">${b}</span></div>`).join('')}<a class="small" href="#/settings/outreach">主动触达设置 →</a></div></div>`;
  prefForm(el, p, v => store.update(d => { d.prefs.autonomy[c.id] = { ...v, savedAt: hm() }; }, { identity: '所有者修改', summary: `记录 ${c.name} 自治偏好（仅展示，不改变运行时权限）`, target: c.id }));
}
/* diag */
export function renderDiag(el, ctx) {
  if (!store.isMock) return providerPage(() => import('./real-operations.js'), 'renderRealDiag',el,ctx);
  const a12 = store.get('actions', 'a12'); const t88 = store.get('tasks', 't88');
  el.innerHTML = `<div class="grid g3"><a class="card tint-primary" href="#/diag/task/t88" data-test="diag-t88"><h3>${icon('warn')} 任务：取花束</h3><p class="small">${badge(t88.state)} 执行 ${t88.runs} 次</p></a>
   <div class="card tint-sun"><h3>${icon('bag')} 行动：去花店取花束</h3><p class="small">${badge(a12.state)} ${esc(a12.reason || '')}</p><button class="btn sm" data-a12 data-test="diag-a12">行动详情</button></div>
   <div class="card ${store.list('characters', c => c.status === '异常').length ? 'tint-accent' : ''}"><h3>角色状态</h3>${store.list('characters').map(c => `<div class="lrow small">${objLink(c.id)}<span class="grow"></span>${badge(c.status, statusKind(c.status))}</div>`).join('')}</div></div>
  <div class="grid g3" style="margin-top:12px"><a class="card" href="#/diag/audit"><h3>${icon('diag')} 操作审计</h3><p>${store.auditLog().length} 条</p></a><a class="card" href="#/diag/pending"><h3>待处理事项</h3><p>${store.pending('c1').length} 项</p></a>
   <a class="card" href="#/c/c1/world/settings"><h3>永久伤害拦截</h3><p>${store.list('characters').filter(c => c.harmIntercepts).map(c => `${esc(c.name)} <b>${c.harmIntercepts}</b> 次`).join('')}</p></a></div>
  <div class="row" style="margin-top:12px"><a class="btn" href="#/data">数据工具</a></div>${mgmt('task t88 ↔ action a12 · page key diag.home')}`;
  el.querySelector('[data-a12]').onclick = () => actionDrawer('a12');
}
export function renderAudit(el, ctx) {
 if(!store.isMock)return providerPage(()=>import("./real-diagnostics.js"),"renderRealDiagnosticRecords",el,{...ctx,def:{...ctx.def,mockSection:"audit"}});
  const who = ctx.query.who || '全部'; const a = store.auditLog().filter(x => who === '全部' || x.identity === who);
  el.innerHTML = `${Tabs({ value: who, attrs: { style: 'margin-bottom:12px' }, items: ['全部', '交流', '行动请求', '所有者修改', '系统运维'].map(value => ({ value, label: value, href: `#/diag/audit?who=${value}` })) })}
  <div class="card">${a.length ? a.map(x => `<div class="lrow small" data-test="audit-row">${badge(x.identity)}<span class="grow">${esc(x.summary)}</span>${x.target ? objLink(x.target, nameOf(x.target)) : ''}<span class="muted">${x.at}</span></div>`).join('') : `<p class="muted">当前范围（${esc(who)}）未找到审计记录</p>`}</div>`;
}
export function renderPending(el, ctx) {
  if (!store.isMock) return providerPage(() => import('./real-operations.js'), 'renderRealPending',el,ctx);
  el.innerHTML = `<p class="small muted">待处理事项从现有对象派生（提案、待应用定义、失败任务），处理掉就会自动消失。</p><div class="card">${store.pending('c1').map(x => `<div class="lrow" data-test="pending-${x.id}"><span class="grow">${esc(x.text)}</span><a class="btn sm primary" href="${x.link}">去处理</a></div>`).join('') || '<p class="muted">当前范围未找到待处理事项</p>'}</div>`;
}
/* data.tools */
const CFG_COLS = ['resources', 'groups', 'prefs', 'worldSettings'];
export function renderData(el, ctx) {
 if(!store.isMock)return providerPage(()=>import('./real-data-tools.js'),'renderRealData',el,ctx);
  el.innerHTML = `<div class="grid g3" style="align-items:start">
  <div class="card"><h3>${icon('inbox')} 配置包 ${badge('全局定义')}</h3><p class="small muted">导出/导入创作定义与设置（资源、分组、偏好、世界设定）。不含聊天、记忆等生活数据。</p>
   <button class="btn primary" data-exp data-test="cfg-export">导出配置包</button><div data-expout></div>
   <h3 style="margin-top:12px">导入</h3><textarea class="input" rows="4" data-imp placeholder="粘贴配置包 JSON" data-test="cfg-input"></textarea>
   <div class="row" style="margin-top:6px"><button class="btn" data-clr>清空</button><button class="btn" data-chk data-test="cfg-check">校验</button></div><div data-chkout></div></div>
  <div class="card"><h3>${icon('diag')} 业务数据备份 ${badge('未完成', 'cov-未完成')}</h3><p class="small">P2：聊天、记忆、日记等生活数据的备份与恢复<b>还没做</b>。</p><button class="btn" disabled>创建备份（未完成）</button><p class="small muted">原型的「重置演示数据」只恢复种子数据，不是备份。</p></div>
  <div class="card"><h3>${icon('reset')} 对象修订与回档</h3><a class="lrow" href="#/data/revisions" data-test="to-revisions">对象修订记录 →</a><a class="lrow" href="#/data/rollback">时间线回档说明（P3）→</a></div></div>`;
  const q = s => el.querySelector(s);
  q('[data-exp]').onclick = () => { const d = store.get(); const pkg = { kind: 'fluctlight-config', version: 1, exportedAt: hm(), ...Object.fromEntries(CFG_COLS.map(c => [c, d[c]])) };
    const txt = JSON.stringify(pkg, null, 2); const url = URL.createObjectURL(new Blob([txt], { type: 'application/json' }));
    q('[data-expout]').innerHTML = `<div class="banner ok small" data-test="cfg-exported">${icon('check')} 已生成：${Object.keys(pkg.resources).length} 个资源 · ${(txt.length / 1024).toFixed(1)} KB <a download="摇光配置包.json" href="${url}">下载</a></div><details class="small"><summary>预览</summary><pre style="max-height:160px;overflow:auto">${esc(txt.slice(0, 1200))}…</pre></details>`;
    q('[data-imp]').value = txt; store.update(() => {}, { identity: '系统运维', summary: '导出配置包', target: null, silent: true }); };
  q('[data-clr]').onclick = () => { q('[data-imp]').value = ''; q('[data-chkout]').innerHTML = ''; };
  q('[data-chk]').onclick = () => { const out = q('[data-chkout]'); let pkg; try { pkg = JSON.parse(q('[data-imp]').value); } catch (e) { out.innerHTML = `<div class="banner err small" data-test="cfg-bad">${icon('warn')} 不是有效的 JSON</div>`; return; }
    if (pkg.kind !== 'fluctlight-config') { out.innerHTML = '<div class="banner err small" data-test="cfg-bad">不是摇光配置包（缺少 kind）</div>'; return; }
    const res = Object.values(pkg.resources || {}); const changed = res.filter(r => JSON.stringify(store.get('resources', r.id)) !== JSON.stringify(r));
    const harm = pkg.worldSettings?.w1 && pkg.worldSettings.w1.harmLocked === false;
    out.innerHTML = `<div class="banner info small" data-test="cfg-ok">校验通过：${res.length} 个资源，其中 ${changed.length} 个与当前不同。${harm ? '<br>! 包内试图开启永久伤害类事件：已忽略（不可绕过）。' : ''}</div><p class="small muted">导入只会写成各资源的<b>草稿</b>，不发布、不应用到实例。</p><div class="row"><button class="btn" data-cx>取消</button><button class="btn accent" data-go data-test="cfg-import">导入为草稿</button></div>`;
    out.querySelector('[data-cx]').onclick = () => { out.innerHTML = ''; toast('已取消导入'); };
    out.querySelector('[data-go]').onclick = async () => { if (!(await requireOwner())) return; if (!(await confirm({ title: '导入配置包', text: `${res.length} 个资源写为草稿。永久伤害限制保持不变。` }))) return;
      store.update(d => { res.forEach(r => { if (d.resources[r.id]) d.resources[r.id].draft = { ...r.versions[r.published] }; else d.resources[r.id] = { ...r, draft: null }; }); d.worldSettings.w1.harmLocked = true; }, { identity: '所有者修改', summary: `导入配置包：${res.length} 个资源写为草稿`, target: null }); toast('已导入为草稿', 'ok'); }; };
}
export function renderRevisions(el, ctx) {
 if(!store.isMock)return providerPage(()=>import('./real-data-tools.js'),'renderRealRevisions',el,ctx);
  const rows = [
    ...store.list('memories').map(m => ({ name: m.content, kind: '记忆', n: m.revisions.length, href: `#/c/${m.charId}/growth/memory/${m.id}`, last: m.revisions[m.revisions.length - 1] })),
    ...store.list('relations').map(r => ({ name: '小遥 ↔ ' + nameOf(r.npcId), kind: '关系', n: r.revisions.length, href: `#/c/${r.charId}/people/npc/${r.npcId}`, last: r.revisions[r.revisions.length - 1] })),
    ...store.list('resources').map(r => ({ name: r.name, kind: '定义', n: Object.keys(r.versions).length, href: `#/studio/${r.type === 'NPC' ? 'npc' : 'edit'}/${r.id}`, last: { text: '已发布 ' + r.published, by: '所有者修改' } })),
  ];
  el.innerHTML = `<p class="small muted">每个对象保留自己的修订历史；这和整条时间线回档（P3，未做）不是一回事。</p><div class="card"><div class="list">${rows.map(r => `<a class="lrow click" href="${r.href}" data-test="revrow">${badge(r.kind, 'plain')}<span class="grow">${esc(r.name)}<div class="small muted">最近：${esc(r.last.text)}</div></span><span class="small muted">${r.n} 版</span></a>`).join('')}</div></div>`;
}
export function renderRollback(el,ctx) {
 if(!store.isMock)return providerPage(()=>import('./real-data-tools.js'),'renderRealRollback',el,ctx);
  el.innerHTML = `<div class="card" style="max-width:720px"><h2>${icon('reset')} 时间线回档（P3 · 未实现）</h2><p>回档是指把某个角色的整个运行世界退回到过去某一刻（之后的聊天、记忆、事件都会消失）。它影响面大，排在 P3，<b>原型里没有这个功能</b>。</p>
   <ul class="small"><li>「重置演示数据」只是把原型恢复成种子数据，<b>不是</b>回档。</li><li>想撤回单个错误，用对象的修订记录（记忆、关系、定义）。</li><li>想保留副本，未来会用「业务数据备份」（P2，也未完成）。</li></ul>
   <div class="row"><a class="btn" href="#/data/revisions">查看对象修订</a><a class="btn" href="#/data">回数据工具</a></div></div>`;
}
