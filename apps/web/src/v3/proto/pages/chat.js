// chat.thread.view —— 会话主页（相处体验 BP-02）
import * as store from '../store.js';
import { esc, icon, badge, drawer, modal, toast, objLink, requireOwner, confirm, sourceDrawer } from '../ui.js';
import { requestBuy, withdraw, addMsg } from './flows.js';

const PHASES = ['待请求', '请求中', '已接受', '执行中', '完成'];
export function genImg(md) {
  const ownerArt = md?.source === "owner_upload" || md?.kind === "visual_identity";
  const mark = ownerArt || md?.nonFact === false ? "" : badge("生成图·非事实");
  return `<div class="genimg" data-test="genimg">${mark}<img src="${md.img}" alt="${esc(md.title)}">${md.overlay ? `<img class="ov" src="${md.overlay}" alt="">` : ''}<img class="ov2" src="/v3/assets/avatars/xiaoyao.svg" alt=""></div>`;
}
function actionCard(a) {
  const i = PHASES.indexOf(a.phase); const fail = a.phase === '失败';
  const btns = {
    '待请求': `<button class="btn primary sm" data-act="req" data-test="act-request">${icon('bag')} 发出行动请求</button><span class="small muted">需小遥接受后才执行</span>`,
    '请求中': `<span class="small typing">等待小遥决定</span><button class="btn sm danger" data-act="withdraw">撤回</button>`,
    '已接受': `<span class="small">小遥已接受：${esc(a.decision || '')}</span>`,
    '执行中': `<span class="small typing">执行中 · 碎花裙入库中</span>`,
    '完成': `<a class="btn sm teal" href="#/c/c1/life/wardrobe?item=${a.itemId}">${icon('shirt')} 去衣柜看看</a>`,
    '已撤回': `<span class="small muted">已撤回</span><button class="btn sm" data-act="req">重新请求</button>`,
    '失败': `<span class="small">${esc(a.reason || '')}</span><a class="btn sm" href="#/diag/task/${a.taskId}?from=chat.thread.view">查看诊断 ${a.taskId}</a>`,
  }[a.phase] || '';
  return `<div class="actcard" data-action="${a.id}" data-test="actcard-${a.id}"><div class="row">${icon('bag')}<b class="grow">${esc(a.title)}</b>${badge('行动请求')}${badge(a.state)}</div>
   <div class="steps">${PHASES.map((p, k) => `<span class="${fail && k >= 2 ? 'fail' : k <= i ? 'on' : ''}" title="${p}"></span>`).join('')}</div>
   <div class="small muted">阶段：<b data-test="act-phase">${a.phase}</b> · 关联 ${objLink(a.goalId || a.taskId || a.id, a.goalId ? '目标 ' + a.goalId : a.taskId)} · ${a.scheduleId ? objLink(a.scheduleId, '日程 ' + a.scheduleId) : ''}</div>
   <div class="row" style="margin-top:8px">${btns}<span class="spacer"></span><button class="btn ghost sm" data-act="detail">详情</button></div></div>`;
}
function msgHtml(m) {
  if (m.from === 'divider') return `<div class="divider">${esc(m.text)}</div>`;
  if (m.from === 'sys') return `<div class="sysline">${badge('系统运维')} ${esc(m.text)} ${m.link ? objLink(m.link, '查看') : ''}</div>`;
  if (m.from === 'action') { const a = store.get('actions', m.actionId); return a ? actionCard(a) : ''; }
  const me = m.from === 'user'; const md = m.mediaId && store.get('media', m.mediaId);
  const marks = [m.recall ? badge('回忆', 'plain') : '', m.corrected ? `<span class="corrected" data-test="corrected">${icon('check')} 已纠正 · 记忆 #${m.memoryRef}</span>` : '', m.feedback ? `<span class="corrected" style="background:var(--primary-soft);color:var(--primary-strong)">已告诉她</span>` : ''].join('');
  return `<div class="msg ${me ? 'me' : ''}" data-msg="${m.id}"><img class="av" src="${me ? '/v3/assets/avatars/user.svg' : '/v3/assets/avatars/xiaoyao.svg'}" alt="">
   <div><div class="bubble">${esc(m.text)}${md ? `<div style="margin-top:8px">${genImg(md)}</div>` : ''}</div>
   <div class="meta">${m.at || ''} ${m.identity && m.identity !== '交流' ? badge(m.identity) : ''} ${marks}
     <span class="tools">${!me ? `<button class="chip" data-wrong="${m.id}" data-test="wrong-${m.id}">${icon('flag')} 这条不对</button>` : ''}<button class="chip" data-source="${m.id}">来源</button></span></div></div></div>`;
}
function summary(c) {
  const next = store.list('schedule', s => s.charId === c.id && s.state === '计划')[0];
  return `<details class="summary card tint-primary" data-test="chat-summary" style="margin:8px 12px 0;padding:8px 12px"><summary>${icon('now')} 此刻：${esc(c.doing)} · ${esc(store.get('places', c.locationId)?.name || '未知')} <span class="small muted">（展开）</span></summary>
   <div class="kv small" style="margin-top:8px"><dt>位置</dt><dd>${objLink(c.locationId)}（远程私聊 · 你不在场）</dd><dt>下一项</dt><dd>${next ? `${next.time} ${esc(next.title)} ${badge('计划')}` : '—'}</dd>
   <dt>心情</dt><dd>${esc(c.mood.value)} ${badge('推测')} 依据 ${c.mood.evidence.map(e => objLink(e, e)).join('、')}</dd></div><a class="small" href="#/c/${c.id}/life/now?from=chat.thread.view">打开 生活·此刻 →</a></details>`;
}
let streaming = false; let openedFor = '';
async function send(text, el) {
  if (!text.trim() || streaming) return;
  streaming = true;
  const stream = el.querySelector('.stream');
  try {
    if (typeof store.submitExchange !== 'function') throw new Error('对话发送未接入');
    await store.submitExchange(text, el);
  } catch (error) {
    const banner = document.createElement('div');
    banner.className = 'banner err';
    banner.textContent = error?.userMessage || error?.message || '发送失败';
    stream?.prepend(banner);
  } finally { streaming = false; }
}
export function actionDrawer(id) {
  const a = store.get('actions', id);
  drawer({ title: `${icon('bag')} 行动详情 · ${id}`, body: `<p class="small muted">page key chat.action.detail</p>
   <div class="kv"><dt>请求</dt><dd>${esc(a.title)} ${badge('行动请求')}</dd><dt>状态</dt><dd>${badge(a.state)} 阶段 ${a.phase}</dd><dt>角色决定</dt><dd>${esc(a.decision || '—')}</dd>
   <dt>地点</dt><dd>${objLink(a.placeId)}</dd><dt>日程</dt><dd>${a.scheduleId ? objLink(a.scheduleId) : '—'}</dd><dt>目标</dt><dd>${a.goalId ? objLink(a.goalId) : '—'}</dd>
   <dt>产出</dt><dd>${a.itemId ? objLink(a.itemId) : '—'}</dd><dt>任务</dt><dd>${a.taskId ? objLink(a.taskId) : '—'}</dd><dt>记录</dt><dd>${(a.log || []).map(esc).join(' → ') || '—'}</dd></div>` });
}
/** chat.correction.edit —— 聊天事实纠错（J-02） */
export function openCorrection(msgId, onDone) {
  const msg = store.get('messages', msgId); const mem = msg.memoryRef && store.get('memories', msg.memoryRef);
  let mode = 'tell'; let base = mem?.version; let draft = mem ? '海边书店暂停装修中（未停业）' : '';
  const d = drawer({ title: `${icon('flag')} 这条不对`, body: '', onClose: () => onDone?.() }); d.el.dataset.test = 'correction';
  const diff = () => `<div class="diff" data-test="diff"><div class="del">- ${esc(store.get('memories', mem.id).content)}</div><div class="add">+ ${esc(draft)}</div></div>`;
  function paint(conflict) {
    const cur = mem && store.get('memories', mem.id);
    d.setBody(`<p class="small muted">page key chat.correction.edit · ${badge('角色范围')}</p>
     <div class="card tint-sun"><div class="small muted">被质疑的陈述（原文不会被改写）</div><b>“${esc(msg.text)}”</b><div class="small">${msg.at}</div></div>
     <h3 style="margin-top:14px">系统识别的关联事实</h3>
     ${cur ? `<div class="card"><div class="row"><b class="grow">记忆 #${cur.id}</b>${badge(cur.state)}<span class="small muted">v${cur.version}</span></div><p data-test="mem-content">${esc(cur.content)}</p>
       <div class="small muted">来源 ${objLink(cur.sourceMsg)} · 地点 ${objLink(cur.relatedPlace)} · 人物 ${objLink(cur.relatedNpc)}</div>
       ${cur.pendingRevision ? `<div class="banner info small">候选修订（来自交流）：${esc(cur.pendingRevision)} ${badge('候选')}</div>` : ''}</div>` : '<div class="banner info">无关联事实，将仅作为对话反馈。</div>'}
     <div class="tabs" style="margin-top:14px"><button class="tab ${mode === 'tell' ? 'on' : ''}" data-mode="tell" data-test="mode-tell">告诉她 ${badge('交流')}</button>${cur ? `<button class="tab ${mode === 'owner' ? 'on' : ''}" data-mode="owner" data-test="mode-owner">所有者修改 ${badge('所有者修改')}</button>` : ''}</div>
     ${mode === 'tell' ? `<p class="small muted">以“交流”身份告诉小遥，她自己决定是否更新记忆（不直接改事实）。</p><textarea class="input" rows="3" data-tell>其实海边书店只是暂停装修，没有停业哦。</textarea>`
      : `${conflict ? `<div class="banner err" data-test="conflict">${icon('warn')} 版本冲突：你打开时是 v${base}，记忆已被他处修改为 v${cur.version}。</div>
          <div class="card tint-accent small"><b>他处的新版本 v${cur.version}：</b>${esc(cur.content)}</div>
          <div class="row" style="margin:8px 0"><button class="btn sm" data-keep>保留草稿</button><button class="btn sm primary" data-refresh data-test="refresh-resubmit">刷新再提交</button></div>` : ''}
        <label class="field">修改为（作用于记忆 #${cur.id}，v${base}）<input class="input" data-draft value="${esc(draft)}"></label>
        <h3 style="margin-top:12px">差异预览</h3>${diff()}
        <p class="small muted">保存需二次确认并写入审计；原消息挂“已纠正”标记但不改写原文。</p>
        <button class="btn ghost sm" data-other data-test="simulate-conflict">${icon('swap')} 模拟他处修改（版本冲突演示）</button>`}`);
    d.setFooter(mode === 'tell' ? `<button class="btn" data-cancel>取消</button><button class="btn primary" data-tellgo data-test="tell-send">${icon('send')} 发送给小遥</button>`
      : `<button class="btn" data-cancel data-test="corr-cancel">取消</button><button class="btn accent" data-save data-test="corr-save">保存修改</button>`);
    const q = s => d.el.querySelector(s);
    d.el.querySelectorAll('[data-mode]').forEach(b => b.onclick = () => { mode = b.dataset.mode; paint(); });
    q('[data-cancel]').onclick = () => { d.close(); toast('已取消，未做任何修改'); };
    q('[data-draft]')?.addEventListener('input', e => { draft = e.target.value; q('.diff').outerHTML = diff(); });
    q('[data-other]') && (q('[data-other]').onclick = () => { store.update(x => { const m = x.memories[mem.id]; m.version++; m.content = '海边书店已经停业（小美说的）'; m.revisions.push({ v: m.version, at: '刚刚', text: m.content, by: '运行时', note: '他处修改（冲突演示）' }); }, { identity: '系统运维', summary: '运行时修改记忆 #m31（冲突演示）', target: mem.id }); toast('已模拟：记忆在他处被修改'); });
    q('[data-keep]') && (q('[data-keep]').onclick = () => toast('草稿已保留，可继续编辑'));
    q('[data-refresh]') && (q('[data-refresh]').onclick = () => { base = store.get('memories', mem.id).version; paint(); toast('已刷新到最新版本，请确认差异后再提交'); });
    q('[data-tellgo]') && (q('[data-tellgo]').onclick = async () => {
      const t = q('[data-tell]').value; d.close();
      store.update(x => { x.messages[msgId].feedback = true; if (mem) x.memories[mem.id].pendingRevision = '海边书店可能只是暂停装修（用户告知）'; }, { identity: '交流', summary: '用户告诉小遥：' + t, target: msgId });
      await send(t, document.querySelector('#page'));
    });
    q('[data-save]') && (q('[data-save]').onclick = async () => {
      if (!(await requireOwner())) return;
      const cur2 = store.get('memories', mem.id);
      if (cur2.version !== base) { paint(true); return; }
      if (!(await confirm({ title: '所有者修改记忆 #' + mem.id, text: `将“${esc(cur2.content)}”改为“${esc(draft)}”。` }))) return;
      store.update(x => { const m = x.memories[mem.id]; m.version++; m.content = draft; m.state = '已确认'; delete m.pendingRevision; m.revisions.push({ v: m.version, at: '刚刚', text: draft, by: '所有者修改', note: '聊天纠错 ' + msgId }); x.messages[msgId].corrected = true; }, { identity: '所有者修改', summary: `修改记忆 #${mem.id} → ${draft}`, target: mem.id });
      addMsg({ from: 'sys', text: `所有者修改了记忆 #${mem.id}：${draft}`, link: mem.id, identity: '系统运维' });
      d.close(); toast('已保存：记忆 #' + mem.id + ' 已更新', 'ok');
    });
  }
  paint();
  return d;
}
function side(c) {
  return `<aside class="chatside stack" data-test="chat-side"><div class="card"><h3>${icon('target')} 进行中</h3>${store.list('goals', g => g.charId === c.id).map(g => `<div class="lrow small" data-test="side-goal-${g.id}">${badge(g.type, 'plain')}<span class="grow">${objLink(g.id, g.title)}</span>${badge(g.state)}</div>`).join('')}</div>
    <div class="card tint-teal"><h3>${icon('shirt')} 当前穿着</h3><div class="outfit" data-test="side-outfit">${store.get('outfit').c1.current.map(i => `<a href="#/c/c1/life/wardrobe?item=${i}" title="${esc(store.get('items', i).name)}"><img src="${store.get('items', i).img}" alt="${esc(store.get('items', i).name)}"></a>`).join('')}</div></div>
    <div class="card"><h3>${icon('brain')} 相关记忆</h3>${['m31', 'm32'].map(id => `<div class="lrow small"><a class="grow" href="#/c/c1/growth/memory/${id}" data-test="side-mem-${id}">${esc(store.get('memories', id).content)}</a></div>`).join('')}
    <a class="small" href="#/c/${c.id}/chat/corrections">纠错记录 →</a></div></aside>`;
}
export function renderChat(el, ctx) {
  const c = store.get('characters', ctx.charId);
  if (c.status !== '正常') { el.innerHTML = `<div class="banner ${c.status === '异常' ? 'err' : 'warn'}">${icon('warn')} ${esc(c.name)} ${c.status}：${esc(c.statusNote || '')}。${c.status === '异常' ? '<a href="#/diag">去运行诊断</a>' : ''}</div>` + `<div class="statebox"><img class="av lg" src="${c.avatar}" alt=""><h2>${esc(c.name)}暂时无法对话</h2><p class="muted">完整演示请切换到小遥。</p><a class="btn primary" href="#/c/c1/chat">切换到小遥</a></div>`; return; }
  const msgs = store.list('messages', m => m.charId === c.id);
  el.innerHTML = `<div class="chatwrap"><section class="chatcol">
    <div class="chathead"><img class="av" src="${c.avatar}" alt=""><div class="grow"><b>${esc(c.name)}</b><div class="small muted">${badge('交流')} 远程私聊 · 你不在场</div></div>
      <a class="btn ghost sm" href="#/c/${c.id}/chat/history">历史</a><a class="btn ghost sm" href="#/c/${c.id}/chat/call">${icon('play')} 通话</a></div>
    ${summary(c)}
    <div class="stream" data-test="stream">${msgs.map(msgHtml).join('')}</div>
    <div class="chips">${['帮我买那条碎花裙', '今天心情怎么样？', '给我拍张照', '海边书店现在怎样？'].map(t => `<button class="chip" data-chip="${t}">${t}</button>`).join('')}</div>
    <form class="composer" data-test="composer"><button type="button" class="btn ghost icon-only" title="多模态附件（未接入）" disabled>${icon('image')}</button><input class="input" name="t" placeholder="和小遥说点什么…" autocomplete="off" data-test="chat-input"><button class="btn primary" data-test="chat-send">${icon('send')} 发送</button></form>
  </section>
  ${side(c)}</div>`;
  const stream = el.querySelector('.stream'); stream.scrollTop = 1e9;
  el.querySelector('form').onsubmit = e => { e.preventDefault(); const i = e.target.t; send(i.value, el); i.value = ''; };
  el.querySelectorAll('[data-chip]').forEach(b => b.onclick = () => send(b.dataset.chip, el));
  stream.onclick = e => {
    const w = e.target.closest('[data-wrong]'); if (w) return openCorrection(w.dataset.wrong);
    const act = e.target.closest('[data-act]'); if (!act) return; const id = act.closest('[data-action]').dataset.action;
    if (act.dataset.act === 'req') { if (id === 'a_buy') requestBuy(); toast('已发出行动请求，等待小遥接受'); }
    if (act.dataset.act === 'withdraw') withdraw(id);
    if (act.dataset.act === 'detail') actionDrawer(id);
  };
  if (ctx.query.say) el.querySelector('[name=t]').value = ctx.query.say;
  if (ctx.query.correct && openedFor !== location.hash) { openedFor = location.hash; setTimeout(() => openCorrection(ctx.query.correct), 50); }
}
export function refreshStream(ctx) {
  const el = document.querySelector('#page[data-page="chat.thread.view"]'); if (!el || streaming) return;
  const c = store.get('characters', 'c1');
  const s = el.querySelector('.stream'); if (!s) return;
  s.innerHTML = store.list('messages', m => m.charId === ctx.charId).map(msgHtml).join(''); s.scrollTop = 1e9;
  const sd = el.querySelector('.chatside'); if (sd) sd.outerHTML = side(c);
  const sum = el.querySelector('.summary'); if (sum) { const open = sum.open; sum.outerHTML = summary(c); if (open) el.querySelector('.summary').open = true; }
}
