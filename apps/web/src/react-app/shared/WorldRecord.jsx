import { Button } from '@/components/ui/button';
import { Disclosure } from '../controls';
import { creationPreviewLabel } from './CreationPreviewValue';
import { enumLabel } from '@/lib/fluctlight-display.ts';

const labels = {
  name: '世界名称', background: '世界背景', description: '世界说明',
  flavor: '世界氛围', rules: '基础规则', active: '是否启用', timezone: '世界时区',
  item_pool: '物品目录', venue_categories: '地点分类', status: '状态',
  category: '分类', items: '物品', quantity: '数量', unit: '单位',
};
// Only the reading projection hides technical identity. Downloads and edit
// commands retain the original world, including unknown extensions.
const technicalKeys = new Set(['id', 'fluctlight_id', 'world_id', 'place_id', 'npc_id',
  'actor_id', 'owner_actor_id', 'source_resource_id', 'source_version', 'revision',
  'venue_category_id', 'source_ref', 'schema_version']);
function WorldValue({ value, field = '' }) {
  if (value == null || value === '') return <span className="muted">尚未记录</span>;
  if (Array.isArray(value)) return value.length
    ? <div className="grid min-w-0 gap-3">{value.map((item, index) => <WorldValue value={item} field={field} key={index}/>)}</div>
    : <span className="muted">尚未记录</span>;
  if (typeof value === 'object') return <dl className="record-facts grid min-w-0 gap-3">{Object.entries(value)
    .filter(([key]) => !technicalKeys.has(key))
    .map(([key, item], index) => <div className="grid min-w-0 gap-1 border-b border-border/60 pb-3 last:border-0" key={key}>
      <dt className="break-words text-sm font-medium">{Object.hasOwn(labels, key) ? labels[key] : creationPreviewLabel(key, index)}</dt>
      <dd className="m-0 min-w-0 whitespace-pre-wrap break-words text-sm leading-7"><WorldValue value={item} field={key}/></dd>
    </div>)}</dl>;
  if (typeof value === 'boolean') return <span>{value ? '是' : '否'}</span>;
  if (field === 'timezone' && value === 'Asia/Shanghai') return <span>北京时间</span>;
  const label = field === 'status' ? enumLabel(value) : value;
  return <span>{label === '未分类' ? String(value) : String(label)}</span>;
}
export default function WorldRecord({ value }) {
  return <Disclosure title="当前世界完整记录">
    <WorldValue value={value}/>
    <Button variant="outline" size="sm" asChild><a
      href={'data:application/json;charset=utf-8,' + encodeURIComponent(JSON.stringify(value, null, 2))}
      download="world-source.json">下载原始记录</a></Button>
  </Disclosure>;
}
