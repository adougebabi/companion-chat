// life.now —— 生活·此刻（生活成长 BP-05/06）
import * as store from '../store.js';
import { esc, icon, badge, objLink, drawer } from '../ui.js';
import { actionDrawer } from './chat.js';
import { charStatus } from './common.js';

export function renderNow(el, ctx, state = '正常') {
  const c = store.get('characters', ctx.charId);
  if (c.id !== 'c1' && charStatus(el, ctx, '此刻')) return;
  if (state === '空') { el.innerHTML = `<div class="statebox" data-test="state-空"><div class="big">🌙</div><h2>${esc(c.name)}尚未开始生活</h2><p class="muted">运行世界未启动（当前范围未找到此刻状态）。</p><a class="btn primary" href="#/studio">去创作中心启动</a></div>`; return; }
  const place = store.get('places', c.locationId);
  const sched = store.list('schedule', s => s.charId === c.id);
  const next = sched.filter(s => s.state === '计划');
  const promises = store.list('goals', g => g.charId === c.id && (g.type === '约定' || g.type === '意向') && g.state !== '取消');
  const outfit = store.get('outfit').c1;
  el.innerHTML = `
  ${state === '失败' ? `<div class="banner err" data-test="state-失败">${icon('warn')} 刷新失败，显示上次快照（10-15 09:40），可能过时。<button class="btn sm" data-demo-state="正常">重试</button></div>` : ''}
  <section class="hero" data-test="now-hero"><img class="scene" src="${place.scene}" alt="${esc(place.name)}">
    <div class="over"><img class="av lg" src="${c.avatar}" alt=""><div class="grow"><div class="small muted">${esc(c.name)} 正在</div><h2 style="margin:2px 0" data-test="now-doing">${esc(c.doing)}</h2>
      <div class="row small">${badge('已确认')} ${icon('pin')} ${objLink(place.id)} <span class="muted">· 远程查看，不会移动她</span></div></div>
      ${c.doingActionId ? `<button class="btn sm" data-actdetail="${c.doingActionId}">行动详情</button>` : ''}</div></section>
  <div class="grid g3" style="margin-top:16px">
    <div class="card"><h3>${icon('calendar')} 下一项日程</h3>${next.length ? next.map(s => `<div class="lrow"><b class="small" style="width:86px">${s.time}</b><div class="grow">${esc(s.title)}<div class="small muted">${objLink(s.placeId)}</div></div>${badge(s.state)}</div>`).join('') : '<p class="muted small">当前范围未找到计划</p>'}
      ${sched.filter(s => s.adjustedBy).map(s => `<div class="banner warn small">${esc(s.title)} ${badge(s.state)} · 因 ${objLink(s.adjustedBy)} 调整</div>`).join('')}<a class="small" href="#/c/${c.id}/life/schedule?from=life.now">完整日程 →</a></div>
    ${state === '未接入' ? `<div class="card" data-test="state-未接入"><h3>${icon('heart')} 心情 / 精力</h3><p class="muted small">心情模块未接入，已隐藏该卡（接口待核验）。</p></div>` :
    `<div class="card tint-sun" data-test="mood"><h3>${icon('heart')} 心情 / 精力</h3>
      <div class="lrow"><span class="grow">心情：<b>${esc(c.mood.value)}</b></span>${badge('推测')}</div><div class="small muted">依据：${c.mood.evidence.map(e => `<button class="chip" data-source="${e}" data-test="evidence">${e}</button>`).join('、')}</div>
      <div class="lrow"><span class="grow">精力：<b>${esc(c.energy.value)}</b></span>${badge('推测')}</div><div class="small muted">依据：${c.energy.evidence.map(e => `<button class="chip" data-source="${e}">${e}</button>`).join('、')}</div></div>`}
    <div class="card tint-teal"><h3>${icon('shirt')} 当前穿着 <span class="small muted">自 ${outfit.since}</span></h3><div class="outfit" data-test="now-outfit">${outfit.current.map(i => `<a href="#/c/c1/life/wardrobe?item=${i}&from=life.now" title="${esc(store.get('items', i).name)}"><img src="${store.get('items', i).img}" alt="${esc(store.get('items', i).name)}"></a>`).join('')}</div>
      <div class="small" style="margin-top:6px">${outfit.current.map(i => esc(store.get('items', i).name)).join('、')}</div><a class="small" href="#/c/c1/life/wardrobe?from=life.now">打开衣柜 →</a></div>
  </div>
  <div class="grid g2" style="margin-top:16px">
    <div class="card"><h3>${icon('target')} 待兑现的约定与意向</h3>${promises.map(g => `<div class="lrow">${badge(g.type, 'plain')}<span class="grow">${objLink(g.id, g.title)} ${g.due ? `<span class="small muted">${g.due}</span>` : ''}</span>${badge(g.state)}</div>`).join('')}</div>
    <div class="card"><h3>${icon('people')} 此地附近</h3>${store.list('npcs', n => n.placeId === c.locationId).map(n => `<div class="lrow"><img class="av sm" src="${n.avatar}" alt="">${objLink(n.id)} <span class="small muted">${esc(n.role)}</span></div>`).join('') || '<p class="small muted">当前范围未找到在场人物</p>'}
      <div class="row" style="margin-top:8px"><a class="btn sm" href="#/c/${c.id}/chat?say=${encodeURIComponent('你现在在' + place.name + '做什么呀？')}">${icon('chat')} 在对话中问问她</a><a class="btn sm" href="#/c/c1/world/scene?from=life.now">远程查看场景</a></div></div>
  </div>`;
  el.querySelectorAll('[data-actdetail]').forEach(b => b.onclick = () => actionDrawer(b.dataset.actdetail));
}
