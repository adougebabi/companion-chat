export const configurationSections = [
  { id: 'models', title: '模型与连接', features: ['SET-01', 'SET-02', 'SET-03'], fields: [
    ['endpointId', 'Endpoint 标识', 'primary'], ['endpointUrl', '服务地址', 'http://localhost:1234/v1', 'url'], ['secret', '密钥', '', 'password'],
    ['genericModel', '认知 / 通用 LLM · Qwen', 'Qwen'], ['replyModel', '私聊回复 · Cydonia', 'Cydonia'], ['embeddingModel', 'Embedding 模型', 'Embedding'], ['visionModel', '视觉理解模型', 'Vision'],
    ['timeoutSeconds', '请求超时（秒）', '120', 'number'],
  ] },
  { id: 'reply-vision', title: '回复与视觉', features: ['SET-06'], fields: [['factOnly', 'Fact-only 回复', true, 'boolean'], ['replyTemperature', '回复温度', '0.7', 'number'], ['visionEnabled', '视觉理解开关', false, 'boolean'], ['visionTimeout', '视觉超时（秒）', '120', 'number']] },
  { id: 'media-workflows', title: '媒体与工作流', features: ['SET-04'], fields: [['comfyUrl', 'ComfyUI 地址', 'http://localhost:8188', 'url'], ['workflow', '工作流 JSON', '{\n  "nodes": [],\n  "nodes": []\n}', 'json'], ['generationTimeout', '生成等待上限（秒）', '600', 'number']] },
  { id: 'kev', title: 'KEV 选择与连接', features: ['SET-07'], fields: [['enabled', '启用 KEV', true, 'boolean'], ['endpoint', 'KEV 服务地址', 'http://localhost:11435', 'url'], ['secret', '密钥', '', 'password'], ['threshold', '选择阈值', '0.7', 'number'], ['timeout', '等待上限（秒）', '30', 'number'], ['decisionPoints', '适用决策点', '对话能力选择\n目标规划\n媒体拍摄方案', 'textarea']] },
  { id: 'runtime', title: '运行与自治策略', features: ['SET-05', 'GOV-03'], fields: [['autonomyMode', '自治模式', '启用', 'select', ['启用', '暂停主动联系', '暂停自治运行']], ['concurrency', '并发上限', '2', 'number'], ['dailyBudget', '每日预算', '20', 'number'], ['cooldown', '冷却时间（秒）', '60', 'number'], ['quietStart', '安静时段开始', '23:00', 'time'], ['quietEnd', '安静时段结束', '08:00', 'time'], ['allowedActions', '允许的行动类型', '日常活动\n记录经历\n分享动态', 'textarea']] },
  { id: 'notifications', title: '通知与触达偏好', features: ['BP-03.5'], fields: [['messages', '新消息通知', true, 'boolean'], ['goals', '约定与目标提醒', true, 'boolean'], ['media', '媒体结果通知', true, 'boolean'], ['browserPermission', '浏览器通知权限', '未申请', 'select', ['未申请', '允许', '拒绝']], ['frequency', '触达频率', '适中', 'select', ['很少', '适中', '较多']]] },
  { id: 'owner', title: '所有者与会话', features: ['SYS-01', 'SET-08'], fields: [['name', '账户显示名称', '我'], ['password', '新密码', '', 'password'], ['confirmPassword', '再次输入', '', 'password']] },
  { id: 'capabilities', title: '能力与权限目录', features: ['GOV-07', 'GAP-02'], fields: [['allowMemory', '允许记录记忆', true, 'boolean'], ['allowMedia', '允许媒体生成', true, 'boolean'], ['allowOutreach', '允许主动联系', true, 'boolean'], ['allowWorldActions', '允许日常世界行动', true, 'boolean']] },
];
export const diagnosticSections = [
 {id:'media-recoveries',title:'媒体恢复尝试'}, {id:'turns',title:'回合诊断'}, {id:'links',title:'执行器关联'},
  { id: 'lifecycle', title: '生命周期', feature: 'DIA-01' }, { id: 'model-runs', title: '模型运行', feature: 'DIA-02' },
  { id: 'agent-runs', title: 'Agent 运行', feature: 'DIA-02' }, { id: 'events', title: '系统事件', feature: 'DIA-02' },
  { id: 'media-prompts', title: '媒体请求', feature: 'DIA-03' }, { id: 'workflows', title: '工作流控制', feature: 'DIA-04' },
  { id: 'kev', title: 'KEV 决策', feature: 'DIA-05' },
];
