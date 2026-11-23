import { ProfileFieldsSummary, currentStateShape } from "./ProfileRecordSummary";
import { readingFields } from "@/approved-ui/reading-fields.mjs";
import { Disclosure } from '../controls';
import { visualSessionState } from '@/approved-ui/media-presentation.mjs';
import JsonDetails from './JsonDetails';

const visualSnapshotShape = { ...readingFields("description physical_features hair eyes body_type"), appearance: currentStateShape.appearance };
const visualReviewShape = readingFields("feedback review_note derived_review_feedback reason observations missing_sections");

export function VisualReviewReading({ value, title }) {
  return <ProfileFieldsSummary value={value} shape={visualReviewShape} title={title} />;
}

const reviewLabels = {
  accepted: '已通过', accept: '已通过', rejected: '未通过',
  regenerate: '需要重新生成', awaiting_review: '待评审',
  generating: '准备中', failed: '准备失败', cancelled: '已取消',
};
function reviewState(value) {
  if (!value) return null;
  return Object.hasOwn(reviewLabels, value) ? reviewLabels[value] : '评审状态待确认';
}

// Project only known business fields. Unknown extensions and technical identity
// remain available in the download, never in the reading surface.
export default function VisualIdentityRecord({ value }) {
  if (!value || !Object.keys(value).length) return null;
  const constraints = value?.renderer_constraints;
  const session = value?.session;
  const current = session?.attempts?.find(row => row.attempt_number === session.current_attempt);
  const cup = constraints?.chest_cup;
  const size = cup === 'not_applicable' ? '不适用'
    : typeof cup === 'string' && /^(AAA|AA|[A-K])$/.test(cup) ? `${cup} 罩杯` : null;
  const weight = Number.isFinite(constraints?.chest_lora_weight) ? String(constraints.chest_lora_weight) : null;
  const applicable = typeof constraints?.chest_lora_applicable === 'boolean' ? constraints.chest_lora_applicable ? '是' : '否' : null;
  const fields = [
    ['渲染约束', (size || weight || applicable) && '已记录身体比例约束'],
    ['胸部尺寸', size],
    ['比例调整权重', weight],
    ['比例调整是否适用', applicable],
    ['会话状态', session?.status ? visualSessionState(session.status) : null],
    ['候选评审状态', reviewState(current?.decision || current?.status)],
    ['头像评审状态', reviewState(value?.generated_reviews?.avatar?.status)],
    ['角色设定图评审状态', reviewState(value?.generated_reviews?.character_sheet?.status)],
  ].filter(([, text]) => text);
  return <>
    {fields.length > 0 && (
      <Disclosure title="渲染约束与视觉会话">
        <dl className="grid min-w-0 gap-3" aria-label="视觉身份记录摘要">
          {fields.map(([label, text]) => (
            <div key={label} className="grid min-w-0 gap-1 border-b border-border/60 pb-3">
              <dt className="text-sm font-medium">{label}</dt>
              <dd className="m-0 min-w-0 break-words text-sm text-muted-foreground">{text}</dd>
            </div>
          ))}
        </dl>
      </Disclosure>
    )}
    <ProfileFieldsSummary value={value.identity_snapshot?.life_profile ?? value.identity_snapshot} shape={visualSnapshotShape} title="视觉身份外貌" identity={value.identity_snapshot?.identity} />
    <VisualReviewReading value={current?.review ?? current} title="候选评审意见" />
    <VisualReviewReading value={value.generated_reviews?.avatar} title="头像评审意见" />
    <VisualReviewReading value={value.generated_reviews?.character_sheet} title="角色设定图评审意见" />
    <JsonDetails title="原始媒体记录（诊断）" value={value} downloadOnly filename="视觉身份原始记录.json" />
  </>;
}
