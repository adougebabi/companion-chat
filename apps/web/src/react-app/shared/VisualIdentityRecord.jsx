import { Button } from '@/components/ui/button';
import { Disclosure } from '../controls';
import { visualSessionState } from '@/approved-ui/media-presentation.mjs';

const missing = '尚未记录';
const reviewLabels = {
  accepted: '已通过', accept: '已通过', rejected: '未通过',
  regenerate: '需要重新生成', awaiting_review: '待评审',
  generating: '准备中', failed: '准备失败', cancelled: '已取消',
};
function reviewState(value) {
  return value == null || value === '' ? missing : reviewLabels[value] || '评审状态待确认';
}
function revision(value) {
  return Number.isSafeInteger(value) && value >= 0 ? `第 ${value} 版` : missing;
}

export function DownloadVisualRecord({ value }) {
  return <Button type="button" variant="outline" disabled={!value} onClick={() => {
    const url = URL.createObjectURL(new Blob([JSON.stringify(value, null, 2)], { type: 'application/json' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = '视觉身份原始记录.json';
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }}>下载原始记录</Button>;
}

// Project only known business fields. Unknown extensions and technical identity
// remain available in the download, never in the reading surface.
export default function VisualIdentityRecord({ value }) {
  const constraints = value?.renderer_constraints;
  const session = value?.session;
  const current = session?.attempts?.find(row => row.attempt_number === session.current_attempt);
  const schema = typeof value?.schema_version === 'string'
    ? /^visual-identity\.v(\d+)$/.exec(value.schema_version)?.[1] : null;
  const cup = constraints?.chest_cup;
  const size = cup === 'not_applicable' ? '不适用'
    : typeof cup === 'string' && /^(AAA|AA|[A-K])$/.test(cup) ? `${cup} 罩杯` : missing;
  const fields = [
    ['规范版本', revision(value?.current_revision)],
    ['记录格式版本', schema ? `第 ${schema} 版` : missing],
    ['渲染约束', size === missing && !Number.isFinite(constraints?.chest_lora_weight) && typeof constraints?.chest_lora_applicable !== 'boolean' ? missing : '已记录身体比例约束'],
    ['胸部尺寸', size],
    ['比例调整权重', Number.isFinite(constraints?.chest_lora_weight) ? String(constraints.chest_lora_weight) : missing],
    ['比例调整是否适用', typeof constraints?.chest_lora_applicable === 'boolean' ? constraints.chest_lora_applicable ? '是' : '否' : missing],
    ['会话状态', session?.status ? visualSessionState(session.status) : missing],
    ['候选评审状态', reviewState(current?.decision || current?.status)],
    ['头像评审状态', reviewState(value?.generated_reviews?.avatar?.status)],
    ['角色设定图评审状态', reviewState(value?.generated_reviews?.character_sheet?.status)],
  ];
  return <Disclosure title="规范来源、渲染约束与完整会话">
    <dl className="grid min-w-0 gap-3" aria-label="视觉身份记录摘要">
      {fields.map(([label, text]) => <div key={label} className="grid min-w-0 gap-1 border-b border-border/60 pb-3">
        <dt className="text-sm font-medium">{label}</dt>
        <dd className="m-0 min-w-0 break-words text-sm text-muted-foreground">{text}</dd>
      </div>)}
    </dl>
    <p className="my-3 text-sm text-muted-foreground">完整规范来源与会话记录可下载保存。</p>
    <DownloadVisualRecord value={value}/>
  </Disclosure>;
}
