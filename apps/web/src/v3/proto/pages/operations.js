import {providerPage} from './provider-page.js';
import * as store from '../store.js';
import { esc, badge, toast, confirm, drawer } from '../ui.js';
import { Field, Button, SearchForm } from '../components/forms.js';
import { RecordList, JsonDetails } from '../components/records.js';
import { ConflictPanel } from '../components/revisions.js';
import { mockOperations, configurationSections, diagnosticSections } from '../services.js';

const drafts = new Map();
store.subscribe(event => { if (event.type === 'reset') drafts.clear(); });
export function settingsLinks() { return `<div class="card" style="margin-top:16px"><h3>完整设置与权限</h3><div class="row">${configurationSections.map(section => `<a class="btn" href="#/settings/${section.id}">${section.title}</a>`).join('')}</div><p class="small muted">${store.isMock?"全部使用隔离 Mock 配置，不向任何服务发送请求。":"读取和保存真实服务端配置；连接测试与业务运行结果分别显示。"}</p></div>`; }
export function diagnosticsLinks() { return `<div class="card" style="margin-top:16px"><h3>运行记录与控制</h3><div class="row">${diagnosticSections.map(section => `<a class="btn" href="#/diag/${section.id}">${section.title}</a>`).join('')}</div></div>`; }

export function renderConfiguration(el, ctx) {
  if(!store.isMock)return providerPage(()=>import("./real-configuration.js"),"renderRealConfiguration",el,ctx);
  const sectionId = ctx.def.mockSection, scope = ctx.params.charId || '';
  let saved = mockOperations.configuration(sectionId, scope);
  if (!drafts.has(saved.key)) drafts.set(saved.key, { values: { ...saved.values }, base: saved.version, dirty: false, conflict: false });
  const draft = drafts.get(saved.key);
  function paint() {
    saved = mockOperations.configuration(sectionId, scope);
    el.innerHTML = `<div class="row" style="margin-bottom:12px"><span class="small muted">${scope ? '角色范围' : '全局配置'} · v${draft.base} · ${badge('Mock', 'plain')}</span></div><div class="grid g2" style="align-items:start"><form class="card stack" data-config-form><h2>${esc(saved.section.title)}</h2>${draft.conflict ? ConflictPanel({ baseVersion: draft.base, currentVersion: saved.version }) : ''}
      ${saved.section.fields.map(([name, label, , type = 'text', options]) => type === 'boolean' ? `<label class="toggle"><input type="checkbox" data-config="${name}" ${draft.values[name] ? 'checked' : ''}> ${esc(label)}</label>` : Field({ label, name, type: type === 'json' ? 'textarea' : type, options, rows: type === 'json' ? 8 : 3, value: type === 'password' ? '' : draft.values[name], attrs: { 'data-config': name, required: type !== 'password' } })).join('')}
      ${sectionId === 'media-workflows' ? '<label class="field">导入工作流 JSON 到草稿<input class="input" type="file" accept="application/json,.json" data-workflow-file></label>' : ''}<div class="row"><button class="btn primary">保存 Mock 配置</button>${Button({ label: '撤销未保存修改', attrs: { 'data-revert-config': true } })}</div><div data-config-feedback role="status"></div></form>
      <div class="stack"><div class="card"><h3>当前保存值</h3><p class="small muted">本页表单是草稿。保存、测试和真实运行效果分开；当前阶段仅演示 Mock。</p>${JsonDetails(saved.values, '已保存的 Mock 配置（密钥不回显）')}<p class="small muted">版本 v${saved.version} · ${draft.dirty ? '有未保存更改' : '与保存值一致'}</p></div><div class="card"><h3>配置测试（Mock）</h3><label class="field">演示结果<select class="select" data-test-outcome><option>成功</option><option>失败</option><option>未配置</option></select></label><button class="btn" style="margin-top:12px" data-config-test>测试已保存配置（Mock）</button><div data-test-result style="margin-top:12px"></div></div>${sectionId === 'models' ? '<div class="card"><h3>已确认的模型分工</h3><p>Qwen 负责认知与通用决策；Cydonia 负责私聊表达。当前都是 Mock 字段，不修改 Agent 或对话循环。</p><button class="btn" data-discover-models>发现模型（Mock）</button><div data-model-options></div></div>' : ''}<div class="card"><h3>相关入口</h3><a class="btn" href="#/diag">运行诊断</a><a class="btn ghost" href="#/diag/pending">待处理事项</a></div></div></div>`;
    el.querySelectorAll('[data-config]').forEach(input => input.oninput = input.onchange = () => { draft.values[input.dataset.config] = input.type === 'checkbox' ? input.checked : input.value; draft.dirty = true; });
    el.querySelector('[data-config-form]').onsubmit = event => { event.preventDefault(); try { saved = mockOperations.saveConfiguration(sectionId, scope, draft.values, draft.base); draft.values = { ...saved.values }; draft.base = saved.version; draft.dirty = false; draft.conflict = false; paint(); toast('Mock 配置已保存并读回，未调用真实服务', 'ok'); } catch (error) { if (error.status === 409) { draft.conflict = true; paint(); } else el.querySelector('[data-config-feedback]').textContent = error.message; } };
    el.querySelector('[data-revert-config]').onclick = () => { draft.values = { ...saved.values }; draft.base = saved.version; draft.dirty = false; draft.conflict = false; paint(); };
    el.querySelector('[data-keep]')?.addEventListener('click', () => toast('草稿已保留'));
    el.querySelector('[data-refresh]')?.addEventListener('click', () => { draft.base = saved.version; draft.conflict = false; paint(); });
    el.querySelector('[data-config-test]').onclick = () => { const result = el.querySelector('[data-test-outcome]').value; el.querySelector('[data-test-result]').innerHTML = `<div class="banner ${result === '成功' ? 'ok' : result === '失败' ? 'err' : 'warn'}">Mock 测试示例：${esc(result)}。仅使用已保存的 v${saved.version}，未发送任何请求。</div>`; };
    el.querySelector('[data-discover-models]')?.addEventListener('click', () => { el.querySelector('[data-model-options]').innerHTML = '<p class="small muted">Mock 模型目录：Qwen · Cydonia · Embedding · Vision。允许在左侧手工填写模型 ID。</p>'; });
    el.querySelector('[data-workflow-file]')?.addEventListener('change', async event => { const file = event.target.files?.[0]; if (!file) return; try { const text = await file.text(); JSON.parse(text); draft.values.workflow = text; draft.dirty = true; paint(); toast('已读入 Mock 草稿，尚未保存'); } catch { el.querySelector('[data-config-feedback]').textContent = '文件不是有效 JSON，原草稿保留。'; } });
  }
  paint();
}

export function renderDiagnosticRecords(el, ctx) {
 if(!store.isMock)return providerPage(()=>import("./real-diagnostics.js"),"renderRealDiagnosticRecords",el,ctx);
  const sectionId = ctx.def.mockSection, query = ctx.query.q || '';
  let offset = 0, rows = [], nextOffset = 0, total = 0;
  function append() { const page = mockOperations.runs(sectionId, query, offset); rows.push(...page.items); nextOffset = page.nextOffset; total = page.total; }
  append();
  function paint() {
    el.innerHTML = `${SearchForm({ value: query, placeholder: '记录 ID / 标题 / 关联 ID', attrs: { 'data-record-search': true } })}<div class="row" style="margin:12px 0"><a class="btn" href="#/diag">返回诊断</a>${Button({ label: '刷新 Mock 记录', attrs: { 'data-refresh-records': true } })}${Button({ label: '导出当前筛选（Mock）', attrs: { 'data-export-records': true } })}<span class="small muted">已载入 ${rows.length} / ${total} · ${badge('Mock', 'plain')}</span></div>
      ${RecordList({ rows, fields: [{ label: '记录', key: 'title' }, { label: '模型 / 身份', key: 'role' }, { label: '状态', render: row => badge(row.status, 'plain') }, { label: '时间', key: 'at' }], action: row => Button({ label: '详情', size: 'sm', attrs: { 'data-open-run': row.id } }) })}<div class="row" style="margin-top:12px">${nextOffset !== null ? Button({ label: '加载更多', attrs: { 'data-more-records': true } }) : '<span class="small muted">当前筛选已全部载入。</span>'}</div>`;
    el.querySelector('[data-record-search]').onsubmit = event => { event.preventDefault(); ctx.navigate(`#/diag/${sectionId}?q=${encodeURIComponent(event.target.q.value)}`); };
    el.querySelector('[data-refresh-records]').onclick = () => { offset = 0; rows = []; append(); paint(); };
    el.querySelector('[data-more-records]')?.addEventListener('click', () => { offset = nextOffset; append(); paint(); });
    el.querySelectorAll('[data-open-run]').forEach(button => button.onclick = () => {
      const row = rows.find(item => item.id === button.dataset.openRun);
      const d = drawer({ title: `${row.title} · Mock`, body: `<div class="row">${badge(row.status, 'plain')}<span class="small muted">${esc(row.id)} · v${row.version}</span></div><p class="small muted">关联 ${esc(row.correlationId)} · ${esc(row.role)}</p>${JsonDetails(row.input, 'Mock 输入 / 来源')}${JsonDetails(row.output, 'Mock 输出 / 错误')}<p class="small muted">此处不是模型真实执行记录，也不会触发真实工作流。</p>`, footer: sectionId === 'workflows' ? ['暂停', '恢复', '取消', '重启', '重置工作流'].map(command => Button({ label: command, tone: command === '取消' ? 'danger' : '', size: 'sm', attrs: { 'data-run-command': command } })).join('') : row.status === '失败' ? Button({ label: '重试（Mock）', attrs: { 'data-run-command': '重试' } }) : '' });
      d.el.querySelectorAll('[data-run-command]').forEach(control => control.onclick = async () => { const command = control.dataset.runCommand; if (!(await confirm({ title: `${command}（Mock）`, identity: '系统运维', text: '仅改变这条 Mock 运行记录，不操作真实任务，也不是生活时间线回档。' }))) return; try { mockOperations.commandRun(sectionId, row.id, command, row.version); d.close(); offset = 0; rows = []; append(); paint(); toast(`${command}已记录（Mock）`, 'ok'); } catch (error) { toast(error.message, 'err'); } });
    });
    el.querySelector('[data-export-records]').onclick = () => { const payload = { source: 'Mock', complete: nextOffset === null, nextOffset, items: rows }, url = URL.createObjectURL(new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' })); const d = drawer({ title: '诊断导出（Mock）', onClose: () => URL.revokeObjectURL(url), body: `<a class="btn primary" download="mock-diagnostics.json" href="${url}">下载 Mock JSON</a><p class="small muted">仅显示当前筛选已载入记录。完整性：${nextOffset === null ? '完整' : '部分（仍有下一页）'}。</p>${JsonDetails({ source: 'Mock', complete: nextOffset === null, nextOffset, items: rows }, '展开导出内容')}` }); };
  }
  paint();
}
