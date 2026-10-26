import { esc, icon, badge } from './controls.js';
export const STATES = ['正常', '空', '加载', '失败', '未接入', '冲突', '部分成功'];
/** 页内演示状态切换器（HTML），点击由 app 统一处理 */
export function stateSwitcher(cur) {
  return `<div class="statesw" data-test="statesw"><span>演示状态：</span>${STATES.map(s => `<button data-domain-ui="button" data-demo-state="${s}" class="${s === cur ? 'on' : ''}">${s}</button>`).join('')}</div>`;
}
/** 通用非正常态渲染 */
export function stateView(state, { zone = '', retry = true, emptyText = '当前范围未找到相关内容', kept = false } = {}) {
  const M = {
    '空': ['🌱', emptyText, '这是空态：不写“没有记录”，而说明范围。'],
    '加载': ['', '', ''],
    '失败': ['⚠️', '加载失败', '显示上次快照时间：10-15 09:40（可能过时）。'],
    '未接入': ['🔌', '该能力尚未接入', `接口状态：待核验。归属：${zone || '—'}。`],
    '冲突': ['🔀', '数据版本冲突', '他处已修改此对象。可查看新版本、比较差异、保留草稿或刷新后再提交。'],
    '部分成功': ['🧩', '部分成功', '4 项中 3 项已完成，1 项失败；已成功的不会重复执行。'],
  };
  if (state === '加载') return `<div data-domain-ui="card" class="card" data-test="state-加载"><div class="skel [width:40%]"></div><div class="skel"></div><div class="skel [width:80%]"></div><div class="skel [width:60%]"></div><p class="small muted">首次加载中…</p></div>`;
  const [big, t, d] = M[state] || ['', state, ''];
  const btns = state === '冲突' ? `<button data-domain-ui="button" class="btn" data-demo-state="正常">查看新版本</button><button data-domain-ui="button" class="btn" data-keep-draft data-test="keep-draft">保留草稿</button><button data-domain-ui="button" class="btn primary" data-demo-state="正常">刷新再提交</button>`
    : state === '失败' || state === '部分成功' ? (retry ? `<button data-domain-ui="button" class="btn primary" data-demo-state="正常">重试（幂等）</button><a data-domain-ui="button" class="btn" href="#/diag/task/t88">查看诊断</a>` : '')
    : state === '空' ? `<button data-domain-ui="button" class="btn" data-demo-state="正常">回到正常态</button>` : `<a data-domain-ui="button" class="btn" href="#/settings">去系统设置</a>`;
  const keptHtml = state === '冲突' && kept ? `<div data-domain-ui="alert" class="banner ok" data-test="draft-kept">${icon('check')} 草稿已保留在本地（未提交）。可稍后「刷新再提交」，他处的新版本不会被覆盖。</div>` : '';
  return `<div data-domain-ui="empty" class="statebox" data-test="state-${state}">${keptHtml}<div class="big">${big}</div><h2>${t}</h2><p class="muted">${d}</p><div class="row [justify-content:center]">${btns}</div></div>`;
}
/** 原型说明面板（顶栏“原型说明”切换显示） */
export function notesPanel(meta) {
  const iface = (meta.iface || []).map(([n, s]) => `${esc(n)} <b>${esc(s)}</b>`).join('；');
  return `<div class="notes" data-test="notes"><table>
  <tr><td>page key</td><td><code>${esc(meta.key)}</code>　${badge(meta.coverage, 'cov-' + meta.coverage)}　分区：${esc(meta.zone || '')}</td></tr>
  <tr><td>BP</td><td>${esc(meta.bp || '')}</td></tr>
  <tr><td>来源/触发</td><td>${esc(meta.source || '—')}</td></tr>
  <tr><td>接口状态</td><td>${iface || '—'}（现有/补齐/新增/待核验）</td></tr></table></div>`;
}
