type JsonRecord = Record<string, unknown>;

const labels: Record<string, string> = {
  id: "标识",
  name: "名称",
  age: "年龄",
  gender: "性别",
  education: "教育经历",
  nationality: "国籍",
  occupation: "职业",
  residence: "居住地",
  timezone: "时区",
  birthday: "生日",
  background: "背景",
  biography: "经历",
  fashion_preference: "穿衣偏好",
  physical_attributes: "外貌特征",
  appearance: "外貌",
  visual_baseline: "外观基线",
  core_values: "核心价值观",
  worldview: "世界观",
  social_background: "社会背景",
  preferences: "偏好",
  life_habits: "生活习惯",
  recurring_commitments: "固定承诺",
  relationship_seeds: "关系种子",
  character_constraints: "性格边界",
  communication_boundaries: "沟通边界",
  desired_distance: "期望距离",
  user_identity: "用户身份",
  default_scene_ref: "默认场景",
  outfit: "穿着",
  notes: "备注",
  openness: "开放性",
  conscientiousness: "尽责性",
  extraversion: "外向性",
  agreeableness: "宜人性",
  neuroticism: "情绪敏感度",
  curiosity: "好奇心",
  independence: "独立性",
  patience: "耐心",
  empathy: "共情",
  assertiveness: "主张性",
  humor: "幽默感",
  sociability: "社交性",
  risk_tolerance: "风险偏好",
  update_policy: "更新策略",
  response_style: "回复风格",
  message_length: "消息长度",
  emoji_frequency: "表情频率",
  punctuation_style: "标点风格",
  humor_style: "幽默风格",
  sarcasm_tendency: "讽刺倾向",
  directness: "直接性",
  initiative: "主动性",
  topic_initiation: "发起话题",
  silence_tolerance: "沉默容忍度",
  response_delay: "回复延迟",
  emotional_expression: "情绪表达",
  conflict_style: "冲突风格",
  refusal_style: "拒绝风格",
  intimacy_expression: "亲密表达",
  source: "来源",
  status: "状态",
  trend: "趋势",
  action_type: "动作类型",
  attention: "注意",
  thought: "想法",
  desire: "愿望",
  agency: "行动判断",
  internal_dynamics: "内在变化",
  wake_up: "唤醒",
  event_kind: "事件类型",
  reason: "原因",
  revision: "修订版本",
  progress: "进度",
  confidence: "置信度",
  importance: "重要性",
  urgency: "紧迫度",
  scene: "场景",
  activity: "活动",
  location: "地点",
  current_task: "当前任务",
  user_presence: "用户状态",
  local_date: "本地日期",
  reschedule_policy: "调整策略",
  candidate_type: "候选类型",
  source_window: "来源窗口",
  key: "键名",
  kind: "类型",
  label: "名称",
  description: "说明",
  value: "当前值",
  value_schema: "取值规则",
  value_type: "值类型",
  schema: "规则",
  scalar: "单值",
  categorical: "分类值",
  bounded_object: "受限对象",
  slot_id: "槽位标识",
  direction: "方向",
  salience: "显著性",
  pressure: "压力",
  provenance: "来源记录",
  decay_policy: "衰减策略",
  evidence_refs: "证据引用",
  requested_delta: "请求变化量",
  applied_delta: "实际变化量",
  aggregate_count: "关联实例数",
  capability_key: "能力标识",
  title: "标题",
  rationale: "提出原因",
  desired_contract: "期望契约",
  side_effect_class: "副作用级别",
  priority: "优先级",
  request_id: "需求标识",
  source_fact_id: "来源事实",
  source_fluctlight_id: "来源实例",
  plugin_version: "插件版本",
  error_code: "错误代码",
  result: "结果",
  response_intent: "回复意图",
  appraisal: "评估",
  dynamics: "动力学",
  focus: "注意焦点",
  drive: "人格动力",
  preference: "偏好",
  trigger: "触发偏好",
  trigger_schema: "触发规则",
  drive_direction: "动力方向",
  arousal: "唤醒度",
  pleasure: "愉悦度",
  dominance: "掌控感",
  stability: "稳定性",
  controllability: "可控性",
  relevance: "相关性",
  goal_congruence: "目标一致性",
  social_threat: "社交威胁",
  relationship_significance: "关系重要性",
  reward: "奖励",
  loss: "损失",
  expected_effect: "预期影响",
  intensity: "强度",
  type: "类型",
  minimum: "最小值",
  maximum: "最大值",
  minimum_value: "最小值",
  maximum_value: "最大值",
  enum: "可选值",
  items: "子项",
  properties: "属性",
  required: "必填项",
  additional_properties: "额外属性",
  superseded_by: "替代槽位",
  before: "变更前",
  after: "变更后",
  changes: "变更内容",
  created_at: "创建时间",
  updated_at: "更新时间",
  start_at: "开始时间",
  end_at: "结束时间",
  completed_at: "完成时间",
  frozen_at: "冻结时间",
  accepted_at: "接受时间",
  occurred_at: "发生时间",
  height: "身高",
  build: "体型",
  body_type: "体型",
  hair: "发型",
  hair_color: "发色",
  eye_color: "瞳色",
  skin_tone: "肤色",
  distinctive_features: "显著特征",
  appearance_description: "外貌描述",
  style: "风格",
  colors: "色彩",
  materials: "材质",
  wardrobe: "衣橱",
  accessories: "配饰",
  makeup: "妆容",
  school: "学校",
  major: "专业",
  degree: "学历",
  institution: "机构",
  language: "语言",
  source_message_id: "来源消息",
  source_event_id: "来源事件",
  slot_key: "槽位键",
  slot_label: "槽位名称",
  slot_description: "槽位说明",
  slot_value: "槽位值",
  slot_status: "槽位状态",
  trigger_type: "触发类型",
  review_status: "审核状态",
  desired_input: "期望输入",
  desired_output: "期望输出",
};

const keyPartLabels: Record<string, string> = {
  education: "教育",
  experience: "经历",
  fashion: "穿衣",
  preference: "偏好",
  physical: "外貌",
  attribute: "特征",
  social: "社会",
  background: "背景",
  life: "生活",
  habit: "习惯",
  recurring: "固定",
  commitment: "承诺",
  relationship: "关系",
  seed: "种子",
  character: "性格",
  constraint: "边界",
  communication: "沟通",
  boundary: "边界",
  desired: "期望",
  distance: "距离",
  user: "用户",
  identity: "身份",
  default: "默认",
  scene: "场景",
  visual: "外观",
  baseline: "基线",
  value: "值",
  schema: "规则",
  slot: "槽位",
  trigger: "触发",
  source: "来源",
  fact: "事实",
  fluctlight: "摇光",
  action: "动作",
  response: "回复",
  intent: "意图",
  error: "错误",
  code: "代码",
  drive: "动力",
  drives: "动力",
  preferences: "偏好",
  minimum: "最小",
  maximum: "最大",
  enum: "可选",
  properties: "属性",
  required: "必填",
  created: "创建",
  updated: "更新",
  started: "开始",
  ended: "结束",
  completed: "完成",
  height: "高度",
  body: "身体",
  hair: "头发",
  eye: "眼睛",
  skin: "皮肤",
  tone: "色调",
  distinctive: "显著",
  feature: "特征",
  color: "颜色",
  material: "材质",
  wardrobe: "衣橱",
  accessory: "配饰",
  makeup: "妆容",
  school: "学校",
  major: "专业",
  degree: "学历",
  institution: "机构",
  language: "语言",
  key: "键",
  label: "名称",
  description: "说明",
  status: "状态",
  input: "输入",
  output: "输出",
};

const enumLabels: Record<string, string> = {
  active: "运行中",
  paused: "已暂停",
  retired: "已归档",
  candidate: "候选",
  pending: "待处理",
  completed: "已完成",
  abandoned: "已放弃",
  cancelled: "已取消",
  expired: "已过期",
  confirmed: "已确认",
  inferred: "推断",
  visible: "可见",
  frozen: "已冻结",
  deferred: "已延后",
  proposed: "待审核",
  accepted: "已接受",
  rejected: "已拒绝",
  initialization: "初始化",
  human: "人工修订",
  lived_fact: "生活事实",
  reflection: "反思",
  rollback: "回滚",
  improving: "改善",
  declining: "下滑",
  stable: "稳定",
  new: "新建立",
  short: "简短",
  direct: "直接",
  clear: "清晰",
  schedule: "日程",
  event: "事件",
  hypothesis: "推断",
  unknown: "未知",
  episodic: "情境记忆",
  semantic: "语义记忆",
  procedural: "程序记忆",
  relationship: "关系记忆",
  preference: "偏好记忆",
  fact: "事实记忆",
  proactive_message: "主动消息",
  no_op: "不行动",
  memory_candidate: "记忆候选",
  relationship_candidate: "关系候选",
  schedule_proposal: "日程提议",
  media_request: "媒体请求",
  moment: "动态",
  scalar: "单值",
  categorical: "分类值",
  set: "集合",
  bounded_object: "受限对象",
};

export function labelFor(key: string): string {
  if (!key || /^[\u4e00-\u9fff]/u.test(key)) return key || "其他信息";
  const normalized = key.replace(/([a-z\d])([A-Z])/g, "$1_$2").replace(/[\s-]+/g, "_").toLowerCase();
  if (labels[normalized]) return labels[normalized];
  const translatedParts = normalized.split("_").map((part) => keyPartLabels[part]);
  if (translatedParts.length > 1 && translatedParts.every(Boolean)) return translatedParts.join("");
  return "自定义字段";
}

export function enumLabel(value: unknown): string {
  if (typeof value !== "string") return formatDisplayValue(value);
  return enumLabels[value] ?? "未分类";
}

export function formatDisplayValue(value: unknown): string {
  if (value === null || value === undefined || value === "") return "未设定";
  if (typeof value === "boolean") return value ? "是" : "否";
  if (typeof value === "string" || typeof value === "number" || typeof value === "bigint") return String(value);
  if (Array.isArray(value)) {
    if (!value.length) return "未设定";
    return value.map((item) => formatDisplayValue(item)).join("、");
  }
  if (typeof value === "object") {
    const entries = Object.entries(value as JsonRecord);
    if (!entries.length) return "未设定";
    return entries.map(([key, item]) => {
      const normalizedKey = key.replace(/([a-z\d])([A-Z])/g, "$1_$2").replace(/[\s-]+/g, "_").toLowerCase();
      const enumText = typeof item === "string" && ["status", "trend", "type", "kind", "action_type", "source", "candidate_type"].includes(normalizedKey)
        ? enumLabels[item]
        : undefined;
      return `${labelFor(key)}：${enumText ?? formatDisplayValue(item)}`;
    }).join("；");
  }
  return "未设定";
}

function validTimezone(timezone: string | undefined): string | null {
  if (!timezone) return null;
  try {
    new Intl.DateTimeFormat("zh-CN", { timeZone: timezone }).format();
    return timezone;
  } catch {
    return null;
  }
}

export function resolveTimezone(...candidates: Array<string | undefined>): string {
  for (const candidate of candidates) {
    const valid = validTimezone(candidate);
    if (valid) return valid;
  }
  return "Asia/Shanghai";
}

function dateParts(value: string, timezone: string): { day: string; text: string } | null {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  const formatter = new Intl.DateTimeFormat("zh-CN", {
    timeZone: timezone,
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  });
  const parts = Object.fromEntries(formatter.formatToParts(date).map((part) => [part.type, part.value]));
  return { day: `${parts.month}-${parts.day}`, text: `${parts.hour}:${parts.minute}` };
}

export function formatZonedRange(start: unknown, end: unknown, ...timezones: Array<string | undefined>): string {
  if (typeof start !== "string" || typeof end !== "string") return "时间未设定";
  const timezone = resolveTimezone(...timezones);
  const startParts = dateParts(start, timezone);
  const endParts = dateParts(end, timezone);
  if (!startParts || !endParts) return "时间未设定";
  const prefix = startParts.day === endParts.day ? "" : `${startParts.day} `;
  const endPrefix = startParts.day === endParts.day ? "" : `${endParts.day} `;
  return `${prefix}${startParts.text} – ${endPrefix}${endParts.text}`;
}

export function formatZonedTime(value: unknown, ...timezones: Array<string | undefined>): string {
  if (typeof value !== "string") return "时间未设定";
  const parts = dateParts(value, resolveTimezone(...timezones));
  return parts ? parts.text : "时间未设定";
}
