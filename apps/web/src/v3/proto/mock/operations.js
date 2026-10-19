import * as store from './store.js';
import { MockConflict } from './client.js';

export const configurationSections = [
  { id: 'models', title: '模型与连接', features: ['SET-01', 'SET-02', 'SET-03'], fields: [
    ['endpointId', 'Endpoint 标识', 'primary'], ['endpointUrl', '服务地址', 'http://localhost:1234/v1', 'url'], ['secret', '密钥（Mock 写入演示，不保存输入）', '', 'password'],
    ['genericModel', '认知 / 通用 LLM · Qwen', 'Qwen（Mock）'], ['replyModel', '私聊回复 · Cydonia', 'Cydonia（Mock）'], ['embeddingModel', 'Embedding 模型', 'Embedding（Mock）'], ['visionModel', '视觉理解模型', 'Vision（Mock）'],
    ['tokenBudget', '输出 Token 预算', '4096', 'number'], ['timeoutSeconds', '请求超时（秒）', '120', 'number'],
  ] },
  { id: 'reply-vision', title: '回复与视觉', features: ['SET-06'], fields: [['factOnly', 'Fact-only 回复', true, 'boolean'], ['replyTemperature', '回复温度', '0.7', 'number'], ['maxOutputTokens', '最大输出 Token', '4096', 'number'], ['visionEnabled', '视觉理解开关', false, 'boolean'], ['visionTimeout', '视觉超时（秒）', '120', 'number']] },
  { id: 'media-workflows', title: '媒体与工作流', features: ['SET-04'], fields: [['comfyUrl', 'ComfyUI 地址', 'http://localhost:8188', 'url'], ['workflow', '工作流 JSON', '{\n  "mock": true,\n  "nodes": []\n}', 'json'], ['generationTimeout', '生成等待上限（秒）', '600', 'number']] },
  { id: 'kev', title: 'KEV 选择与连接', features: ['SET-07'], fields: [['enabled', '启用 KEV（Mock）', true, 'boolean'], ['endpoint', 'KEV 服务地址', 'http://localhost:11435', 'url'], ['secret', '密钥（Mock 写入演示，不保存输入）', '', 'password'], ['threshold', '选择阈值', '0.7', 'number'], ['timeout', '等待上限（秒）', '30', 'number'], ['decisionPoints', '适用决策点', '对话能力选择\n目标规划\n媒体拍摄方案', 'textarea']] },
  { id: 'runtime', title: '运行与自治策略', features: ['SET-05', 'GOV-03'], fields: [['autonomyMode', '自治模式', '启用', 'select', ['启用', '暂停主动联系', '暂停自治运行']], ['concurrency', '并发上限', '2', 'number'], ['dailyBudget', '每日预算', '20', 'number'], ['cooldown', '冷却时间（秒）', '60', 'number'], ['quietStart', '安静时段开始', '23:00', 'time'], ['quietEnd', '安静时段结束', '08:00', 'time'], ['allowedActions', '允许的行动类型', '日常活动\n记录经历\n分享动态', 'textarea']] },
  { id: 'notifications', title: '通知与触达偏好', features: ['BP-03.5'], fields: [['messages', '新消息通知', true, 'boolean'], ['goals', '约定与目标提醒', true, 'boolean'], ['media', '媒体结果通知', true, 'boolean'], ['browserPermission', '浏览器通知权限（仅 Mock）', '未申请', 'select', ['未申请', '允许（Mock）', '拒绝（Mock）']], ['frequency', '触达频率', '适中', 'select', ['很少', '适中', '较多']]] },
  { id: 'owner', title: '所有者与会话', features: ['SYS-01', 'SET-08'], fields: [['name', '账户显示名称', '我'], ['password', '新密码（仅 Mock，不保存输入）', '', 'password'], ['confirmPassword', '再次输入', '', 'password']] },
  { id: 'capabilities', title: '能力与权限目录', features: ['GOV-07', 'GAP-02'], fields: [['allowMemory', '允许记录记忆', true, 'boolean'], ['allowMedia', '允许媒体生成', true, 'boolean'], ['allowOutreach', '允许主动联系', true, 'boolean'], ['allowWorldActions', '允许日常世界行动', true, 'boolean']] },
];
export const diagnosticSections = [
  { id: 'lifecycle', title: '生命周期', feature: 'DIA-01' }, { id: 'model-runs', title: '模型运行', feature: 'DIA-02' },
  { id: 'agent-runs', title: 'Agent 运行', feature: 'DIA-02' }, { id: 'events', title: '系统事件', feature: 'DIA-02' },
  { id: 'media-prompts', title: '媒体请求', feature: 'DIA-03' }, { id: 'workflows', title: '工作流控制', feature: 'DIA-04' },
  { id: 'kev', title: 'KEV 决策', feature: 'DIA-05' },
];
const clone = value => structuredClone(value);

export const mockOperations = {
  configuration(sectionId, characterId = '') {
    const section = configurationSections.find(item => item.id === sectionId); if (!section) throw new Error('未找到设置分区');
    const key = `${characterId || 'global'}:${sectionId}`, saved = store.get('meta').mockConfigurations?.[key];
    return clone({ key, section, version: saved?.version || 0, values: saved?.values || Object.fromEntries(section.fields.map(([name, , value]) => [name, value])) });
  },
  saveConfiguration(sectionId, characterId, values, expectedVersion) {
    const current = this.configuration(sectionId, characterId); if (current.version !== expectedVersion) throw new MockConflict(current.version);
    const next = clone(values);
    for (const [name, , , type] of current.section.fields) {
      if (type === 'json') { try { JSON.parse(next[name]); } catch { throw new Error('工作流 JSON 格式无效，草稿已保留'); } }
      if (type === 'number' && (!Number.isFinite(Number(next[name])) || Number(next[name]) < 0)) throw new Error('请填写有效的非负数值');
      if (type === 'password') next[name] = next[name] ? '已配置（Mock）' : current.values[name];
    }
    if (sectionId === 'owner' && values.password && (values.password.length < 6 || values.password !== values.confirmPassword)) throw new Error('两次密码应一致且至少 6 个字符；输入未保存');
    store.update(data => { (data.meta.mockConfigurations ||= {})[current.key] = { version: current.version + 1, values: next }; }, { identity: '系统运维', summary: `保存${current.section.title}（Mock，无真实运行效果）`, target: sectionId });
    return this.configuration(sectionId, characterId);
  },
  runs(sectionId, query = '', offset = 0) {
    if (!diagnosticSections.some(section => section.id === sectionId)) throw new Error('未找到诊断分区');
    const edits = store.get('meta').mockRunEdits || {};
    const rows = Array.from({ length: 24 }, (_, i) => {
      const id = `${sectionId}_mock_${i + 1}`, original = i % 5 === 0 ? '失败' : i % 4 === 0 ? '执行中' : '完成';
      return { id, title: sectionId === 'workflows' ? `生活工作流 ${i + 1}` : `运行记录 ${i + 1}`, status: edits[id]?.status || original, at: `10-15 ${String(10 - Math.floor(i / 6)).padStart(2, '0')}:${String(50 - i).padStart(2, '0')}`, correlationId: `mock-correlation-${Math.floor(i / 3) + 1}`, role: i % 2 ? 'Cydonia · 回复' : 'Qwen · 认知', input: { source: 'Mock 验收样本', taskId: 't88', step: i % 4 + 1 }, output: original === '失败' ? { error: 'provider_timeout（Mock 示例）', retryable: true } : { summary: 'Mock 执行结果，仅供页面验收' }, version: edits[id]?.version || 1 };
    }).filter(row => !query || `${row.id} ${row.title} ${row.correlationId}`.includes(query));
    return { items: rows.slice(offset, offset + 8), nextOffset: offset + 8 < rows.length ? offset + 8 : null, total: rows.length };
  },
  commandRun(sectionId, id, command, expectedVersion) {
    const row = this.runs(sectionId, id).items.find(item => item.id === id); if (!row) throw new Error('未找到运行记录'); if (row.version !== expectedVersion) throw new MockConflict(row.version);
    const next = { 暂停: '已暂停', 恢复: '执行中', 取消: '已取消', 重启: '执行中', 重试: '执行中', 重置工作流: '执行中' }[command]; if (!next) throw new Error('不支持的 Mock 操作');
    store.update(data => { (data.meta.mockRunEdits ||= {})[id] = { status: next, version: row.version + 1 }; }, { identity: '系统运维', summary: `${command} ${id}（Mock，无真实工作流调用）`, target: id });
  },
};
