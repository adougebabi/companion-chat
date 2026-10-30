import { semanticLabel } from '@/v3/proto/components/semantic-fields.js';
import { enumLabel } from '@/lib/fluctlight-display.ts';
import {relationMetrics} from '@/approved-ui/people-presentation.mjs';
import {wardrobeCategoryLabel, wardrobeSlotLabel, wardrobeOwnershipOptions, wardrobeAvailabilityOptions} from '@/lib/wardrobe-form.ts';

const labels = {
  eyes: '眼睛 / 瞳色', family: '家庭背景', hair: '发型', body_type: '体态',
  target_actor_id: '关系对象', role: '关系', metrics: '初始指标',
  summary: '关系摘要', emotional_association: '情感联想',
  familiarity: '熟悉程度', trust: '信任', affection: '好感',
  importance: '重要度', urgency: '紧迫度', scope: '范围', action: '意图',
  goal_index: '关联目标', profile_id: '人格视角', description: '说明',
};
export function creationPreviewLabel(key) {
  const label = Object.hasOwn(labels,key) ? labels[key] : semanticLabel(key);
  return typeof label !== 'string' || label === '自定义字段' ? '补充设定' : label;
}
const order = {
  appearance: ['description', 'hair', 'eyes', 'body_type'],
  initial_relationships: ['target_actor_id', 'role', 'metrics', 'summary', 'emotional_association'],
};

/** A reading projection only. Editors always receive the untouched source value. */
export default function CreationPreviewValue({value, field = ''}) {
  if (value == null || value === '') return <span className="muted">尚未记录</span>;
  if (Array.isArray(value)) return value.length ? <div className="creation-preview-records">{value.map((item, index) => <div key={index}><CreationPreviewValue value={item} field={field}/></div>)}</div> : <span className="muted">尚未记录</span>;
  if (typeof value === 'object') {
    const priority = Object.hasOwn(order,field) ? order[field] : [];
    const metrics = field === 'metrics' ? new Map(relationMetrics({metrics:value}).map(row => [row.key,row])) : new Map();
    const entries = Object.entries(value).sort(([a], [b]) => {
      const rank = key => priority.includes(key) ? priority.indexOf(key) : priority.length;
      return rank(a) - rank(b);
    });
    return entries.length ? <dl className="creation-preview-facts">{entries.map(([key, item]) => <div key={key}>
      <dt>{metrics.get(key)?.label || creationPreviewLabel(key)}</dt>
      <dd>{key === 'target_actor_id' && item === 'owner' ? '你' : metrics.has(key)
        ? new Intl.NumberFormat('zh-CN', {style:'percent', maximumFractionDigits:2}).format(item)
        : <CreationPreviewValue value={item} field={key}/>}</dd>
    </div>)}</dl> : <span className="muted">尚未记录</span>;
  }
  if (typeof value === 'boolean') return <span>{value ? '是' : '否'}</span>;
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
