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
  core_values: "核心价值观",
  worldview: "世界观",
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
  memory_candidate: "记忆候选",
  relationship_candidate: "关系候选",
  schedule_proposal: "日程提议",
  media_request: "媒体请求",
  moment: "动态",
};

export function labelFor(key: string): string {
  if (labels[key]) return labels[key];
  if (!key || /^[\u4e00-\u9fff]/u.test(key)) return key || "其他信息";
  return "其他信息";
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
    return entries.map(([key, item]) => `${labelFor(key)}：${formatDisplayValue(item)}`).join("；");
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
