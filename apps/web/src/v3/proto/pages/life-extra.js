// 生活成长扩展页：日程（计划 vs 实际）、目标/意向/约定、日记、共同经历、记忆、反思、自我认知、角色资料
import * as store from '../store.js';
import { esc, icon, badge, drawer, toast, objLink, mgmt, nameOf, confirm, requireOwner } from '../ui.js';
import { T, addMsg } from './flows.js';
import { wait, hm, statusKind, hl } from './common.js';

/* 建议日程抽屉（生活·日程、地点详情共用）：行动请求 → 她决定 */
export function suggestScheduleDrawer(placeId = 'p_cafe', title = '') {
  const c = store.get('characters', 'c1');
  const d = drawer({ title: `${icon('calendar')} 建议一项日程`, body: `<p class="small muted">${badge('行动请求')} 只是建议，小遥接受后才进入她的计划；现在她仍在${esc(nameOf(c.locationId))}。</p>
   <form class="stack" data-sf><label class="field">做什么<input class="input" name="title" value="${esc(title || '去' + nameOf(placeId) + '看看')}" data-test="sched-title"></label>
   <label class="field">时间<input class="input" name="time" value="17:00-18:00"></label>
   <label class="field">地点<select class="select" name="place" data-test="sched-place">${store.list('places').map(p => `<option value="${p.id}" ${p.id === placeId ? 'selected' : ''}>${esc(p.name)}</option>`).join('')}</select></label></form>`,
    footer: `<button class="btn" data-c data-test="sched-cancel">取消</button><button class="btn primary" data-ok data-test="sched-submit">发出建议</button>` });
  d.el.dataset.test = 'sched-drawer';
  d.el.querySelector('[data-c]').onclick = () => { d.close(); toast('已取消，没有发出建议'); };
  d.el.querySelector('[data-ok]').onclick = async () => {
    const f = d.el.querySelector('[data-sf]'); if (!f.title.value.trim()) return toast('写一下做什么', 'err');
    const id = store.uid('s_'); const v = { title: f.title.value.trim(), time: f.time.value, placeId: f.place.value };
    store.update(x => { x.schedule[id] = { id, charId: 'c1', ...v, state: '候选', proposedBy: 'u1', fact: '尚未发生', factState: '未知' }; }, { identity: '行动请求', summary: `建议小遥日程：${v.title}`, target: id });
    d.close(); toast('建议已发出，等小遥决定');
    await wait(T.step);
    store.update(x => { x.schedule[id].state = '计划'; x.schedule[id].note = '小遥接受了你的建议'; }, { identity: '行动请求', summary: `小遥接受日程建议 ${v.title}`, target: id });
    addMsg({ from: 'char', text: `好呀，${v.time.split('-')[0]} 我去${nameOf(v.placeId)}～`, identity: '交流' });
  };
  return d;
}
export function renderSchedule(el, ctx) {
  const list = store.list('schedule', s => s.charId === ctx.charId).sort((a, b) => a.time > b.time ? 1 : -1);
  el.innerHTML = `<div class="row" style="margin-bottom:12px"><span class="grow small muted">今天 10-15 · 左边是计划，右边是实际发生的</span><button class="btn primary" data-sug data-test="sched-suggest">${icon('calendar')} 建议一项日程</button></div>
  <div class="card"><div class="planfact small muted"><b>时间</b><b>计划</b><b>实际</b></div><div class="list">${list.map(s => `<div class="lrow planfact" data-test="sched-${s.id}" style="border-left:3px ${s.state === '计划' || s.state === '候选' ? 'dashed var(--sky)' : 'solid var(--leaf)'};padding-left:10px">
    <b>${esc(s.time)}</b><div>${esc(s.title)} ${badge(s.state)}<div class="small">${icon('pin')} ${objLink(s.placeId)}${s.adjustedBy ? ` · 因${objLink(s.adjustedBy)}调整` : ''}${s.note ? ` · <span class="muted">${esc(s.note)}</span>` : ''}</div></div>
    <div data-test="fact-${s.id}">${esc(s.fact || '—')} ${badge(s.factState || '未知')}${mgmt(`${s.id}${s.actionId ? ' · 行动 ' + objLink(s.actionId, s.actionId) : ''}${s.goalId ? ' · 目标 ' + objLink(s.goalId, s.goalId) : ''} · placeId ${s.placeId}`)}</div></div>`).join('')}</div></div>
  <p class="small muted">虚线＝计划/候选，实线＝已发生。建议的日程要等她接受。</p>`;
  el.querySelector('[data-sug]').onclick = () => suggestScheduleDrawer();
}
/* goals.list.view */
export function renderGoals(el, ctx) {
  el.innerHTML = `<div class="row" style="margin-bottom:12px"><span class="grow small muted">她想要的、她在做的、和你说好的</span><button class="btn primary" data-promise data-test="promise-new">${icon('heart')} 和她约定</button></div>
  <div class="grid g3">${['目标', '意向', '约定'].map(t => `<div class="card"><h3>${t}</h3>${store.list('goals', g => g.charId === ctx.charId && g.type === t).map(g => `<div class="lrow" data-test="goal-${g.id}"><div class="grow"><b>${esc(g.title)}</b><div class="small muted">${esc(g.why || g.due || g.progress || g.note || '')}</div>
    ${g.scheduleId ? `<div class="small">→ ${objLink(g.scheduleId)} → ${objLink(g.actionId)} → ${objLink(g.itemId)}</div>` : ''}${mgmt(`${g.id}${g.sourceMsg ? ' · 来源 ' + objLink(g.sourceMsg, g.sourceMsg) : ''}`)}</div>${badge(g.state)}</div>`).join('') || '<p class="small muted">暂无</p>'}</div>`).join('')}</div>
  <div class="banner info" style="margin-top:12px">${icon('info')} 想要碎花裙：在<a href="#/c/c1/chat">对话</a>里对「购买碎花裙」行动卡发出请求。</div>`;
  el.querySelector('[data-promise]').onclick = () => {
    const d = drawer({ title: `${icon('heart')} 和小遥约定 ${badge('交流')}`, body: `<form class="stack" data-pf><label class="field">约定内容<input class="input" name="title" value="下周一起去花店看向日葵" data-test="promise-title"></label><label class="field">时间<input class="input" name="due" value="10-20 15:00"></label><p class="small muted">约定要她答应才算数。</p></form>`,
      footer: `<button class="btn" data-c>取消</button><button class="btn primary" data-ok data-test="promise-submit">提出约定</button>` });
    d.el.querySelector('[data-c]').onclick = () => d.close();
    d.el.querySelector('[data-ok]').onclick = async () => { const f = d.el.querySelector('[data-pf]'); const id = store.uid('g_');
      store.update(x => { x.goals[id] = { id, charId: 'c1', type: '约定', title: f.title.value, due: f.due.value, state: '候选', note: '等她答应' }; }, { identity: '交流', summary: '向小遥提出约定：' + f.title.value, target: id });
      d.close(); await wait(T.step);
      store.update(x => { x.goals[id].state = '已确认'; x.goals[id].note = '她答应了'; }, { identity: '交流', summary: '小遥答应约定', target: id });
      addMsg({ from: 'char', text: `一言为定！${f.title.value}～`, identity: '交流' }); toast('小遥答应了', 'ok'); };
  };
}
/* diary */
export function renderDiaryList(el, ctx) {
  el.innerHTML = `<p class="small muted">${badge('角色主观记录', 'plain')} 日记是她自己的感受，可能和事实不一致。</p>` + store.list('diary', d => d.charId === ctx.charId).sort((a, b) => b.date > a.date ? 1 : -1).map(d => `<a class="card tint-sun" style="display:block;margin-bottom:12px" href="#/c/${ctx.charId}/life/diary/${d.id}" data-test="diary-${d.id}"><b>${d.date}</b><p>${esc(d.text)}</p><span class="small muted">${(d.replies || []).length} 条回应 · 打开 →</span></a>`).join('')
    + `<a class="btn" href="#/c/${ctx.charId}/life/shared">共同经历时间线</a>`;
}
export function renderDiaryEntry(el, ctx) {
  const d = store.get('diary', ctx.params.entryId);
  if (!d) { el.innerHTML = `<div class="statebox"><h2>当前范围未找到这篇日记</h2><a class="btn" href="#/c/${ctx.charId}/life/diary">回日记</a></div>`; return; }
  const wrong = d.refs.includes('m31') && store.get('memories', 'm31').version === 1;
  el.innerHTML = `<div class="card tint-sun"><div class="row"><h2 class="grow" style="margin:0">${d.date}</h2>${badge('角色主观记录', 'plain')}</div><p style="font-size:16px">${esc(d.text)}</p>
   <div class="small">提到：${d.refs.map(r => objLink(r)).join('、')}</div>${wrong ? `<div class="banner warn small">${icon('warn')} 这里提到的“书店停业”可能有误（实际是暂停装修）。日记原文不会被改写，<a href="#/c/${ctx.charId}/chat/corrections">去纠错</a>。</div>` : ''}
   ${mgmt(`ID ${d.id} · page key diary.entry.detail`)}</div>
  <div class="card" style="margin-top:12px"><h3>你的回应</h3><div class="list" data-test="replies">${(d.replies || []).map(r => `<div class="lrow small">${badge('交流')}<span class="grow">${esc(r.text)}</span><span class="muted">${r.at}</span></div>`).join('') || '<p class="small muted">还没有回应</p>'}</div>
   <form class="row" style="margin-top:8px" data-rf><input class="input grow" name="t" placeholder="她会在对话里看到…" data-test="reply-input"><button type="button" class="btn" data-clear>取消</button><button class="btn primary" data-test="reply-send">回应</button></form></div>`;
  const f = el.querySelector('[data-rf]');
  f.querySelector('[data-clear]').onclick = () => { f.t.value = ''; };
  f.onsubmit = e => { e.preventDefault(); const t = f.t.value.trim(); if (!t) return;
    store.update(x => { x.diary[d.id].replies = x.diary[d.id].replies || []; x.diary[d.id].replies.push({ text: t, at: hm() }); }, { identity: '交流', summary: '回应日记 ' + d.id, target: d.id });
    addMsg({ from: 'user', text: `（看了你 ${d.date} 的日记）${t}`, identity: '交流' }); toast('已回应，也发到了对话里', 'ok'); };
}
export function renderShared(el, ctx) {
  const rows = [
    ...store.list('events', e => e.worldId === 'w1').map(e => ({ at: e.at, html: `${badge('事件', 'plain')} ${objLink(e.id)}` })),
    ...store.list('memories', m => m.charId === ctx.charId && m.kind !== '世界认知').map(m => ({ at: m.revisions[0].at, html: `${badge('记忆', 'plain')} ${objLink(m.id, m.content)}` })),
    ...store.list('goals', g => g.type === '约定').map(g => ({ at: g.due || '', html: `${badge('约定', 'plain')} ${esc(g.title)} ${badge(g.state)}` })),
    ...store.list('diary', d => d.charId === ctx.charId).map(d => ({ at: d.date, html: `${badge('日记', 'plain')} ${objLink(d.id, d.text.slice(0, 24) + '…')}` })),
  ].sort((a, b) => a.at > b.at ? 1 : -1);
  el.innerHTML = `<p class="small muted">从事件、记忆、约定、日记汇总，只读。</p><div class="card chain">${rows.map(r => `<div class="lrow"><b class="small" style="width:90px">${esc(r.at)}</b><span class="grow">${r.html}</span></div>`).join('')}</div>`;
}
/* growth.memory.list */
export function renderMemList(el, ctx) {
  const kind = ctx.query.kind || '全部'; const kw = ctx.query.q || '';
  const all = store.list('memories', m => m.charId === ctx.charId); const kinds = ['全部', ...new Set(all.map(m => m.kind))];
  const ms = all.filter(m => (kind === '全部' || m.kind === kind) && (!kw || m.content.includes(kw)));
  el.innerHTML = `<div class="row" style="margin-bottom:12px"><div class="tabs grow">${kinds.map(k => `<a class="tab ${k === kind ? 'on' : ''}" href="#/c/${ctx.charId}/growth?kind=${encodeURIComponent(k)}">${k}</a>`).join('')}</div>
    <form class="row" data-mf><input class="input" name="q" value="${esc(kw)}" placeholder="搜索记忆" data-test="mem-search"><button class="btn sm">搜</button></form></div>
  <div class="card"><div class="list">${ms.map(m => `<a class="lrow click" href="#/c/${ctx.charId}/growth/memory/${m.id}" data-test="mem-${m.id}"><span class="grow">${hl(m.content, kw)}<div class="small muted">${m.kind}${m.revisions.length > 1 ? ` · 改过 ${m.revisions.length - 1} 次` : ''}${m.pendingRevision ? ' · 有候选修订' : ''}</div></span>${badge(m.state)}</a>`).join('') || '<p class="small muted">当前筛选未找到记忆</p>'}</div></div>
  <div class="row" style="margin-top:12px"><a class="btn" href="#/c/${ctx.charId}/growth/reflection">反思与成长</a><a class="btn" href="#/c/${ctx.charId}/growth/self">自我认知</a></div>`;
  el.querySelector('[data-mf]').onsubmit = e => { e.preventDefault(); ctx.navigate(`#/c/${ctx.charId}/growth?kind=${encodeURIComponent(kind)}&q=${encodeURIComponent(e.target.q.value)}`); };
}
/* growth.memory.item */
export function renderMemItem(el, ctx) {
  const m = store.get('memories', ctx.params.memId);
  if (!m) { el.innerHTML = '<div class="statebox"><h2>当前范围未找到该记忆</h2></div>'; return; }
  const suspect = m.id === 'm31' && /已经停业/.test(m.content);
  el.innerHTML = `<div class="card"><div class="row"><span class="grow small muted">${m.kind}</span>${badge(m.state)}<span class="muted small">第 ${m.version} 版</span></div><p style="font-size:17px" data-test="memory-content">${esc(m.content)}</p>
    <div class="small">来源 ${objLink(m.sourceMsg || m.sourceEvent)}${m.relatedPlace ? ' · 地点 ' + objLink(m.relatedPlace) : ''}${m.relatedNpc ? ' · 人物 ' + objLink(m.relatedNpc) : ''}</div>
    ${suspect ? `<div class="banner warn" data-test="mem-suspect">${icon('warn')} 可能有误：海边书店的实例修正写的是“暂停装修”，不是停业。</div>` : ''}
    ${m.pendingRevision ? `<div class="banner info">${badge('候选')} 你告诉过她：${esc(m.pendingRevision)}（她还没决定）</div>` : ''}
    <div class="row" style="margin-top:10px"><button class="btn" data-tell data-test="mem-tell">${icon('chat')} 告诉她</button><button class="btn accent" data-edit data-test="mem-edit">所有者修改…</button></div>
    <div data-editor></div>${mgmt(`#${m.id} · page key growth.memory.item`)}</div>
  <div class="card" style="margin-top:12px"><h3>修订记录</h3>${m.revisions.slice().reverse().map(r => `<div class="lrow" data-test="rev"><b>v${r.v}</b><span class="grow">${esc(r.text)}<div class="small muted">${esc(r.note || '')}</div></span>${badge(r.by === '所有者修改' ? '所有者修改' : '系统运维')}<span class="small muted">${r.at}</span></div>`).join('')}</div>`;
  el.querySelector('[data-tell]').onclick = () => {
    const d = drawer({ title: `告诉她 ${badge('交流')}`, body: `<p class="small muted">她自己决定要不要改记忆，事实不会直接变。</p><textarea class="input" rows="3" data-t>${m.id === 'm31' ? '海边书店只是暂停装修，没有停业哦。' : ''}</textarea>`, footer: `<button class="btn" data-c>取消</button><button class="btn primary" data-ok data-test="mem-tell-send">发送</button>` });
    d.el.querySelector('[data-c]').onclick = () => d.close();
    d.el.querySelector('[data-ok]').onclick = () => { const t = d.el.querySelector('[data-t]').value.trim(); if (!t) return; store.update(x => { x.memories[m.id].pendingRevision = t; }, { identity: '交流', summary: `告诉小遥关于记忆 #${m.id}：${t}`, target: m.id }); addMsg({ from: 'user', text: t, identity: '交流' }); d.close(); toast('已告诉她', 'ok'); };
  };
  el.querySelector('[data-edit]').onclick = () => {
    const box = el.querySelector('[data-editor]'); const base = m.version; let draft = m.content;
    const diff = () => `<div class="diff" data-test="mem-diff"><div class="del">- ${esc(m.content)}</div><div class="add">+ ${esc(draft)}</div></div>`;
    box.innerHTML = `<div class="card tint-accent" style="margin-top:10px"><label class="field">修改为<input class="input" data-d value="${esc(draft)}" data-test="mem-draft"></label><div data-diff>${diff()}</div>
      <div class="row" style="margin-top:8px"><button class="btn" data-c data-test="mem-cancel">取消</button><button class="btn accent" data-s data-test="mem-save">保存修改</button></div></div>`;
    box.querySelector('[data-d]').oninput = e => { draft = e.target.value; box.querySelector('[data-diff]').innerHTML = diff(); };
    box.querySelector('[data-c]').onclick = () => { box.innerHTML = ''; toast('已取消，记忆未改变'); };
    box.querySelector('[data-s]').onclick = async () => {
      if (draft === m.content) return toast('内容没有变化', 'err');
      if (!(await requireOwner())) return;
      if (store.get('memories', m.id).version !== base) { toast('版本已变化，请刷新后再改', 'err'); return; }
      if (!(await confirm({ title: '所有者修改记忆', text: `将“${esc(m.content)}”改为“${esc(draft)}”。` }))) return;
      store.update(x => { const mm = x.memories[m.id]; mm.version++; mm.content = draft; mm.state = '已确认'; delete mm.pendingRevision; mm.revisions.push({ v: mm.version, at: '刚刚', text: draft, by: '所有者修改', note: '记忆页直接修改' }); }, { identity: '所有者修改', summary: `修改记忆 #${m.id} → ${draft}`, target: m.id });
      toast('已保存', 'ok');
    };
  };
}
/* reflection */
export function renderReflections(el, ctx) {
  el.innerHTML = `<p class="small muted">反思是她自己的想法，你可以回应，但不能替她改。</p><div class="chain">${store.list('reflections', r => r.charId === ctx.charId).sort((a, b) => b.at > a.at ? 1 : -1).map(r => `<a class="card" style="display:block;margin-bottom:12px" href="#/c/${ctx.charId}/growth/reflection/${r.id}" data-test="refl-${r.id}"><div class="row"><b class="grow">${esc(r.title)}</b>${badge(r.state)}</div><p class="small">${esc(r.text)}</p><div class="small muted">${r.at} · 成长：${esc(r.growth)}${r.feedback ? ' · 你已回应' : ''}</div></a>`).join('')}</div>`;
}
export function renderReflection(el, ctx) {
  const r = store.get('reflections', ctx.params.refId);
  if (!r) { el.innerHTML = `<div class="statebox"><h2>当前范围未找到这条反思</h2><a class="btn" href="#/c/${ctx.charId}/growth/reflection">回反思时间线</a></div>`; return; }
  el.innerHTML = `<div class="card"><div class="row"><h2 class="grow" style="margin:0">${esc(r.title)}</h2>${badge(r.state)}</div><p style="font-size:16px">${esc(r.text)}</p><div class="small">成长方向：<b>${esc(r.growth)}</b></div><div class="small">想到这些：${r.sources.map(s => objLink(s)).join('、')}</div>${mgmt(`${r.id} · ${r.at} · page key growth.reflection.detail`)}</div>
  <div class="card" style="margin-top:12px"><h3>你的回应 ${badge('交流')}</h3>${r.feedback ? `<div class="banner ok" data-test="refl-fb">${esc(r.feedback.kind)}：${esc(r.feedback.text || '')}<span class="small muted"> · ${r.feedback.at}</span></div>` : ''}
   <form class="stack" data-ff><textarea class="input" rows="2" name="t" placeholder="（可选）想对她说的话" data-test="refl-text"></textarea><div class="row"><button type="button" class="btn" data-k="有不同看法">有不同看法</button><button type="button" class="btn primary" data-k="认同" data-test="refl-agree">认同</button></div></form></div>`;
  el.querySelectorAll('[data-k]').forEach(b => b.onclick = () => { const t = el.querySelector('[name=t]').value.trim(); store.update(() => {}, { identity: '交流', summary: `回应反思 ${r.id}：${b.dataset.k}${t ? ' ' + t : ''}`, target: r.id }); });
}
/* growth.self.view */
export function renderSelf(el, ctx) {
  const c = store.get('characters', ctx.charId); const props = store.list('proposals', p => p.charId === c.id);
  el.innerHTML = `<div class="grid g2"><div class="card tint-primary"><h3>${icon('user')} 她眼中的自己</h3><div class="row">${c.traits.map(t => badge(t, 'plain')).join('')}</div>
    <h3 style="margin-top:12px">最近在变化</h3>${store.list('reflections', r => r.charId === c.id).map(r => `<div class="lrow small"><span class="grow">${esc(r.growth)}</span>${badge(r.state)}</div>`).join('')}</div>
   <div class="card"><h3>人格提案</h3><p class="small muted">提案来自她的反思，所有者决定是否写入人格定义。</p>${props.map(p => `<div class="lrow" data-test="prop-${p.id}"><span class="grow">${esc(p.title)}</span>${badge(p.state)}${p.state === '候选' ? `<button class="btn sm" data-no="${p.id}">暂不采纳</button><button class="btn sm accent" data-yes="${p.id}" data-test="prop-accept">采纳</button>` : ''}</div>`).join('') || '<p class="small muted">暂无提案</p>'}</div></div>`;
  const decide = async (id, yes) => { if (!(await requireOwner())) return; if (!(await confirm({ title: yes ? '采纳人格提案' : '暂不采纳', text: esc(store.get('proposals', id).title) }))) return;
    store.update(x => { x.proposals[id].state = yes ? '已确认' : '取消'; if (yes && !x.characters[c.id].traits.includes('主动分享')) x.characters[c.id].traits.push('主动分享'); }, { identity: '所有者修改', summary: `${yes ? '采纳' : '暂不采纳'}人格提案 ${id}`, target: id }); };
  el.querySelectorAll('[data-yes]').forEach(b => b.onclick = () => decide(b.dataset.yes, true));
  el.querySelectorAll('[data-no]').forEach(b => b.onclick = () => decide(b.dataset.no, false));
}
/* profile.char.view —— 每个角色显示自己的资料 */
export function renderProfile(el, ctx) {
  const c = store.get('characters', ctx.charId);
  if (!c) { el.innerHTML = '<div class="statebox"><h2>当前范围未找到该角色</h2></div>'; return; }
  const w = store.get('worlds', c.worldId); const ro = c.status === '已退休';
  el.innerHTML = `<div class="grid g2"><div class="card" data-test="profile-${c.id}"><div class="row"><img class="av lg" src="${c.avatar}" alt=""><div class="grow"><h2 style="margin:0">${esc(c.name)}</h2><div class="small muted">${esc(w?.name || '运行世界未创建')}</div></div>${badge(c.status, statusKind(c.status))}</div>
    <p data-test="profile-bio">${esc(c.bio)}</p><div class="row">${c.traits.map(t => badge(t, 'plain')).join('') || '<span class="small muted">尚未设定性格</span>'}</div>
    ${c.statusNote ? `<div class="banner ${c.status === '异常' ? 'err' : 'warn'}" style="margin-top:8px">${esc(c.statusNote)}</div>` : ''}
    <div class="row" style="margin-top:12px"><button class="btn" data-edit data-test="profile-edit" ${ro ? 'disabled title="已退休，只读归档"' : ''}>编辑简介…</button><a class="btn ghost" href="#/c/${c.id}/settings/autonomy">自治与触达</a>${c.id === 'c1' ? `<a class="btn ghost" href="#/c/c1/growth/self">自我认知</a>` : ''}</div>
    ${mgmt(`ID ${c.id} · 定义 ${c.defVersion || '—'} · 世界 ${c.worldId || '—'} · 分组 ${c.groupId}`)}</div>
   <div class="card"><h3>她现在</h3>${c.id === 'c1' ? `<div class="kv"><dt>在</dt><dd>${objLink(c.locationId)}</dd><dt>正在</dt><dd>${esc(c.doing)}</dd><dt>心情</dt><dd>${esc(c.mood.value)} ${badge('推测')}</dd><dt>安全拦截</dt><dd>${typeof c.harmIntercepts === 'number' ? `${c.harmIntercepts} 次永久伤害事件被拦截` : '尚未接入'}</dd></div>`
      : `<p class="muted">${esc(c.name)}目前${esc(c.status)}，没有实时生活状态。</p>`}
    ${store.list('proposals', p => p.charId === c.id && p.state === '候选').map(p => `<div class="banner warn small"><a href="#/c/${c.id}/growth/self">${esc(p.title)}</a> ${badge(p.state)}</div>`).join('')}</div></div>`;
  el.querySelector('[data-edit]').onclick = () => {
    const d = drawer({ title: `编辑简介 ${badge('所有者修改')}`, body: `<textarea class="input" rows="4" data-b data-test="bio-input">${esc(c.bio)}</textarea><p class="small muted">修改的是角色范围的资料，会写入审计。</p>`, footer: `<button class="btn" data-c data-test="bio-cancel">取消</button><button class="btn accent" data-ok data-test="bio-save">保存</button>` });
    d.el.querySelector('[data-c]').onclick = () => d.close();
    d.el.querySelector('[data-ok]').onclick = async () => { const v = d.el.querySelector('[data-b]').value.trim(); if (!v) return; if (!(await requireOwner())) return; store.update(x => { x.characters[c.id].bio = v; }, { identity: '所有者修改', summary: `修改 ${c.name} 简介`, target: c.id }); d.close(); toast('已保存', 'ok'); };
  };
}
