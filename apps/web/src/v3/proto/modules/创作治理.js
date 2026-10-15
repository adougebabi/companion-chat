// 分区：创作治理（BP-01/15/16/17/18）
import { register } from './registry.js';
import * as store from '../store.js';
import { badge, esc, objLink, toast } from '../ui.js';
import { renderStudioEdit } from '../pages/studio-edit.js';
import { actionDrawer } from '../pages/chat.js';
import * as G from '../pages/govern.js';
const Z = '创作治理';
const R = (route, key, title, bp, coverage, render, extra = {}) => register({ route, key, title, bp, zone: Z, coverage, scope: '全局定义', render, ...extra });
R('/auth', 'auth.signin', '登录/注册', 'BP-01', '可操作', G.renderAuth, { source: '原型模拟登录，不连接真实账号' });
R('/library', 'library.home', '角色库', 'BP-01', '可操作', G.renderLibrary);
R('/library/create', 'library.char.create', '新建角色向导', 'BP-01', '可操作', G.renderCreate);
R('/studio', 'studio.home', '创作中心', 'BP-15', '可操作', G.renderStudioHome);
R('/studio/edit/:resId', 'studio.resource.edit', '资源编辑', 'BP-15', '可操作', renderStudioEdit, { iface: [['资源 CRUD', '补齐'], ['发布版本', '补齐'], ['试生成', '新增'], ['应用到实例', '新增']], source: '创作中心首页；角色资料“有新版本”提示；通知 nt_3' });
R('/studio/preview', 'studio.preview', '试生成沙盒', 'BP-15/14', '可操作', G.renderPreview, { source: '结果只写 studioDrafts，标「创作预览·不进入生活」' });
R('/studio/apply', 'studio.apply', '应用到实例', 'BP-15/05', '可操作', G.renderApply);
R('/studio/template', 'studio.event.template', '事件模板', 'BP-11.2-3', '仅展示', G.renderTemplate);
R('/settings', 'settings.global', '系统设置', 'BP-16', '仅展示', G.renderSettings, { source: '仅记录偏好，不改变运行时权限' });
R('/c/:charId/settings/autonomy', 'settings.char.autonomy', '角色自治与触达设置', 'BP-16', '仅展示', G.renderAutonomy, { scope: '角色范围', source: '权限只读展示；偏好仅记录' });
R('/diag', 'diag.home', '运行诊断', 'BP-17', '可操作', G.renderDiag);
R('/diag/task/:taskId', 'diag.task.detail', '诊断任务详情', 'BP-17', '可操作', (el, ctx) => {
  const t = store.get('tasks', ctx.params.taskId); if (!t) { el.innerHTML = '<div class="statebox"><h2>当前范围未找到该任务</h2></div>'; return; }
  const C = { ok: '已确认', failed: '失败', pending: '未知' };
  el.innerHTML = `<div class="card"><h2>${esc(t.title)} ${badge(t.state)}</h2><p class="small">行动 ${objLink(t.actionId)} · 执行次数 ${t.runs} <button class="btn ghost sm" data-act data-test="task-action">行动详情</button></p>${t.steps.map(s => `<div class="lrow" data-test="step-${s.n}"><b>${s.n}</b><span class="grow">${esc(s.name)} ${s.err ? `<span class="small muted">${esc(s.err)}</span>` : ''}</span>${badge(C[s.state])}</div>`).join('')}
  ${t.state === '失败' ? `<button class="btn primary" data-retry data-test="retry">重试（只执行步骤 3-4，幂等）</button>` : ''}<details class="mgmt small muted"><summary>管理信息</summary>${t.id} · action ${t.actionId}</details></div>`;
  el.querySelector('[data-act]').onclick = () => actionDrawer(t.actionId);
  el.querySelector('[data-retry]')?.addEventListener('click', () => { store.update(d => { const x = d.tasks[t.id]; x.runs++; x.steps.forEach(s => { if (s.state !== 'ok') { s.state = 'ok'; delete s.err; } }); x.state = '已确认'; d.actions[x.actionId].state = '已确认'; d.actions[x.actionId].phase = '完成'; d.items.it_bouquet.status = '拥有'; d.schedule.s_flower.state = '已确认'; d.schedule.s_flower.fact = '重试后取到花束'; d.schedule.s_flower.factState = '已确认'; }, { identity: '系统运维', summary: `重试 ${t.id}：仅步骤 3-4，步骤 1-2 未重复`, target: t.id }); toast('重试成功：步骤 1-2 未重复执行', 'ok'); });
});
R('/diag/audit', 'diag.audit', '操作审计日志', 'BP-17', '仅展示', G.renderAudit);
R('/diag/pending', 'diag.pending.list', '待处理事项', 'BP-17.7', '仅展示', G.renderPending, { source: '由 proposals/resources/tasks 派生，不另存' });
R('/data', 'data.tools', '数据工具', 'BP-18', '可操作', G.renderData, { source: '配置包可导出/导入为草稿；业务备份 P2 未完成；回档 P3 只有说明' });
R('/data/revisions', 'data.object.revisions', '对象修订记录', 'BP-18', '仅展示', G.renderRevisions);
R('/data/rollback', 'data.rollback', '回档说明（P3）', 'BP-18', '占位', G.renderRollback);
