import { Fragment } from "react";
import { Button } from "@/components/ui/button";
import { labelFor, recordText, recordTime, scoreText } from "@/approved-ui/memory-record-summary.mjs";
import { profileEnumLabel, reflectionSubject } from "@/approved-ui/behavior-enum-labels.mjs";
import { selfCategoryLabels } from "@/v3/proto/components/self-claims.js";
import { stateLabel } from "@/v3/domain-presentation.mjs";
import { requestDecisionText } from "@/v3/proto/components/request-failure.js";
import JsonDetails from "../shared/JsonDetails";
import { ClaimValueSummary } from "../shared/ProfileRecordSummary";

export const memoryDownload = "原始记忆记录（诊断）";
export const reflectionDownload = "原始反思记录（诊断）";
export const claimDownload = "原始认识记录（诊断）";

export function RecordDownload({ value, title }) {
  return value && Object.keys(value).length ? <JsonDetails value={value} title={title} downloadOnly /> : null;
}

export function SummaryFacts({ facts }) {
  const present = facts.filter(([, value]) => value);
  return present.length ? (
    <dl className="replica-key-values">
      {present.map(([label, value]) => (
        <Fragment key={label}><dt>{label}</dt><dd>{value}</dd></Fragment>
      ))}
    </dl>
  ) : null;
}

export const reflectionStatus = value => labelFor({
  pending:"等待处理", queued:"等待处理", scheduled:"已安排", running:"正在处理", applying:"正在处理",
  applied:"已应用", completed:"已完成", failed:"处理未完成", rejected:"暂未采用",
  superseded:"已有新记录", no_change:"未产生变更", cancelled:"已取消", processing:"正在处理",
  retry_wait:"等待重试", dispatched:"已安排", terminal:"处理已结束",
}, value) || (value ? stateLabel(value, "") : "");

export const reflectionTitle = value => /^反思窗口\s*\d+\s*[—–-]\s*\d+$/.test(value) ? '' : recordText(value);

const domains = {
  active_memory:["近期记忆","active_memory_candidates"], memory:["记忆","memory_candidates"],
  relationship:["关系观察","relationship_observations"], goal:["目标","goal_candidates"],
  intention:["行动意图","intention_candidates"],
  affect_profile:["情绪调整","affect_recalibration_candidates"], drive:["动机","drive_candidates"],
  preference:["偏好","preference_candidates"], trigger:["触发规律","trigger_candidates"],
  developing_self:["成长认识","developing_self_candidates"], personality_overlay:["人格调整","personality_evolution_candidates"],
  behavior_policy_overlay:["行为方式调整","behavior_policy_evolution_candidates"],
};
const dispositions = {accepted:"已采纳", applied:"已应用", rejected:"未采纳", deferred:"暂缓采纳", no_change:"维持原状"};
const reasons = {
  domain_reducer_unavailable:"当前无法应用这类提议", confidence_below_threshold:"现有依据不足",
  cross_window_evidence_required:"还需要其他时期的依据", cooldown_active:"距离上次调整时间较短",
  semantic_no_change:"提议与现状一致", exact_duplicate:"已有相同内容",
  reflection_memory_source_stale:"相关记忆的依据已变化", field_forbidden:"这项设定不可通过反思调整",
  field_not_allowlisted:"这项设定超出反思可调整范围",
};
const operations = {
  create:"新增", update:"更新", revise:"修订", merge:"合并", confirm:"确认", complete:"完成",
  expire:"到期", deprecate:"停用", pause:"暂停", resume:"恢复", cancel:"取消", no_change:"维持原状",
};
const sourceKinds = {
  "conversation.turn":"对话交流", "life.owner_request.submitted":"用户提交的请求",
  "autonomy.result":"自主行动结果", "internal.wake_up":"日常唤醒",
};
const directions = {increase:"增强", strengthen:"增强", improve:"改善", positive:"改善", toward:"增强",
  decrease:"减弱", weaken:"减弱", negative:"减弱", away:"减弱", maintain:"维持"};
const items = value => Array.isArray(value) ? value.filter(Boolean) : [];
const textList = value => items(value).map(recordText).filter(Boolean).join("、");
const countText = value => Number.isSafeInteger(value) && value > 0 ? value + " 项" : "";

export function ReflectionObservations({ record }) {
  const summary = recordText(record.summary) || recordText(record.proposal?.summary);
  const body = recordText(record.body);
  const evidence = items(record.source_window?.evidence);
  const times = evidence.map(item => item.occurred_at).filter(recordTime).sort((a, b) => Date.parse(a) - Date.parse(b));
  const counts = new Map();
  for (const item of evidence) {
    const label = labelFor(sourceKinds, item.kind) || "其他反思素材";
    counts.set(label, (counts.get(label) || 0) + 1);
  }
  return <>
    {summary && <p className="lp-copy">{summary}</p>}
    {body && body !== summary && <p className="lp-copy">{body}</p>}
    <SummaryFacts facts={[
      ["反思时间", recordTime(record.at) || recordTime(record.created_at)],
      ["反思来源", [...counts].map(([label, count]) => label + " " + count + " 条").join("、")],
      ["素材发生时间", [...new Set([recordTime(times[0]), recordTime(times.at(-1))])].filter(Boolean).join(" 至 ")],
      ["关联行动结果", countText(items(record.source_window?.outcomes).length)],
      ["来源可追溯性", record.source_window_status === "legacy_snapshot_unavailable" ? "较早记录的素材范围无法核对" : ""],
    ]} />
  </>;
}

function candidateFacts(candidate) {
  const subject = reflectionSubject(candidate.field_path || candidate.target);
  return [
    ["提议操作", labelFor(operations, candidate.operation)],
    ["置信度", scoreText(candidate.confidence)],
    ["重要程度", scoreText(candidate.importance)],
    ["情感意义", scoreText(candidate.emotional_significance)],
    ["能力约束", textList(candidate.capability_constraints)],
    ["内容", recordText(candidate.content)],
    ["认识内容", recordText(candidate.claim)],
    ["认识类别", labelFor(selfCategoryLabels, candidate.category)],
    ["关系观察", recordText(candidate.observation)],
    ["期望结果", recordText(candidate.desired_outcome) || recordText(candidate.expected_outcome)],
    ["完成标准", textList(candidate.success_criteria)],
    ["动机", recordText(candidate.motivation)],
    ["行动意图", recordText(candidate.action_intent)],
    ["调整对象", subject.label],
    ["拟调整内容", profileEnumLabel(recordText(candidate.semantic_value), subject.path)],
    ["变化方向", labelFor(directions, candidate.direction)],
    ["主要情绪规律", textList(candidate.dominant_patterns)],
    ["情绪触发因素", textList(candidate.triggers)],
    ["恢复方式", textList(candidate.recovery_patterns)],
    ["情绪冲突", textList(candidate.conflicts)],
    ["提议原因", recordText(candidate.semantic_reason)],
    ["生效时间", recordTime(candidate.valid_from)],
    ["有效期至", recordTime(candidate.valid_until)],
    ["期望行动时间", recordTime(candidate.preferred_time)],
    ["截止时间", recordTime(candidate.expiration)],
    ["原时间表述", recordText(candidate.original_time_expression)],
    ["触发条件", labelFor({time:"到达指定时间", event:"相关事件发生", semantic:"符合语义条件"}, candidate.typed_trigger?.type)],
    ["触发事件", labelFor(sourceKinds, candidate.typed_trigger?.event_type)],
    ["触发时间", recordTime(candidate.typed_trigger?.due_at)],
    ["目标进展", candidate.complete === true ? "提议标记为完成" : ""],
  ];
}

export function ReflectionChanges({ record, fid }) {
  const candidates = new Map();
  for (const [domain, [label, field]] of Object.entries(domains)) {
    const offset = domain === "affect_profile" ? 1 : 0;
    const group = new Map();
    (Array.isArray(record.proposal?.[field]) ? record.proposal[field] : []).forEach((candidate, index) => {
      if (candidate && typeof candidate === "object") group.set(index + offset, {candidate, label});
    });
    if (domain === "affect_profile" && record.proposal?.emotional_summary) {
      group.set(0, {candidate:record.proposal.emotional_summary, label:"情绪总结"});
    }
    candidates.set(domain, group);
  }
  const decisions = items(record.dispositions).map(decision => {
    const group = candidates.get(decision.domain);
    const match = group?.get(decision.candidate_index);
    group?.delete(decision.candidate_index);
    return { ...match, label:match?.label || labelFor(domains, decision.domain)?.[0] || "其他变化", decision };
  });
  const proposals = [...candidates.values()].flatMap(group => [...group.values()]);
  const references = new Map(items(record.references).map(row => [row.ref, row]));
  const outcome = record.result || {};
  const summaries = [...decisions, ...proposals].map(({ candidate = {}, label, decision }, index) => {
    const facts = [
      ...candidateFacts(candidate),
      ["实际处置", labelFor(dispositions, decision?.disposition)],
      ["处理原因", labelFor(reasons, decision?.reason_code)],
    ];
    const reference = references.get(decision?.changed_ref) || references.get(decision?.target_ref) || references.get(candidate.target_ref);
    const target = labelFor({memory:["growth/memory", "查看记忆"], goal:["life/goal", "查看目标"]}, reference?.kind);
    return facts.some(([, value]) => value) || (target && reference?.entity_id) ? (
      <div className="lp-review-card" key={index}>
        <h4>{label}</h4>
        <SummaryFacts facts={facts} />
        {target && reference.entity_id && <Button asChild variant="link"><a href={"#/c/" + encodeURIComponent(fid) + "/" + target[0] + "/" + encodeURIComponent(reference.entity_id)}>{target[1]}</a></Button>}
      </div>
    ) : null;
  });
  const counts = Object.keys(domains).flatMap(domain => {
    const count = outcome.counts?.[domain] || (domain === "memory" ? outcome.memory?.counts : domain === "active_memory" ? outcome.active_memory?.counts : null);
    const text = [
      countText(count?.applied) && "已应用 " + countText(count.applied),
      countText(count?.rejected) && "未采纳 " + countText(count.rejected),
      countText(count?.deferred) && "暂缓 " + countText(count.deferred),
      countText(count?.no_change) && "维持原状 " + countText(count.no_change),
    ].filter(Boolean).join("、");
    return text ? [[domains[domain][0], text]] : [];
  });
  return <>
    <SummaryFacts facts={[
      ["应用结果", reflectionStatus(outcome.status)],
      ["已改变的记录", countText(items(outcome.changed_refs).length)],
      ...counts,
    ]} />
    {summaries}
  </>;
}

const feedbackStatus = {
  pending:"等待回应", processing:"正在考虑", accepted:"已接受，等待实际行动",
  completed:"本次处理已结束", rejected:"暂未采纳", deferred:"稍后决定", failed:"处理失败",
  cancelled:"已取消",
};

export function ReflectionFeedbackSummary({ record }) {
  return <SummaryFacts facts={[
    ["反馈", recordText(record.payload?.text)],
    ["态度", labelFor({agree:"赞同", disagree:"有不同看法"}, record.payload?.stance)],
    ["处理状态", labelFor(feedbackStatus, record.status)],
    ["提交时间", recordTime(record.created_at)],
    ["处理说明", recordText(requestDecisionText(record))],
  ]} />;
}

export const claimStatuses = {
  active:"已记录的认识", uncertain:"仍不确定", forgotten:"已遗忘", superseded:"已有替代",
};

export function ClaimSummary({ record }) {
  const claim = recordText(record.claim);
  return <>
    {claim && <blockquote className="lp-memory-quote">{claim}</blockquote>}
    <SummaryFacts facts={[
      ["类别", labelFor(selfCategoryLabels, record.category)],
      ["状态", labelFor(claimStatuses, record.status)],
      ["置信度", scoreText(record.confidence)],
      ["来源有效", record.source_valid === true ? "是" : record.source_valid === false ? "来源需要重新核对" : ""],
      ["有效期限", recordTime(record.expires_at)],
      ["有效性", record.expired === true ? "已过期" : ""],
      ["形成时间", recordTime(record.created_at)],
      ["最近更新", recordTime(record.updated_at)],
    ]} />
    <ClaimValueSummary value={record.value} title="认识内容补充" />
  </>;
}
