import {isInternalId} from '../v3/internal-id.mjs';
import {userInstant} from '../v3/presentation-time.mjs';
import {memoryRevisionReason} from './memory-presentation.mjs';

export const memoryTypes = {episodic:'经历', semantic:'知识', relationship:'关系', autobiographical:'自我'};
export const memoryStatuses = {active:'有效记忆', forgotten:'已遗忘', superseded:'已替代', deprecated:'已废弃'};
export const provenanceLabels = {valid:'有效', invalid:'来源失效', pending:'待核对', legacy_unknown:'旧记录，来源未知'};
const visibilityLabels = {private:'仅角色可见', owner:'所有者可见', participants:'参与者可见'};
const sourceKinds = {message:'原消息', owner:'所有者说明', owner_statement:'所有者说明', event:'事件', reflection:'反思', fact:'经历事实', conversation_summary:'对话摘要', authenticated_command:'记忆管理操作', owner_confirmation:'所有者确认', memory:'相关记忆', outcome:'行动结果'};
const revisionOperations = {create:'创建记录', revise:'纠正正文', forget:'遗忘记录', rollback:'历史补偿恢复'};

export const labelFor = (labels, key) => Object.hasOwn(labels, key) ? labels[key] : '';
export const recordTime = value => typeof value === 'string' && Number.isFinite(Date.parse(value)) ? userInstant(value) : '';
export const unitScore = value => typeof value === 'number' && value >= 0 && value <= 1 ? value : null;
export const scoreText = value => unitScore(value) === null ? '' : Math.round(value * 100) + '%';

export function recordText(value) {
  if (typeof value !== 'string' || !value.trim()) return '';
  const text = value.trim();
  const version = /^(?:[\w.-]+\.)?v\d+(?:\.\d+)*$/i;
  const reference = /^(?:(?:conversation|message|ctx|proposal|reflection_proposal|evolution_candidate|goal|intent|outcome|profile|task|event)_[\w-]+|(?:message|memory|outcome|sequence|fact|owner):[^\s]+)$/i;
  if (isInternalId(text) || version.test(text) || reference.test(text)) return '';
  return /^\d{4}-\d{2}-\d{2}T\d/i.test(text) ? recordTime(text) : text;
}

export function memoryFacts(row) {
  return [
    ['状态', labelFor(memoryStatuses, row.status)],
    ['来源状态', labelFor(provenanceLabels, row.provenance_status)],
    ['可见范围', labelFor(visibilityLabels, row.visibility)],
    ['记于', recordTime(row.created_at)],
    ['发生时间', recordTime(row.occurred_at)],
    ['最近确认', recordTime(row.last_confirmed_at)],
    ['置信度', scoreText(row.confidence)],
    ['情感意义', scoreText(row.emotional_significance)],
    ['人格理解', (Array.isArray(row.personality_perspectives) ? row.personality_perspectives : [])
      .map(perspective => {
        const interpretation = recordText(perspective?.interpretation);
        const emotion = recordText(perspective?.emotion);
        return [interpretation, emotion && '感受：' + emotion].filter(Boolean).join(' · ');
      }).filter(Boolean).join('；')],
    ['与其他记忆的关系', [row.supersedes_memory_id && '替代了较早的记忆', row.superseded_by_memory_id && '已被另一条记忆替代'].filter(Boolean).join('，')],
  ];
}

export function memorySourceFacts(row) {
  return [
    ['来源', labelFor(sourceKinds, row.source_kind) || '来源记录'],
    ['来源时间', recordTime(row.occurred_at) || recordTime(row.recorded_at)],
    ['有效性', row.live === true ? '来源有效' : row.live === false ? '来源失效或未知' : labelFor(provenanceLabels, row.status)],
    ['来源变更', row.superseded_by_ref ? '已被其他来源替代' : ''],
  ];
}

export function memoryRevisionFacts(row) {
  return [
    ['修订方式', labelFor(revisionOperations, row.operation)],
    ['修订时间', recordTime(row.created_at)],
    ['修订说明', recordText(memoryRevisionReason(row))],
    ['状态', labelFor(memoryStatuses, row.status)],
  ];
}
