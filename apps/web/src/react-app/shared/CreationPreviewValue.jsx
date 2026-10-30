import { semanticLabel } from '@/v3/proto/components/semantic-fields.js';
import { enumLabel } from '@/lib/fluctlight-display.ts';
import {relationMetrics, relationshipRoleLabel} from '@/approved-ui/people-presentation.mjs';
import {wardrobeCategoryLabel, wardrobeSlotLabel, wardrobeOwnershipOptions, wardrobeAvailabilityOptions} from '@/lib/wardrobe-form.ts';

const labels = {
  eyes: '眼睛 / 瞳色', family: '家庭背景', hair: '发型', body_type: '体态',
  target_actor_id: '关系对象', role: '关系', metrics: '初始指标',
  summary: '关系摘要', emotional_association: '情感联想',
  familiarity: '熟悉程度', trust: '信任', affection: '好感',
  importance: '重要度', urgency: '紧迫度', scope: '范围', action: '意图',
  goal_index: '关联目标', profile_id: '人格视角', description: '说明',
  style: '风格', preferred_style: '偏好风格', fit: '版型', colors: '偏好颜色',
  color_palette: '配色', materials: '偏好材质', clothing: '服装偏好',
  food: '饮食偏好', foods: '饮食偏好', food_preferences: '饮食偏好',
  taste: '口味', flavor: '口味', drink: '饮品偏好', drinks: '饮品偏好',
  music: '音乐偏好', music_genres: '音乐类型', genre: '类型', genres: '偏好类型',
  media_preferences: '媒体偏好',
  movies: '电影偏好', books: '阅读偏好', topics: '话题偏好',
  frequency: '频率', recurrence: '重复周期', cadence: '重复周期', schedule: '时间安排',
  time: '时间', day: '日期', days: '日期', weekday: '星期', duration: '持续时间',
  relationship: '关系说明', relationship_type: '关系类型', type: '类型',
  addressing: '相互称呼', preferred: '偏好称呼', self_reference: '自称',
  mode: '人格模式', profiles: '人格档案', active_profile_id: '当前人格',
  switching: '切换规则', rules: '规则', policy: '采用方式',
  influence: '人格间的影响', edges: '影响关系', conflict_resolution: '分歧处理',
  forced_activation: '特别激活条件', takeover_rules: '接管规则',
  condition: '触发条件', source_profile_id: '来源人格', target_profile_id: '目标人格',
  enabled: '是否启用', shared_memory: '共享记忆', extensions: '扩展资料',
  extra_detail: '其他细节', extra_note: '其他说明', extension_count: '扩展资料数量',
  custom_metric: '自定义指标', unknown_goal: '其他目标设定', custom_intention: '其他意图设定',
};
export function creationPreviewLabel(key, index = 0) {
  const label = Object.hasOwn(labels,key) ? labels[key] : semanticLabel(key);
  // Unknown extensions have no reliable translation. Keep them individually
  // addressable; their exact keys and values remain in the full-setting editor.
  return typeof label !== 'string' || label === '自定义字段' ? `其他设定 ${index + 1}` : label;
}
export const hasCreationPreviewValue = value => value != null && value !== '' &&
  (Array.isArray(value) ? value.some(hasCreationPreviewValue) :
    typeof value !== 'object' || Object.values(value).some(hasCreationPreviewValue));
const order = {
  appearance: ['description', 'hair', 'eyes', 'body_type'],
  initial_relationships: ['target_actor_id', 'role', 'metrics', 'summary', 'emotional_association'],
};

/** A reading projection only. Editors always receive the untouched source value. */
export default function CreationPreviewValue({value, field = '', parentField = '', compact = false}) {
  if (value == null || value === '') return <span className="muted">尚未记录</span>;
  if (Array.isArray(value)) {
    const items = compact ? value.filter(hasCreationPreviewValue) : value;
    return items.length ? <div className="creation-preview-records">{items.map((item, index) => <div key={index}><CreationPreviewValue value={item} field={field} parentField={parentField} compact={compact}/></div>)}</div> : <span className="muted">尚未记录</span>;
  }
  if (typeof value === 'object') {
    const priority = Object.hasOwn(order,field) ? order[field] : [];
    const metrics = field === 'metrics' ? new Map(relationMetrics({metrics:value}).map(row => [row.key,row])) : new Map();
    const entries = Object.entries(value).filter(([, item]) => !compact || hasCreationPreviewValue(item)).sort(([a], [b]) => {
      const rank = key => priority.includes(key) ? priority.indexOf(key) : priority.length;
      return rank(a) - rank(b);
    });
    return entries.length ? <dl className="creation-preview-facts">{entries.map(([key, item], index) => <div key={key}>
      <dt>{field === 'role' && key === 'label' ? '关系称呼' : metrics.get(key)?.label || creationPreviewLabel(key, index)}</dt>
      <dd>{key === 'target_actor_id' && ['owner', 'actor_user'].includes(item) ? '你' : metrics.has(key)
        ? new Intl.NumberFormat('zh-CN', {style:'percent', maximumFractionDigits:2}).format(item)
        : <CreationPreviewValue value={item} field={key} parentField={field} compact={compact}/>}</dd>
    </div>)}</dl> : <span className="muted">尚未记录</span>;
  }
  if (typeof value === 'boolean') return <span>{value ? '是' : '否'}</span>;
  if (['role', 'relationship_type'].includes(field) || field === 'label' && parentField === 'role' || field === 'type' && ['friend','family','colleague','partner','acquaintance','stranger'].includes(value)) return <span>{relationshipRoleLabel(value)}</span>;
  if (['active_profile_id','profile_id','source_profile_id','target_profile_id','id'].includes(field) && value === 'default') return <span>默认人格</span>;
  if (['frequency','recurrence','cadence'].includes(field)) {
    const frequencies = {daily:'每天',weekly:'每周',monthly:'每月',yearly:'每年',weekdays:'工作日',weekends:'周末',occasionally:'偶尔',rarely:'很少',often:'经常',never:'从不',always:'总是',default:'默认频率'};
    if (Object.hasOwn(frequencies,value)) return <span>{frequencies[value]}</span>;
  }
  if (field === 'category') {const categories={upper:'上装',lower:'下装',full:'连身服装'},label=Object.hasOwn(categories,value)?categories[value]:wardrobeCategoryLabel(value);return <span>{typeof label==='string'?label:String(value)}</span>;}
  if (['slot','clothing_type'].includes(field)) return <span>{wardrobeSlotLabel(value)}</span>;
  const choices = field === 'ownership' ? wardrobeOwnershipOptions : field === 'availability' ? wardrobeAvailabilityOptions : [];
  const choice = choices.find(item => item.value === value);
  if (choice) return <span>{choice.label}</span>;
  const aliases = {single:'单一人格',multiple:'多重人格',black:'黑色',brown:'棕色',blonde:'金色',long:'长发',shoulder_length:'齐肩',short_hair:'短发',spring:'春',summer:'夏',autumn:'秋',winter:'冬',gentle:'温和',direct:'直接',natural:'自然'};
  if (Object.hasOwn(aliases,value)) return <span>{aliases[value]}</span>;
  const translated = enumLabel(value);
  return <span>{typeof translated !== 'string' || translated === '未分类' ? String(value) : translated}</span>;
}
