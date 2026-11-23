import { selectReadingFields } from '../../../approved-ui/reading-fields.mjs';
import { esc } from './controls.js';
import { semanticLabel, SemanticValue } from './semantic-fields.js';

const labels = {
  media_preferences: '媒体偏好', relationship_seeds: '关系起点', social_background: '社交背景',
  summary: '摘要', rule: '规则', personality_system: '人格系统',
  family: '家庭', upbringing: '成长经历', work: '工作经历', role: '关系角色',
  posts: '动态', moments: '动态', moment: '动态', selfies: '自拍', selfie: '自拍',
  drawings: '画作', drawing: '画作', painting: '画作', paintings: '画作', artwork: '画作',
  frequency: '频率', triggers: '触发条件', trigger: '触发条件',
  input: '输入', inputs: '输入', output: '输出', outputs: '输出',
  limits: '限制', limitations: '限制', constraints: '约束', restrictions: '限制',
  preconditions: '前置条件', prerequisites: '前置条件', examples: '示例',
  example: '示例', error_handling: '错误处理', errors: '错误处理',
  read_only: '只读', parameters: '参数', failure_modes: '可能的失败情况',
  use_cases: '使用场景', requirements: '要求', assumptions: '适用假设', output_format: '输出格式',
  success_criteria: '完成条件', temperament: '气质', sensitivities: '敏感点', rituals: '日常仪式',
  date_range: '日期范围', busy_intervals: '忙碌时段',
  metrics: '关系评估', warmth: '温暖程度', affinity: '好感程度', cooperation: '合作程度',
  secondary: '其他关系称谓', addressing: '相处称呼', preferred: '偏好称呼', self_reference: '自称',
  profiles: "人格档案", emotional_association: "情感联想", cooldown_seconds: "冷却时间（秒）",
  occupation: '职业', background_detail: '详细背景', display_name: '显示名称',
  conditions: '条件', condition: '条件', effect: '影响', effects: '影响',
  stable_motivations: '长期动机', switching: '人格切换', forced_activation: '强制切换条件',
  takeover_rules: '接管条件', influence: '人格间影响', conflict_resolution: '分歧处理',
  tags: '标签', color: '颜色', material: '材质', texture: '质感',
  behavior: '行为表现', rules: '规则', limitations_text: '限制说明',
  observations: '评审观察', missing_sections: '缺失部分',
  review_note: '评审意见', feedback: '反馈', derived_review_feedback: '评审反馈',
  expected_outcome: '预期结果', quantity: '数量',
  momentum: "行动势头", drives: "动机", conflicts: "内在冲突", regulation: "情绪调节",
  desired_hair_color: "期望发色", desired_hair_length: "期望发长", duration_minutes: "持续时间（分钟）",
  position: "位置", placement: "摆放位置", details: "细节",
  eyes: '眼睛', boundaries: '边界',
  body_part: "身体部位", severity: "严重程度", impact: "影响",
  motivation: '动机',
};
const parts = {
  daily: '每日', weekly: '每周', monthly: '每月', max: '最多', min: '最少',
  post: '动态', moment: '动态', selfie: '自拍', drawing: '画作', painting: '画作', art: '画作',
  frequency: '频率', trigger: '触发条件', count: '次数', length: '长度',
  name: '名称', type: '类型', detail: '详情', description: '说明', style: '风格',
  response: '响应', request: '请求', example: '示例', error: '错误', message: '消息',
  school: '学校', family: '家庭', work: '工作', hours: '小时', days: '天',
  history: "经历", craft: "技艺",
  required: '必填', optional: '可选', format: '格式', timeout: '超时', seconds: '秒',
};

const valueLabels = {
  input: { date_range: '日期范围' },
  output: { busy_intervals: '忙碌时段' },
  body_part: { hand: '手部', arm: '手臂', leg: '腿部', foot: '足部', head: '头部', ankle: '脚踝', torso: '躯干' },
  severity: { minor: '轻微', moderate: '中等', significant: '明显' },
  impact: { pain: '疼痛', mobility_limited: '行动受限', none: '无明显影响' },
};

export function readingValueLabel(value, field) {
  const choices = Object.hasOwn(valueLabels, field) ? valueLabels[field] : {};
  return typeof value === 'string' && Object.hasOwn(choices, value) ? choices[value] : value;
}

export function readingFieldLabel(key) {
  if (Object.hasOwn(labels, key)) return labels[key];
  const label = semanticLabel(key);
  if (label !== '自定义字段' && typeof label === 'string') return label;
  const words = key.replace(/([a-z\d])([A-Z])/g, '$1_$2').toLowerCase().split(/[_ -]+/);
  const translated = words.map(word => (Object.hasOwn(parts, word) ? parts[word] : ''));
  return translated.every(Boolean) ? translated.join('') : '补充说明';
}

function readingMarkup(value, key) {
  if (Array.isArray(value)) {
    return `<ul class="semantic-list">${value.map(item => `<li>${readingMarkup(item, key)}</li>`).join('')}</ul>`;
  }
  if (value && typeof value === 'object') {
    return `<dl class="record-facts">${Object.entries(value).map(([field, child]) =>
      `<div><dt>${esc(readingFieldLabel(field))}</dt><dd>${readingMarkup(child, field)}</dd></div>`).join('')}</dl>`;
  }
  return SemanticValue(readingValueLabel(value, key), key);
}

export function ReadingFields(value, shape) {
  const selected = selectReadingFields(value, shape);
  return selected === undefined ? '' : readingMarkup(selected);
}
