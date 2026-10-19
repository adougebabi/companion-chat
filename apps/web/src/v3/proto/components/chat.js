import { esc, icon, badge } from './controls.js';
import { TextComposer } from './forms.js';

const PHASES = ['待请求', '请求中', '已接受', '执行中', '完成'];
export function GeneratedImage(md) {
  return `<div class="genimg" data-test="genimg">${badge('生成图·非事实')}<img src="${md.img}" alt="${esc(md.title)}">${md.overlay ? `<img class="ov" src="${md.overlay}" alt="">` : ''}<img class="ov2" src="/v3/assets/avatars/xiaoyao.svg" alt=""></div>`;
}
export function ActionCard(a, link) {
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
   <div class="small muted">阶段：<b data-test="act-phase">${a.phase}</b> · 关联 ${link(a.goalId || a.taskId || a.id, a.goalId ? '目标 ' + a.goalId : a.taskId)} · ${a.scheduleId ? link(a.scheduleId, '日程 ' + a.scheduleId) : ''}</div>
   <div class="row" style="margin-top:8px">${btns}<span class="spacer"></span><button class="btn ghost sm" data-act="detail">详情</button></div></div>`;
}
export function MessageBubble(m, { actionById, mediaById, link, mediaUrl, characterAvatar }) {
  if (m.from === 'divider') return `<div class="divider">${esc(m.text)}</div>`;
  if (m.from === 'sys') return `<div class="sysline">${badge('系统运维')} ${esc(m.text)} ${m.link ? link(m.link, '查看') : ''}</div>`;
  if (m.from === 'action') { const a = actionById(m.actionId); return a ? ActionCard(a, link) : ''; }
  const me = m.from === 'user'; const md = m.mediaId && mediaById(m.mediaId);
  const marks = [m.recall ? badge('回忆', 'plain') : '', m.corrected ? `<span class="corrected" data-test="corrected">${icon('check')} 已纠正 · 记忆 #${m.memoryRef}</span>` : '', m.feedback ? `<span class="corrected" style="background:var(--primary-soft);color:var(--primary-strong)">已告诉她</span>` : ''].join('');
  return `<div class="msg ${me ? 'me' : ''}" data-msg="${m.id}"><img class="av" src="${esc(me ? '/v3/assets/avatars/user.svg' : characterAvatar ?? '/v3/assets/avatars/xiaoyao.svg')}" alt="">
   <div><div class="bubble">${esc(m.text)}${mediaUrl ? (m.attachmentRefs || []).map(id => `<div class="genimg"><img src="${esc(mediaUrl(id))}" alt="会话媒体" loading="lazy"></div>`).join('') : ''}${md ? `<div style="margin-top:8px">${GeneratedImage(md)}</div>` : ''}</div>
   <div class="meta">${m.at || ''} ${m.identity && m.identity !== '交流' ? badge(m.identity) : ''} ${marks}
     <span class="tools">${!me ? `<button class="chip" data-wrong="${m.id}" data-test="wrong-${m.id}">${icon('flag')} 这条不对</button>` : ''}<button class="chip" data-source="${m.id}">来源</button></span></div></div></div>`;
}

export function MessageLog(messages, options = {}) {
  if (options.variant === 'assistant') return `<div class="log">${messages.map(message => message.u ? `<div class="u">${esc(message.u)}</div>` : `<div class="a">${esc(message.a)}</div>`).join('')}</div>`;
  return `<div class="stream" data-test="stream">${messages.map(message => MessageBubble(message, options)).join('')}</div>`;
}
export function TypingMessage() {
  return `<img class="av" src="/v3/assets/avatars/xiaoyao.svg" alt=""><div><div class="bubble typing"></div><div class="meta">正在输入…</div></div>`;
}
export function ChatPanel({ character, summary, messages, messageOptions, sidebar }) {
  return `<div class="chatwrap"><section class="chatcol">
    <div class="chathead"><img class="av" src="${character.avatar}" alt=""><div class="grow"><b>${esc(character.name)}</b><div class="small muted">${badge('交流')} 远程私聊 · 你不在场</div></div>
      <a class="btn ghost sm" href="#/c/${character.id}/chat/history">历史</a><a class="btn ghost sm" href="#/c/${character.id}/chat/call">${icon('play')} 通话</a></div>
    ${summary}
    ${MessageLog(messages, messageOptions)}
    ${TextComposer({ placeholder: '和小遥说点什么…' })}
  </section>
  ${sidebar}</div>`;
}
