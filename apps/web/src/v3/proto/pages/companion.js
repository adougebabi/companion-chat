import { providerPage } from './provider-page.js';
import { Tabs, Field } from '../components/forms.js';
import { PreferenceFields } from '../components/forms.js';
// 相处体验扩展页：会话历史、纠错记录、多模态、动态、主动触达、通知
import * as store from '../store.js';
import { esc, icon, badge, drawer, toast, objLink, mgmt, nameOf, avatarOf, confirm } from '../ui.js';
import { openCorrection, genImg } from './chat.js';
import { T } from './flows.js';
import { hl, wait, hm } from './common.js';

/* chat.history.view */
export function renderHistory(el, ctx) {
  if (!store.isMock) return providerPage(() => import('./real-history.js'), 'renderHistory',el,ctx);
  const c = store.get('characters', ctx.charId); const kw = ctx.query.q || '';
  const all = store.list('messages', m => m.charId === c.id && m.from !== 'action');
  const hits = kw ? all.filter(m => m.from !== 'divider' && (m.text || '').includes(kw)) : all;
  el.innerHTML = `<form class="row" data-test="hist-form"><input class="input grow" name="q" value="${esc(kw)}" placeholder="搜索和${esc(c.name)}的聊天…" data-test="hist-input"><button class="btn primary" data-test="hist-go">${icon('chat')} 搜索</button>${kw ? '<button type="button" class="btn" data-clear>清除</button>' : ''}</form>
  <p class="small muted" data-test="hist-count">范围：和${esc(c.name)}的已读取消息 · ${kw ? `匹配“${esc(kw)}”的 ${hits.length} 条` : `${hits.filter(m => m.from !== 'divider').length} 条消息`}</p>
  ${hits.length ? `<div class="card"><div class="list" data-test="hist-list">${hits.map(m => m.from === 'divider' ? `<div class="divider">${esc(m.text)}</div>`
    : `<a class="lrow click" href="#/c/${c.id}/chat?from=chat.history.view" data-test="hist-hit"><img class="av sm" src="${m.from === 'user' ? '/v3/assets/avatars/user.svg' : c.avatar}" alt=""><span class="grow">${m.from === 'sys' ? badge('系统运维') + ' ' : ''}${hl(m.text, kw)}${m.corrected ? ' ' + badge('已纠正', 'plain') : ''}</span><span class="small muted">${m.at || ''}</span></a>`).join('')}</div></div>`
    : `<div class="statebox"><h2>当前范围未找到“${esc(kw)}”</h2><p class="muted">只搜索和${esc(c.name)}的聊天，不含动态和日记。</p><button class="btn" data-clear>清除搜索</button></div>`}
  ${mgmt(`page key chat.history.view · 消息来源 messages（charId=${c.id}）· 行动卡不参与搜索`)}`;
  el.querySelector('form').onsubmit = e => { e.preventDefault(); ctx.navigate(`#/c/${c.id}/chat/history?q=${encodeURIComponent(e.target.q.value.trim())}`); };
  el.querySelectorAll('[data-clear]').forEach(b => b.onclick = () => ctx.navigate(`#/c/${c.id}/chat/history`));
}

/* chat.correction.list —— 纠错记录 */
export function renderCorrections(el, ctx) {
  const msgs = store.list('messages', m => m.charId === ctx.charId);
  const todo = msgs.filter(m => m.memoryRef && !m.corrected);
  const done = msgs.filter(m => m.corrected || m.feedback);
  const memEdits = store.list('memories', m => m.charId === ctx.charId).flatMap(m => m.revisions.filter(r => r.by === '所有者修改').map(r => ({ m, r })));
  const pend = store.list('memories', m => m.charId === ctx.charId && m.pendingRevision);
  el.innerHTML = `<div class="grid g2">
   <div class="card"><h3>${icon('flag')} 可能有误的说法</h3>${todo.map(m => `<div class="lrow" data-test="corr-todo-${m.id}"><span class="grow">“${esc(m.text)}”<div class="small muted">${m.at} · 关联记忆：${esc(store.get('memories', m.memoryRef)?.content)}</div></span><button class="btn sm primary" data-fix="${m.id}" data-test="corr-open-${m.id}">纠正</button></div>`).join('') || '<p class="small muted">当前范围未找到待纠正的说法</p>'}</div>
   <div class="card"><h3>${icon('brain')} 等她自己决定的修订 ${badge('候选')}</h3>${pend.map(m => `<div class="lrow small"><span class="grow">${objLink(m.id, m.content)}<div class="muted">你告诉她：${esc(m.pendingRevision)}</div></span>${badge('交流')}</div>`).join('') || '<p class="small muted">当前范围未找到候选修订</p>'}</div></div>
  <div class="card" style="margin-top:12px"><h3>已处理</h3><div class="list" data-test="corr-done">
   ${done.map(m => `<div class="lrow small">${m.corrected ? badge('所有者修改') : badge('交流')}<span class="grow">“${esc(m.text)}” → ${m.corrected ? '已纠正（原文保留）' : '已告诉她'}</span><span class="muted">${m.at}</span></div>`).join('')}
   ${memEdits.map(({ m, r }) => `<div class="lrow small">${badge('所有者修改')}<span class="grow">${objLink(m.id, m.content)} <span class="muted">v${r.v} · ${esc(r.note || '')}</span></span><span class="muted">${r.at}</span></div>`).join('') || ''}
   ${!done.length && !memEdits.length ? '<p class="small muted">当前范围未找到纠错记录</p>' : ''}</div></div>
  ${mgmt('page key chat.correction.list（由 messages.corrected/feedback 与 memories.revisions 派生，不另存）· 纠错抽屉 chat.correction.edit')}`;
  el.querySelectorAll('[data-fix]').forEach(b => b.onclick = () => openCorrection(b.dataset.fix));
}

/* chat.media.view —— 多模态（展示部分） */
export function renderChatMedia(el, ctx) {
  const imgs = store.list('messages', m => m.charId === ctx.charId && m.mediaId).map(m => ({ m, md: store.get('media', m.mediaId) })).filter(x => x.md);
  el.innerHTML = `<div class="banner info">${icon('info')} 这里汇总当前已读取会话中的图片。可前往角色媒体库查看其他素材。</div>
   <div class="wgrid" style="margin-top:12px">${imgs.map(({ m, md }) => `<a class="card" style="padding:8px" href="#/c/${ctx.charId}/media/${md.id}">${genImg(md)}<div class="small" style="margin-top:6px">${esc(m.text || md.title)}<div class="muted">${m.at}</div></div></a>`).join('') || '<div class="statebox"><h2>当前已读取的会话中没有图片</h2><a class="btn" href="#/c/${ctx.charId}/media">查看角色媒体库</a></div>'}</div>`;
}

/* feed.post.list —— 动态（用户以自己身份发；或建议她发；不能代她发） */
function postCard(p, { full = false } = {}) {
  const md = p.mediaId && store.get('media', p.mediaId);
  return `<div class="card" data-test="post-${p.id}"><div class="row"><img class="av" src="${avatarOf(p.author)}" alt=""><div class="grow"><b>${esc(p.authorType === 'user' ? '我' : nameOf(p.author))}</b> ${p.authorType === 'user' ? badge('交流') : ''}<div class="small muted">${p.at}${p.fromSuggestion ? ' · 采纳了你的建议，用她自己的话写的' : ''}</div></div></div>
   <p>${esc(p.text)}</p>${md ? genImg(md) : ''}
   <div class="row small" style="margin-top:8px"><button class="chip" data-like="${p.id}" data-test="like-${p.id}">${p.liked ? '♥ 已赞' : '♡ 赞'} ${p.likes}</button>${full ? '' : `<a class="chip" href="#/feed/${p.id}">评论 ${p.comments.length}</a>`}</div>
   ${mgmt(`ID ${p.id} · 作者 ${p.author}（${p.authorType}）${p.refs ? ' · 引用 ' + p.refs.map(r => objLink(r, r)).join('、') : ''}${p.sourceAction ? ' · 来自行动 ' + objLink(p.sourceAction, p.sourceAction) : ''}`)}</div>`;
}
function bindLikes(el) {
  el.querySelectorAll('[data-like]').forEach(b => b.onclick = () => { const id = b.dataset.like; store.update(d => { const p = d.posts[id]; p.liked = !p.liked; p.likes += p.liked ? 1 : -1; }, { identity: '交流', summary: '点赞/取消点赞动态 ' + id, target: id }); });
}
export async function suggestPost(text) {
  const id = store.uid('ps_');
  store.update(d => { d.postSuggestions[id] = { id, charId: 'c1', text, state: '候选', at: hm() }; }, { identity: '交流', summary: '建议小遥发一条动态（由她决定）', target: id });
  await wait(T.step);
  const pid = store.uid('po_');
  store.update(d => { const s = d.postSuggestions[id]; s.state = '已确认'; s.postId = pid;
    d.posts[pid] = { id: pid, worldId: 'w1', author: 'c1', authorType: 'char', text: `听了你的建议～${text.replace(/^(帮我|建议你|你可以)?/, '')}（我自己写的）`, at: '10-15 ' + hm(), likes: 0, liked: false, comments: [], fromSuggestion: id }; },
    { identity: '交流', summary: '小遥采纳建议，用自己的话发了动态', target: pid });
  return pid;
}
export function renderFeed(el, ctx) {
  if (!store.isMock) return providerPage(() => import('./real-moments.js'), 'renderRealFeed',el,ctx);
  const posts = store.list('posts').sort((a, b) => (b.at > a.at ? 1 : -1));
  const sugs = store.list('postSuggestions').reverse();
  el.innerHTML = `<div class="grid g2" style="align-items:start"><div class="stack" data-test="feed-list">${posts.map(p => postCard(p)).join('') || '<div class="statebox"><h2>运行世界里还没有动态</h2></div>'}</div>
   <div class="stack"><form class="card tint-primary" data-mine><h3>${icon('feed')} 以「我」发一条 ${badge('交流')}</h3><textarea class="input" rows="3" name="t" placeholder="说点什么…（发在你自己的名下）" data-test="post-input"></textarea>
     <div class="row" style="margin-top:8px"><button type="button" class="btn" data-clear>清空</button><button class="btn primary" data-test="post-send">发布</button></div></form>
    <form class="card tint-sun" data-sug><h3>${icon('sparkle')} 建议小遥发一条</h3><p class="small muted">你不能以小遥的身份发动态。建议只是建议，发不发、怎么写由她决定。</p><textarea class="input" rows="2" name="t" data-test="sug-input">把今天画的明信片分享一下吧</textarea>
     <div class="row" style="margin-top:8px"><button class="btn accent" data-test="sug-send">发出建议</button></div></form>
    <div class="card"><h3>我的建议</h3>${sugs.map(s => `<div class="lrow small" data-test="sug-${s.id}"><span class="grow">${esc(s.text)}</span>${s.state === '候选' ? '<span class="typing">等她决定</span>' : s.postId ? `<a href="#/feed/${s.postId}">她发了</a>` : ''}${badge(s.state)}</div>`).join('') || '<p class="small muted">还没有建议</p>'}</div></div></div>`;
  const mine = el.querySelector('[data-mine]');
  mine.onsubmit = e => { e.preventDefault(); const t = mine.t.value.trim(); if (!t) return toast('先写点内容', 'err'); const id = store.uid('po_');
    store.update(d => { d.posts[id] = { id, worldId: 'w1', author: 'u1', authorType: 'user', text: t, at: '10-15 ' + hm(), likes: 0, liked: false, comments: [] }; }, { identity: '交流', summary: '用户以自己身份发布动态', target: id }); toast('已发布（以你自己的名义）', 'ok'); };
  mine.querySelector('[data-clear]').onclick = () => { mine.t.value = ''; };
  el.querySelector('[data-sug]').onsubmit = e => { e.preventDefault(); const t = e.target.t.value.trim(); if (!t) return; toast('建议已发出，等小遥决定'); suggestPost(t).then(() => toast('小遥采纳了建议并发了动态', 'ok')); };
  bindLikes(el);
}
export function renderPost(el, ctx) {
  if (!store.isMock) return providerPage(() => import('./real-moments.js'), 'renderRealPost',el,ctx);
  const p = store.get('posts', ctx.params.postId);
  if (!p) { el.innerHTML = `<div class="statebox"><h2>当前范围未找到这条动态</h2><a class="btn" href="#/feed">回到动态</a></div>`; return; }
  el.innerHTML = `${postCard(p, { full: true })}<div class="card" style="margin-top:12px"><h3>评论</h3><div class="list" data-test="comments">${p.comments.map(c => `<div class="lrow small"><img class="av sm" src="${avatarOf(c.author)}" alt=""><b>${esc(c.author === 'u1' ? '我' : nameOf(c.author))}</b><span class="grow">${esc(c.text)}</span><span class="muted">${c.at}</span></div>`).join('') || '<p class="small muted">还没有评论</p>'}</div>
   <form class="row" style="margin-top:8px" data-cf><input class="input grow" name="t" placeholder="以「我」评论…" data-test="comment-input"><button type="button" class="btn" data-clear>取消</button><button class="btn primary" data-test="comment-send">评论</button></form></div>`;
  const f = el.querySelector('[data-cf]');
  f.onsubmit = e => { e.preventDefault(); const t = f.t.value.trim(); if (!t) return; store.update(d => { d.posts[p.id].comments.push({ id: 'pc_' + Date.now().toString(36), author: 'u1', text: t, at: hm() }); }, { identity: '交流', summary: '用户评论动态 ' + p.id, target: p.id }); };
  f.querySelector('[data-clear]').onclick = () => { f.t.value = ''; };
  bindLikes(el);
}

/* settings.char.autonomy#outreach —— 主动触达（偏好展示） */
export function renderOutreach(el, ctx) {
  if (!store.isMock) return providerPage(() => import('./real-autonomy.js'), 'renderRealAutonomy',el,ctx);
  const pr = store.get('prefs').autonomy.c1;
  el.innerHTML = `<div class="banner info">${icon('info')} 仅展示：这里记录你对小遥主动找你的偏好。原型不会改变运行时的触达权限或频率。</div>
  <form class="card" style="margin-top:12px" data-of><div class="stack">
   <label class="field">主动触达频率<select class="select" name="主动触达" data-test="outreach-freq">${['很少', '适中', '经常'].map(v => `<option ${pr.主动触达 === v ? 'selected' : ''}>${v}</option>`).join('')}</select></label>
   <label class="field">免打扰时段<input class="input" name="免打扰" value="${esc(pr.免打扰)}"></label>
   <label class="toggle"><input type="checkbox" name="允许发动态" ${pr.允许发动态 ? 'checked' : ''}> 允许她在动态里提到你</label></div>
   <div class="row" style="margin-top:12px"><button type="reset" class="btn">撤销更改</button><button class="btn primary" data-test="outreach-save">记录偏好</button></div>
   <p class="small muted" data-test="outreach-saved">${pr.savedAt ? `上次记录：${pr.savedAt}（未改变运行时权限）` : ''}</p></form>`;
  el.querySelector('[data-of]').onsubmit = e => { e.preventDefault(); const f = e.target;
    store.update(d => Object.assign(d.prefs.autonomy.c1, { 主动触达: f.主动触达.value, 免打扰: f.免打扰.value, 允许发动态: f.允许发动态.checked, savedAt: hm() }), { identity: '所有者修改', summary: '记录主动触达偏好（仅展示，不改变运行时权限）', target: 'c1' }); toast('已记录偏好（不改变运行时权限）', 'ok'); };
}

/* inbox.notice.list */
export function renderInbox(el, ctx) {
  if (!store.isMock) return providerPage(() => import('./real-operations.js'), 'renderRealInbox',el,ctx);
  const tab = ctx.query.tab || '全部';
  const n = store.get('notices').filter(x => tab === '全部' || !x.read); const p = store.pending(ctx.charId);
  el.innerHTML = `<div class="grid g2"><div class="card">${Tabs({ value: tab, items: ['全部', '未读'].map(value => ({ value, label: value, href: `#/inbox?tab=${value}` })) })}
    <div class="list" style="margin-top:8px">${n.map(x => `<div class="lrow" data-test="notice-${x.id}"><a class="grow" href="${x.link}">${esc(x.text)}</a>${x.read ? '<span class="small muted">已读</span>' : `${badge('未读', 'plain')}<button class="btn ghost sm" data-one="${x.id}" data-test="read-${x.id}">标为已读</button>`}</div>`).join('') || '<p class="small muted">当前筛选没有通知</p>'}</div>
    <button class="btn sm" data-read data-test="read-all" style="margin-top:8px">全部标为已读</button></div>
    <div class="card"><h3>待处理事项</h3><p class="small muted">从现有对象派生，和通知分开。</p>${p.map(x => `<div class="lrow"><a class="grow" href="${x.link}">${esc(x.text)}</a></div>`).join('') || '<p class="muted small">当前范围未找到</p>'}<a class="small" href="#/diag/pending">全部待处理 →</a></div></div>`;
  el.querySelector('[data-read]').onclick = () => store.update(d => d.notices.forEach(x => x.read = true), { identity: '交流', summary: '通知全部已读' });
  el.querySelectorAll('[data-one]').forEach(b => b.onclick = () => store.update(d => { d.notices.find(x => x.id === b.dataset.one).read = true; }, { identity: '交流', summary: '通知已读 ' + b.dataset.one, target: null }));
}
