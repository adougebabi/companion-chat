// 世界人物扩展页：世界、地点、场景、事件、随机事件设定、人物、媒体
import * as store from '../store.js';
import { esc, icon, badge, modal, toast, objLink, mgmt, nameOf, confirm, requireOwner } from '../ui.js';
import { genImg } from './chat.js';
import { photoModal } from './wardrobe.js';
import { suggestScheduleDrawer } from './life-extra.js';
import { hm } from './common.js';

const here = () => store.get('characters', 'c1').locationId;
export function renderMap(el, ctx) {
  el.innerHTML = `<div class="banner info small">${icon('globe')} 海湾小城 · 你在远程浏览，点开地点不会让小遥移动。</div>
  <div class="wgrid" style="margin-top:12px" data-test="map">${store.list('places').map(p => `<a class="icard" href="#/c/${ctx.charId}/world/place/${p.id}" data-test="place-${p.id}"><img src="${p.scene}" alt=""><div class="cap"><b>${esc(p.name)}</b>${here() === p.id ? '<span class="badge st-穿着">小遥在这里</span>' : ''}<span class="small muted">${store.list('npcs', n => n.placeId === p.id).map(n => esc(n.name)).join('、')}</span></div></a>`).join('')}</div>
  <div class="card" style="margin-top:16px"><h3>${icon('warn')} 最近的世界变化</h3>${store.list('events').map(e => `<div class="lrow"><span class="grow">${objLink(e.id)} <span class="small muted">${esc(nameOf(e.placeId))}</span></span><span class="small muted">${e.at}</span>${badge(e.state)}</div>`).join('')}</div>
  <div class="row" style="margin-top:12px"><a class="btn" href="#/c/${ctx.charId}/world/scene">她身边的场景</a><a class="btn" href="#/c/${ctx.charId}/world/events">事件流</a><a class="btn ghost" href="#/c/${ctx.charId}/world/settings">随机事件设定</a></div>`;
}
export function renderPlace(el, ctx) {
  const p = store.get('places', ctx.params.placeId);
  if (!p) { el.innerHTML = '<div class="statebox"><h2>当前范围未找到该地点</h2></div>'; return; }
  const isHere = here() === p.id;
  el.innerHTML = `<section class="hero"><img class="scene" src="${p.scene}" alt=""><div class="over"><div><h2>${esc(p.name)}</h2><p class="small">${esc(p.desc)}</p></div></div></section>
  <div class="banner ${isHere ? 'ok' : 'info'}" style="margin-top:12px" data-test="place-loc">${icon('pin')} ${isHere ? '小遥现在就在这里。' : `你在远程看${esc(p.name)}；小遥仍在 <b data-test="char-loc">${esc(nameOf(here()))}</b>，浏览不会移动她。`}</div>
  <div class="grid g3" style="margin-top:12px"><div class="card"><h3>这里的人</h3>${store.list('npcs', n => n.placeId === p.id).map(n => `<div class="lrow"><img class="av sm" src="${n.avatar}" alt="">${objLink(n.id)}<span class="small muted">${esc(n.role)}</span></div>`).join('') || '<p class="small muted">暂无常驻人物</p>'}</div>
   <div class="card"><h3>她在这里的日程</h3>${store.list('schedule', s => s.placeId === p.id).map(s => `<div class="lrow small">${esc(s.time)} ${esc(s.title)} ${badge(s.state)}</div>`).join('') || '<p class="small muted">当前范围未找到</p>'}</div>
   <div class="card"><h3>发生过</h3>${store.list('events', e => e.placeId === p.id).map(e => `<div class="lrow small">${objLink(e.id)} ${badge(e.state)}</div>`).join('') || '<p class="small muted">当前范围未找到</p>'}</div></div>
  <div class="row" style="margin-top:12px"><button class="btn primary" data-sug data-test="place-suggest">${icon('calendar')} 建议她来这里</button><a class="btn" href="#/c/c1/chat?say=${encodeURIComponent(p.name + '最近怎么样？')}">在对话里问问</a></div>
  ${mgmt(`${p.id}${p.resourceId ? ' · 定义 ' + objLink(p.resourceId, p.resourceId) : ''} · page key world.place.detail`)}`;
  el.querySelector('[data-sug]').onclick = () => suggestScheduleDrawer(p.id);
}
export function renderScene(el, ctx) {
  const c = store.get('characters', 'c1'); const p = store.get('places', c.locationId);
  el.innerHTML = `<section class="hero"><img class="scene" src="${p.scene}" alt=""><div class="over"><img class="av lg" src="${c.avatar}" alt=""><div class="grow"><div class="small muted">远程查看 · 你不在场</div><h2 style="margin:2px 0">${esc(p.name)}</h2><p class="small">${esc(c.doing)}</p></div></div></section>
  <div class="grid g2" style="margin-top:12px"><div class="card"><h3>周围</h3>${store.list('npcs', n => n.placeId === p.id).map(n => `<div class="lrow"><img class="av sm" src="${n.avatar}" alt="">${objLink(n.id)}<span class="small muted">${esc(n.role)}</span></div>`).join('') || '<p class="small muted">附近没有熟人</p>'}<p class="small muted">${esc(p.desc)}</p></div>
   <div class="card"><h3>${icon('camera')} 看看她</h3><p class="small muted">生成一张此刻的画面，${badge('生成图·非事实')}，不改变任何事实。没有“进入场景”。</p><button class="btn primary" data-photo data-test="scene-photo">${icon('camera')} 拍一张</button></div></div>`;
  el.querySelector('[data-photo]').onclick = () => photoModal(null);
}
export function renderEvents(el, ctx) {
  el.innerHTML = `<div class="card chain">${store.list('events').sort((a, b) => b.at > a.at ? 1 : -1).map(e => `<a class="lrow click" href="#/c/${ctx.charId}/world/event/${e.id}" data-test="ev-${e.id}"><b class="small" style="width:90px">${e.at}</b><span class="grow">${esc(e.title)}<div class="small muted">${esc(nameOf(e.placeId))} · 影响 ${e.impacts.length} 处</div></span>${badge(e.state)}</a>`).join('')}</div>`;
}
const TYPE = { schedule: '日程', diary: '日记', memory: '记忆', relation: '关系', item: '物品' };
export function renderEvent(el, ctx) {
  const e = store.get('events', ctx.params.evId);
  if (!e) { el.innerHTML = '<div class="statebox"><h2>当前范围未找到该事件</h2></div>'; return; }
  el.innerHTML = `<div class="card"><div class="row"><h2 class="grow" style="margin:0">${esc(e.title)}</h2>${badge(e.state)}</div><p class="small">${e.at} · ${objLink(e.placeId)}</p>${mgmt(`${e.id} · page key world.event.detail`)}</div>
  <div class="card" style="margin-top:12px"><h3>影响链 <span class="small muted">点一项看现在的状态</span></h3><div class="chain">${e.impacts.map((i, k) => { const f = store.find(i.id); const o = f?.obj || {};
    return `<div data-test="impact"><button class="lrow click" style="width:100%;text-align:left;background:none;border:0" data-imp="${k}" data-test="impact-open-${k}">${badge(TYPE[i.type] || i.type, 'plain')}<span class="grow">${esc(i.text)}</span>${icon('more')}</button>
    <div class="card tint-sun small" data-impd="${k}" hidden data-test="impact-detail-${k}">${objLink(i.id)} · 现在：${o.state ? badge(o.state) : o.status ? `<span class="badge st-${o.status}">${o.status}</span>` : o.stage ? badge(o.stage, 'plain') : '已记录'}${o.note ? ' · ' + esc(o.note) : ''}</div></div>`; }).join('')}</div></div>
  <div class="row" style="margin-top:12px"><a class="btn" href="#/c/${ctx.charId}/world/events">全部事件</a><a class="btn" href="#/c/c1/chat?say=${encodeURIComponent('封路那天还好吗？')}">在对话里问问她</a></div>`;
  el.querySelectorAll('[data-imp]').forEach(b => b.onclick = () => { const d = el.querySelector(`[data-impd="${b.dataset.imp}"]`); d.hidden = !d.hidden; });
}
/* world.instance.settings —— 随机事件设定；永久伤害不可开启、不可绕过 */
export function renderWorldSettings(el, ctx) {
  const ws = store.get('worldSettings').w1; const c = store.get('characters', 'c1');
  if (!ws || store.get('meta')?.unwired?.harm) {
    el.innerHTML = `<div class="card"><h3>${icon('warn')} 安全拦截</h3><p data-test="harm-count">永久伤害拦截计数尚未接入</p><p class="small muted">缺计数时不显示 0。</p></div>`;
    return;
  }
  el.innerHTML = `<div class="grid g2" style="align-items:start"><form class="card" data-wf><h3>${icon('sparkle')} 随机事件</h3>
   <label class="toggle"><input type="checkbox" name="randomEvents" ${ws.randomEvents ? 'checked' : ''} data-test="rand-on"> 允许随机事件</label>
   <label class="field" style="margin-top:8px">频率<select class="select" name="freq" data-test="rand-freq">${['很少', '适中', '较多'].map(v => `<option ${v === ws.freq ? 'selected' : ''}>${v}</option>`).join('')}</select></label>
   <h3 style="margin-top:12px">类别</h3>${Object.entries(ws.cats).map(([k, v]) => `<label class="toggle" style="margin-right:12px"><input type="checkbox" name="cat_${k}" ${v ? 'checked' : ''}> ${k}</label>`).join('')}
   <div class="lockrow" style="margin-top:12px" data-test="harm-row"><label class="toggle"><input type="checkbox" disabled data-test="harm-toggle"> 永久伤害类事件（致残、死亡、永久失去重要关系）</label><div class="small">${icon('warn')} 系统强制关闭，任何设定、模板或导入都无法开启。<button type="button" class="btn ghost sm" data-why>为什么？</button></div></div>
   <div class="row" style="margin-top:12px"><button type="reset" class="btn" data-test="ws-cancel">撤销更改</button><button class="btn primary" data-test="ws-save">保存设定</button></div>${mgmt(`world w1 · page key world.instance.settings${ws.savedAt ? ' · 上次保存 ' + ws.savedAt : ''}`)}</form>
  <div class="card tint-sun"><h3>${icon('warn')} 安全拦截</h3><div class="row"><div class="grow"><div style="font-size:28px;font-weight:700" data-test="harm-count">${ws.intercepts.length}</div><div class="small muted">本世界拦截的事件</div></div><div class="grow"><div style="font-size:28px;font-weight:700" data-test="harm-count-char">${typeof c.harmIntercepts === 'number' ? c.harmIntercepts : '尚未接入'}</div><div class="small muted">小遥身上避免的伤害</div></div></div>
   ${ws.intercepts.map(i => `<div class="lrow small" data-test="intercept"><span class="grow">${esc(i.text)}<div class="muted">规则：${esc(i.rule)}</div></span><span class="muted">${i.at}</span>${badge('取消')}</div>`).join('')}<a class="small" href="#/diag">运行诊断 →</a></div></div>`;
  const f = el.querySelector('[data-wf]');
  f.querySelector('[data-why]').onclick = () => { const m = modal({ title: '永久伤害事件不可开启', body: '<p>摇光的随机事件可以带来小波折，但不会让角色承受不可逆的伤害。这条规则在运行时强制执行，不受设定、事件模板或配置包导入影响，所以这里没有开关。</p>', footer: '<button class="btn primary" data-ok>知道了</button>' }); m.el.querySelector('[data-ok]').onclick = () => m.close(); };
  f.addEventListener('reset', () => setTimeout(() => toast('已撤销未保存的更改'), 0));
  f.onsubmit = async e => { e.preventDefault(); if (!(await requireOwner())) return;
    const v = { randomEvents: f.randomEvents.checked, freq: f.freq.value, cats: Object.fromEntries(Object.keys(ws.cats).map(k => [k, f['cat_' + k].checked])) };
    if (!(await confirm({ title: '保存随机事件设定', text: `随机事件 ${v.randomEvents ? '开' : '关'} · 频率 ${v.freq}。永久伤害类保持关闭。` }))) return;
    store.update(d => { Object.assign(d.worldSettings.w1, v, { harmLocked: true, savedAt: hm() }); }, { identity: '所有者修改', summary: `保存随机事件设定（${v.freq}，永久伤害保持禁用）`, target: null }); toast('已保存', 'ok'); };
}
/* 人物 */
export function renderGraph(el, ctx) {
  el.innerHTML = `<div class="grid g2">${store.list('relations', r => r.charId === 'c1').map(r => { const n = store.get('npcs', r.npcId); return `<a class="card" style="display:block" href="#/c/${ctx.charId}/people/npc/${n.id}" data-test="rel-${r.id}"><div class="row"><img class="av lg" src="${n.avatar}" alt=""><div class="grow"><h3 style="margin:0">${esc(n.name)}</h3><span class="small muted">${esc(n.role)}</span></div>${badge(r.stage, 'plain')}</div>
    <div class="small" style="margin-top:6px">亲密度 ${r.intimacy.value} ${badge('推测')} · 最近：${esc(r.revisions[r.revisions.length - 1].text)}</div></a>`; }).join('')}</div>
  <div class="row" style="margin-top:12px"><a class="btn" href="#/c/${ctx.charId}/people/list">人物目录</a><a class="btn" href="#/c/${ctx.charId}/people/owner">我的背景</a></div>`;
}
export function renderPeopleList(el, ctx) {
  const kw = ctx.query.q || '';
  const ns = store.list('npcs', n => !kw || n.name.includes(kw) || n.role.includes(kw));
  el.innerHTML = `<form class="row" data-pf style="margin-bottom:12px"><input class="input grow" name="q" value="${esc(kw)}" placeholder="找人（名字或身份）" data-test="people-q"><button class="btn">搜索</button></form>
  <div class="card"><div class="list"><a class="lrow click" href="#/c/${ctx.charId}/people/owner"><img class="av sm" src="/v3/assets/avatars/user.svg" alt=""><b>我</b><span class="grow small muted">所有者</span></a>
   ${ns.map(n => { const r = store.list('relations', x => x.npcId === n.id)[0]; return `<a class="lrow click" href="#/c/${ctx.charId}/people/npc/${n.id}" data-test="npc-${n.id}"><img class="av sm" src="${n.avatar}" alt=""><b>${esc(n.name)}</b><span class="grow small muted">${esc(n.role)} · ${esc(nameOf(n.placeId))}</span>${r ? badge(r.stage, 'plain') : '<span class="small muted">未建立关系</span>'}</a>`; }).join('')}</div></div>`;
  el.querySelector('[data-pf]').onsubmit = e => { e.preventDefault(); ctx.navigate(`#/c/${ctx.charId}/people/list?q=${encodeURIComponent(e.target.q.value)}`); };
}
export function renderNpc(el, ctx) {
  const n = store.get('npcs', ctx.params.npcId);
  if (!n) { el.innerHTML = '<div class="statebox"><h2>当前范围未找到该人物</h2></div>'; return; }
  const r = store.list('relations', x => x.npcId === n.id)[0];
  const mems = store.list('memories', m => m.relatedNpc === n.id || (n.id === 'n_alan' && m.id === 'm33'));
  const items = store.list('items', i => i.owner === n.id);
  el.innerHTML = `<div class="grid g2" style="align-items:start"><div class="card"><div class="row"><img class="av lg" src="${n.avatar}" alt=""><div class="grow"><h2 style="margin:0">${esc(n.name)}</h2><div class="small muted">${esc(n.role)} · ${objLink(n.placeId)}</div></div></div><p>${esc(n.bio || '')}</p>
    <div class="banner info small" data-test="npc-not-full">${icon('info')} ${esc(n.name)}是小城里的 NPC，不是完整的摇光：默认没有自己的记忆库、日程和自治，只在和小遥相关时出现。</div>
    <h3 style="margin-top:12px">和小遥有关</h3>${mems.map(m => `<div class="lrow small">${objLink(m.id, m.content)}</div>`).join('')}${items.map(i => `<div class="lrow small">借给她：${objLink(i.id)} <span class="badge st-${i.status}">${i.status}</span></div>`).join('') || ''}${!mems.length && !items.length ? '<p class="small muted">暂无</p>' : ''}${mgmt(`${n.id} · full=${n.full}`)}</div>
   <div class="card"><h3>和小遥的关系 ${r ? badge(r.stage, 'plain') : ''}</h3>${r ? `<div class="small">亲密度 ${r.intimacy.value} ${badge('推测')} 依据 ${r.intimacy.evidence.map(x => objLink(x)).join('、')}</div>
    <h3 style="margin-top:12px">关系变化</h3><div class="chain" data-test="rel-revs">${r.revisions.slice().reverse().map(v => `<div class="lrow small" data-test="relrev"><b>v${v.v}</b><span class="grow">${esc(v.text)}${v.source ? `<div class="muted">因为：${objLink(v.source)}</div>` : ''}${v.note ? `<div class="muted">${esc(v.note)}</div>` : ''}</span>${badge(v.by === '所有者修改' ? '所有者修改' : '系统运维')}<span class="muted">${v.at}</span></div>`).join('')}</div>
    <a class="btn" style="margin-top:8px" href="#/c/${ctx.charId}/people/relation/${r.id}" data-test="rel-edit">修正关系…</a>` : '<p class="small muted">还没有关系记录</p>'}</div></div>`;
}
const STAGES = ['陌生人', '熟人', '朋友', '好朋友', '挚友'];
export function renderRelEdit(el, ctx) {
  const r = store.get('relations', ctx.params.relId);
  if (!r) { el.innerHTML = '<div class="statebox"><h2>当前范围未找到该关系</h2></div>'; return; }
  const n = store.get('npcs', r.npcId);
  el.innerHTML = `<div class="card" style="max-width:640px"><div class="row"><img class="av" src="${n.avatar}" alt=""><h3 class="grow" style="margin:0">小遥 ↔ ${esc(n.name)}</h3>${badge('所有者修改')}</div>
   <p class="small muted">关系通常由运行时根据经历变化。所有者修正会作为新的修订记录保留，旧记录不删除。</p>
   <form class="stack" data-rf><label class="field">关系阶段<select class="select" name="stage" data-test="rel-stage">${STAGES.map(s => `<option ${s === r.stage ? 'selected' : ''}>${s}</option>`).join('')}</select></label>
   <label class="field">修正原因<input class="input" name="note" placeholder="例如：其实她们小时候就认识" data-test="rel-note"></label>
   <div class="diff" data-rd><div class="ctx">当前 v${r.version}：${esc(r.stage)}</div></div>
   <div class="row"><button type="button" class="btn" data-back data-test="rel-cancel">取消</button><button class="btn accent" data-test="rel-save">保存修正</button></div></form></div>`;
  const f = el.querySelector('[data-rf]');
  f.stage.onchange = () => { f.querySelector('[data-rd]').innerHTML = `<div class="del">- ${esc(r.stage)}</div><div class="add">+ ${esc(f.stage.value)}</div>`; };
  f.onsubmit = async e => { e.preventDefault(); const st = f.stage.value, note = f.note.value.trim(); if (st === r.stage && !note) return toast('没有改动', 'err');
    if (!(await requireOwner())) return; if (!(await confirm({ title: '修正关系', text: `${esc(r.stage)} → ${esc(st)}` }))) return;
    store.update(d => { const x = d.relations[r.id]; x.version++; x.revisions.push({ v: x.version, at: '刚刚', text: `${x.stage} → ${st}`, by: '所有者修改', note }); x.stage = st; }, { identity: '所有者修改', summary: `修正关系 ${r.id}：${r.stage} → ${st}`, target: r.id });
    toast('已保存为新修订', 'ok'); ctx.navigate(`#/c/${ctx.charId}/people/npc/${n.id}`); };
}
export function renderOwner(el, ctx) {
  const b = store.get('user').background;
  el.innerHTML = `<form class="card" style="max-width:640px" data-of><div class="row"><img class="av lg" src="/v3/assets/avatars/user.svg" alt=""><div class="grow"><h3 style="margin:0">我的背景</h3><p class="small muted">小遥会把这些当成“关于你的事实”。只填你愿意让她知道的。</p></div>${badge('角色范围')}</div>
   <div class="stack">${Object.entries(b).map(([k, v]) => `<label class="field">${k}<input class="input" name="${k}" value="${esc(v)}" data-test="owner-${k}"></label>`).join('')}</div>
   <div class="row" style="margin-top:12px"><button type="reset" class="btn" data-test="owner-cancel">撤销更改</button><button class="btn accent" data-test="owner-save">保存</button></div></form>`;
  const f = el.querySelector('[data-of]');
  f.onsubmit = async e => { e.preventDefault(); if (!(await requireOwner())) return; const nb = Object.fromEntries(Object.keys(b).map(k => [k, f[k].value]));
    store.update(d => { d.user.background = nb; }, { identity: '所有者修改', summary: '更新所有者背景', target: null }); toast('已保存', 'ok'); };
}
/* 媒体 */
export function renderGallery(el, ctx) {
  const ms = store.list('media', m => m.charId === 'c1').reverse();
  el.innerHTML = `<div class="banner warn">${badge('生成图·非事实')} 这里的图都是生成画面，不代表发生过。</div><div class="wgrid" data-test="media-grid" style="margin-top:12px">${ms.map(m => `<a class="card" style="padding:8px;display:block" href="#/c/${ctx.charId}/media/${m.id}" data-test="media-${m.id}">${genImg(m)}<div class="small" style="margin-top:6px"><b>${esc(m.title)}</b>${m.itemId ? `<br>物品：${esc(nameOf(m.itemId))}` : ''}${m.outfitSnapshot ? `<br>穿搭：${m.outfitSnapshot.map(i => esc(nameOf(i))).join('、')}` : ''}</div></a>`).join('')}</div>
  <div class="row" style="margin-top:12px"><a class="btn primary" href="#/c/${ctx.charId}/media/generate">${icon('sparkle')} 生成一张</a><a class="btn" href="#/c/${ctx.charId}/media/identity">视觉身份</a></div>`;
}
export function renderMediaDetail(el, ctx) {
  const m = store.get('media', ctx.params.mdId);
  if (!m) { el.innerHTML = `<div class="statebox"><h2>当前范围未找到这张图</h2><a class="btn" href="#/c/${ctx.charId}/media">回媒体库</a></div>`; return; }
  el.innerHTML = `<div class="grid g2" style="align-items:start"><div class="card">${genImg(m)}</div><div class="card"><h2>${esc(m.title)}</h2>${badge('生成图·非事实')}
   <div class="kv" style="margin-top:8px"><dt>画面地点</dt><dd>${m.placeId ? objLink(m.placeId) : '—'}</dd><dt>物品</dt><dd>${m.itemId ? objLink(m.itemId) : '—'}</dd><dt>穿搭快照</dt><dd>${m.outfitSnapshot ? m.outfitSnapshot.map(i => objLink(i)).join('、') : '—'}</dd><dt>出现在</dt><dd>${m.sourceMsg ? objLink(m.sourceMsg, '对话消息') : '媒体库'}</dd></div>
   <p class="small muted">快照只记录生成那一刻的穿搭，不会反过来改变她的穿着。</p><a class="btn primary" href="#/c/${ctx.charId}/media/generate?ref=${m.id}">以此为参考再生成</a>${mgmt(`${m.id} · nonFact=${m.nonFact}`)}</div></div>`;
}
export function renderIdentity(el, ctx) {
  const c = store.get('characters', 'c1');
  el.innerHTML = `<div class="grid g3"><div class="card" style="text-align:center"><img src="${c.avatar}" alt="" style="width:160px"><h3>标准头像</h3>${badge('全局定义')}</div>
   <div class="card"><h3>外貌要点</h3><ul class="small"><li>齐肩短发、浅棕色</li><li>常穿暖色系、碎花和条纹</li><li>爱戴草编帽</li></ul><p class="small muted">生成图都参考这里，保证她看起来是同一个人。</p></div>
   <div class="card"><h3>参考图</h3><div class="row">${store.list('media', m => m.charId === 'c1').slice(0, 3).map(m => `<img src="${m.img}" alt="" style="width:80px;border-radius:8px">`).join('')}</div><p class="small muted">修改视觉身份需在创作中心编辑角色定义（未接入）。</p></div></div>`;
}
export function renderGenerate(el, ctx) {
  const c = store.get('characters', 'c1'); const ref = ctx.query.ref && store.get('media', ctx.query.ref);
  const owned = store.list('items', i => i.charId === 'c1' && ['拥有', '使用', '借用'].includes(i.status));
  el.innerHTML = `<div class="grid g2" style="align-items:start"><form class="card" data-gf><h3>${icon('sparkle')} 生成一张图</h3><div class="banner warn small">${badge('生成图·非事实')} 只是画面；不会改变她的位置、穿着或任何事实。</div>
   <div class="stack" style="margin-top:8px"><label class="field">画面描述<input class="input" name="prompt" value="${esc(ref ? ref.title + '（再来一张）' : '小遥在海边画画')}" data-test="gen-prompt"></label>
   <label class="field">画面地点<select class="select" name="place">${store.list('places').map(p => `<option value="${p.id}" ${p.id === (ref?.placeId || 'p_book') ? 'selected' : ''}>${esc(p.name)}</option>`).join('')}</select></label>
   <label class="field">带上物品<select class="select" name="item"><option value="">不带</option>${owned.map(i => `<option value="${i.id}">${esc(i.name)}</option>`).join('')}</select></label></div>
   <div class="row" style="margin-top:12px"><button type="reset" class="btn">重置</button><button class="btn primary" data-test="gen-submit">${icon('sparkle')} 生成</button></div></form>
   <div class="card" data-out><h3>结果</h3><p class="small muted">小遥此刻实际在：${esc(nameOf(c.locationId))}</p><div data-res></div></div></div>`;
  el.querySelector('[data-gf]').onsubmit = e => { e.preventDefault(); const f = e.target; const out = el.querySelector('[data-res]'); const loc0 = c.locationId;
    out.innerHTML = '<div class="skel" style="height:140px"></div><p class="small muted">生成中…</p>';
    setTimeout(() => { const id = store.uid('md_'); const p = store.get('places', f.place.value);
      store.update(d => { d.media[id] = { id, charId: 'c1', img: p.scene, overlay: f.item.value ? d.items[f.item.value].img : null, title: f.prompt.value, nonFact: true, itemId: f.item.value || null, outfitSnapshot: d.outfit.c1.current.slice(), placeId: p.id, at: hm() }; }, { identity: '交流', summary: '媒体生成（生成图·非事实）', target: id });
      const md = store.get('media', id); const box = document.querySelector('[data-res]');
      if (box) box.innerHTML = `${genImg(md)}<p class="small" data-test="gen-page-result">已存入媒体库 · <a href="#/c/c1/media/${id}">查看</a><br>${store.get('characters', 'c1').locationId === loc0 ? '✓ 她的位置没有改变' : ''}</p>`; }, 400); };
}
