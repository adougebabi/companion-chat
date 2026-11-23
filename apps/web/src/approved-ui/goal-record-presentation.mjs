import {domainName, stateLabel} from '../v3/domain-presentation.mjs';
import {userInstant} from '../v3/presentation-time.mjs';

const operationLabels = {
  create: '创建目标', update: '修改目标', pause: '暂停目标', resume: '继续目标',
  abandon: '放下目标', cancel: '取消目标', reassess: '重新评估目标',
  candidate: '保留为候选目标', link_goal: '关联已有目标', dismiss: '不采纳为目标',
};
const decisionLabels = {
  applied: '已采纳规划并更新目标', no_change: '保留当前目标',
  no_viable_candidate: '本次没有可采纳的候选目标', awaiting_facts: '等待新信息后再规划',
  blocked_by_policy: '当前策略下未采纳规划', failed: '本次规划未完成',
  suggestions: '已提出候选，等待采纳',
};
const planningModes = {apply: '可采纳并更新目标', suggestions_only: '只提出建议'};
const evidenceKinds = {
  message: '对话', outcome: '实际行动结果', item: '物品', activity: '活动',
  actor_fact: '人物事实', goal_revision: '目标变更',
};
const statusLabels = {
  awaiting_next_step: '等待确定下一步', needs_qualification: '等待核对推进条件',
  ready: '可以推进', due: '已到推进时间', executing: '正在推进', waiting: '等待条件满足',
  awaiting_profile: '等待相关人格启用', requires_semantic_review: '待核对来源含义',
  candidate_saved: '已保留为候选', linked_existing: '已关联已有目标',
  dismissed_non_action_source: '未采纳为目标', confirmed: '已确认', withdrawn: '已撤回',
};
const reasonLabels = {
  'goal created': '目标已创建',
  'no applicable evidence': '暂无适用证据',
  'actual outcome explicitly bound to Goal': '实际结果已关联到此目标',
  'current cognition proposed related actual conversation': '相关对话被提议用作实际证据',
  'Automatic planning is disabled': '自动规划已关闭',
  'Restore authorized planning or wait for current policy conditions': '可开启自动规划，或等待当前策略条件满足',
  'Pending events belong to another private profile; retained until it becomes active': '相关信息已保留，等待对应人格启用',
  'Recovered durable goal set commit': '已找回保存的目标变更',
  'Recovered the durable Tool commit after final response failure': '已确认此前保存的目标变更',
  'No new opportunity': '当前没有新的机会',
  'No current opportunity': '当前没有可行的机会',
  'Await new facts': '等待新信息后再复核',
};
const label = (labels, value) => Object.hasOwn(labels, value) ? labels[value] : '';
const time = value => typeof value === 'string' && Number.isFinite(Date.parse(value)) ? userInstant(value) : '';

export function goalRecordText(value) {
  if (typeof value !== 'string' || !value.trim()) return '';
  const known = label(reasonLabels, value);
  if (known) return known;
  if (/\b(?:adk_run|NodeRunError|provider request failed|GPU memory|Prompt\s*\(\d+)\b/i.test(value)
    || /^\s*[\[{]/.test(value) || /^\s*\w+_[\w_]+\s*$/.test(value)) return '';
  return value
    .replace(/\b(?:source:\d+|(?:goal|stage|commitment|actor|profile):ctx_[\w-]+)\b/gi, '（详见诊断记录）')
    .replace(/\b(?:res|world_item|place|req|fluctlight|actor|goal(?:_planner)?|stage|commitment|source|task|profile)_[\w-]+\b/gi, '（详见诊断记录）')
    .replace(/\b[a-z][a-z\d+.-]*:\/\/[^\s"'，。]+|\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b/gi, '（详见诊断记录）')
    .replace(/\b(?:revision|source_ref|idempotency(?:_key)?|metrics|correlation_id|protocol_version)\b\s*[:=]?\s*(?:[\w.-]+)?/gi, '（详见诊断记录）')
    .replace(/\bfluctlight(?:\.[\w-]+)+\.v\d+\b|(?:协议)?版本\s*[:：]?\s*v?\d+(?:\.\d+)*/gi, '（详见诊断记录）')
    .replace(/\b\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})\b/g, value => time(value));
}

export function goalRecordStatus(value) {
  if (typeof value !== 'string') return '';
  const status = label(statusLabels, value) || stateLabel(/[\u3400-\u9fff]/u.test(value) ? goalRecordText(value) : value, '');
  return typeof status === 'string' ? status : '';
}

export function planningFailure(error, status) {
  const code = String(error?.code || error?.type || '');
  const detail = String(error?.message || error || '');
  if (error?.status === 401 || code === 'unauthorized') return '登录状态已失效，请重新登录后再试。';
  if (error?.status === 403 || code === 'forbidden') return '当前账户没有操作此目标规划的权限。';
  if (code === 'goal_planning_invalid') return '规划请求未受理，请刷新页面后再试。';
  if (error?.status === 409) return '目标资料已变化，当前输入已保留，请重新读取后核对。';
  if (status === 'retry' || status === 'deferred' || status === 'awaiting_trigger') {
    return '上次规划未完成，任务正在等待再次尝试。';
  }
  if ([408, 504].includes(error?.status) || /timeout|timed out|deadline exceeded/i.test(code + ' ' + detail)) {
    return '这次规划等待超时，可以稍后重试。';
  }
  if (/provider_request_failed|provider request failed|adk_run|NodeRunError|GPU memory/i.test(code + ' ' + detail)) {
    return '规划服务暂时未能完成这次请求，可以稍后重试。';
  }
  return '这次规划没有完成，可以稍后重试。';
}

export function goalRecordSummary(row, kind, goalNames) {
  const result = row.result;
  const source = row.source;
  const read = goalRecordText;
  const status = goalRecordStatus(row.to_status || row.status);
  if (kind === 'planning') {
    const decision = result?.decision || row.decision;
    const failure = row.error_code || row.error || result?.error_code || result?.error
      || (decision === 'failed' || row.status === 'failed' || result?.status === 'failed' ? result?.reason || row.reason || 'failed' : '');
    const suggestions = Array.isArray(result?.suggestions) ? result.suggestions : [];
    const snapshotGoals = Array.isArray(result?.snapshot?.goals) ? result.snapshot.goals : [];
    const nameFor = id => id && read(snapshotGoals.find(goal => goal?.id === id)?.desired_outcome || goalNames?.get(id));
    const names = suggestions.map(item => read(item?.desired_outcome) || read(item?.title)
      || read(item?.description) || nameFor(item?.goal_id || item?.id) || read(item));
    names.push(read(result?.desired_outcome) || nameFor(result?.goal_id || row.goal_id));
    if (Array.isArray(result?.goal_ids)) names.push(...result.goal_ids.map(nameFor));
    return {
      title: label(decisionLabels, decision) || '目标规划',
      status,
      facts: [
        ['规划时间', time(row.created_at || row.occurred_at)],
        ['规划方式', label(planningModes, row.mode)],
        ['涉及目标', [...new Set(names.filter(Boolean))].join('、')],
        ['规划原因', failure ? planningFailure(failure, row.status) : read(result?.reason) || read(row.reason)],
        ['复核条件', failure ? '' : read(result?.review_condition)],
      ],
    };
  }
  const sourceText = item => read(item?.desired_outcome) || read(item?.text) || read(item?.description) || read(item);
  if (kind === 'source') return {
    title: '初始化来源审查',
    status,
    facts: [
      ['来源内容', Array.isArray(row.raw_source) ? row.raw_source.map(sourceText).filter(Boolean).join('；') : sourceText(row.raw_source)],
      ['审查结果', label(operationLabels, row.decision?.operation)],
      ['审查原因', read(row.decision?.reason) || read(row.reason)],
      ['记录时间', time(row.created_at)],
    ],
  };
  const reason = read(row.reason) || read(row.decision_reason);
  const title = read(row.desired_outcome) || read(row.description) || label(operationLabels, row.operation)
    || label(reasonLabels, row.reason) || (kind === 'evidence' ? '目标证据' : domainName(read(row.operation || row.event_type), '目标记录'));
  return {
    title,
    status,
    facts: [
      ['记录时间', time(row.created_at || row.occurred_at || source?.recorded_at || source?.occurred_at)],
      ['证据来源', kind === 'evidence' ? label(evidenceKinds, source?.source_kind || source?.kind) : ''],
      [kind === 'evidence' ? '关联依据' : '变更原因', reason === title ? '' : reason],
      ['实际证据', read(source?.summary) || read(source?.source_data?.summary) || read(source?.source_data?.text)
        || read(source?.data?.summary) || read(source?.data?.text) || read(row.summary)],
      ['期望结果', read(row.snapshot?.desired_outcome)],
      ['完成条件', Array.isArray(row.snapshot?.success_criteria) ? row.snapshot.success_criteria.map(read).filter(Boolean).join('；') : ''],
      ['期望完成时间', time(row.snapshot?.deadline)],
    ],
  };
}

export function goalExecutionFacts(execution) {
  if (!execution) return [];
  const {last_attempt: attempt, last_result: result, last_evidence: evidence, evaluation_request: evaluation} = execution;
  return [
    ['推进状态', goalRecordStatus(execution.stage)],
    ['下一步', goalRecordText(execution.next_step)],
    ['当前阶段', goalRecordText(execution.current_stage?.purpose)],
    ['当前阻碍', goalRecordText(execution.blocker)],
    ['等待条件', goalRecordText(execution.wait_condition)],
    ['计划推进时间', time(execution.due_at)],
    ['下次复核时间', time(execution.next_review_at)],
    ['最近尝试', [goalRecordStatus(attempt?.status), time(attempt?.started_at)].filter(Boolean).join(' · ')],
    ['最近结果', [goalRecordStatus(result?.status), time(result?.occurred_at)].filter(Boolean).join(' · ')],
    ['结果说明', goalRecordText(result?.reason)],
    ['最近证据', [label(evidenceKinds, evidence?.kind), goalRecordStatus(evidence?.status), time(evidence?.occurred_at)].filter(Boolean).join(' · ')],
    ['证据依据', goalRecordText(evidence?.reason)],
    ['证据核对状态', goalRecordStatus(evaluation?.status)],
  ].filter(([, value]) => value);
}
