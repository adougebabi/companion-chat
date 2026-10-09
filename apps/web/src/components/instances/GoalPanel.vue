<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { BrowserApiError, BrowserClient, type BrowserGoal, type BrowserGoalCommand } from "@fluctlight/browser-client";
import Button from "@/components/ui/button/Button.vue";
import Input from "@/components/ui/input/Input.vue";
import Textarea from "@/components/ui/textarea/Textarea.vue";
import { apiOrigin } from "../../runtime-config";
import { formatTimelineTime } from "../../lib/fluctlight-display";

const props = defineProps<{ fluctlightId: string; readOnly?: boolean; actors?:{id:string;label:string}[]; profileOptions?:{id:string;label:string}[] }>();
const client = new BrowserClient(apiOrigin);
const historyTab = ref(false);
const goals = ref<BrowserGoal[]>([]);
const cursor = ref("");
const selected = ref<BrowserGoal | null>(null);
const revisions = ref<Record<string, unknown>[]>([]);
const revisionCursor = ref("");
const evidenceCursor = ref("");
const evidenceBusy = ref(false);
const busy = ref(false);
const error = ref("");
const editing = ref(false);
const desired = ref("");
const motivation = ref("");
const criteria = ref("");
const reason = ref("");
const candidateOnly=ref(false);
const targetActorId=ref(""),goalProfileId=ref("");
const ownerProtected=ref(false);
const deadline = ref("");
const deadlinePolicy = ref<"soft" | "hard">("soft");
const goalScope=ref("general");
const missedThreshold=ref(3);
const ineffectiveThreshold=ref(3);
const resetCounters=ref(false);
const frozenRevision = ref(0);
const editorGoalId = ref<string | null>(null);
const revisionBusy = ref(false);
let scopeRevision = 0;
let selectionRevision = 0;
let pending: { fingerprint: string; key: string } | null = null;
const statusLabels: Record<string, string> = { candidate: "候选", active: "推进中", paused: "已暂停", completed: "已完成", cancelled: "已取消", abandoned: "已放弃" };
const reasonLabels: Record<string, string> = { progressed:"有真实推进", no_opportunity:"没有合适机会",not_yet_due:"时机未到",waiting_external:"等待外部结果",ineffective_attempt:"已有尝试但未推进",blocked:"存在阻碍",missed_opportunity:"需复核可能错失的机会",evaluation_incomplete:"评估尚未完成",refusal:"尊重明确拒绝",sleeping:"睡眠期间" };
const terminal = computed(() => selected.value && ["completed", "cancelled", "abandoned"].includes(selected.value.status));
function record(value: unknown): Record<string, unknown> { return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {}; }
function records(value: unknown): Record<string, unknown>[] { return Array.isArray(value) ? value.map(record) : []; }
function message(cause: unknown): string {
 if (cause instanceof BrowserApiError) {
  if (cause.code === "goal_capacity_exceeded") return "活动目标名额已满。草稿已保留，请先暂停一个目标，或仅保存为候选。";
  if (cause.code === "goal_candidate_capacity_exceeded") return "候选目标数量已达上限。草稿已保留，请先处理现有候选。";
  if (cause.status === 409) return "目标已发生变化。草稿已保留，请重新读取目标后再决定如何修改。";
 }
 return cause instanceof Error ? cause.message : "目标操作失败，请重试。";
}
async function load(append = false) {
 const scope = scopeRevision; const id = props.fluctlightId;
 if (!id) return;
 busy.value = true; error.value = "";
 try { const page = await client.goals(id, { history: historyTab.value, cursor: append ? cursor.value : "" }); if (scope !== scopeRevision) return; goals.value = append ? [...goals.value, ...page.items.filter(item => !goals.value.some(old => old.id === item.id))] : page.items; cursor.value = page.next_cursor ?? ""; }
 catch (cause) { if (scope === scopeRevision) error.value = message(cause); }
 finally { if (scope === scopeRevision) busy.value = false; }
}
async function inspect(goal: BrowserGoal) {
 const scope = scopeRevision; const selection = ++selectionRevision; const id = props.fluctlightId;
 revisionBusy.value = false; evidenceBusy.value = false;
 busy.value = true; error.value = "";
 try { const [detail, history] = await Promise.all([client.goalDetail(id, goal.id), client.goalHistory(id, goal.id)]); if (scope !== scopeRevision || selection !== selectionRevision) return; selected.value = detail; evidenceCursor.value = detail.evidence_next_cursor ?? ""; revisions.value = history.items; revisionCursor.value = history.next_cursor ?? ""; }
 catch (cause) { if (scope === scopeRevision && selection === selectionRevision) error.value = message(cause); }
 finally { if (scope === scopeRevision && selection === selectionRevision) busy.value = false; }
}
function edit(create = false) {
 editing.value = true; const goal = create ? null : selected.value;goalScope.value=goal?.scope??"general"; editorGoalId.value = goal?.id ?? null;
 if (create) selected.value = null;
 ownerProtected.value=record(goal).owner_protected===true;candidateOnly.value=false;targetActorId.value=String(record(goal).target_actor_id??"");goalProfileId.value=goal?.profile_id??"";
 desired.value = goal?.desired_outcome ?? ""; motivation.value = goal?.motivation ?? ""; criteria.value = goal?.success_criteria.join("\n") ?? "";
 const policy=record(record(goal?.execution?.review_governance).policy);missedThreshold.value=Number(policy.missed_opportunity_threshold??3);ineffectiveThreshold.value=Number(policy.ineffective_attempt_threshold??3);resetCounters.value=false;
 deadline.value = goal?.deadline ?? ""; deadlinePolicy.value = goal?.deadline_policy ?? "soft"; frozenRevision.value = goal?.revision ?? 0; reason.value = ""; pending = null;
}
async function command(operation: "create" | "update" | "pause" | "resume" | "cancel" | "abandon" | "reassess") {
 if (props.readOnly || busy.value || !reason.value.trim()) return;
 const scope = scopeRevision; const id = props.fluctlightId; const goal = selected.value;
 const targetId = operation === "update" ? editorGoalId.value : goal?.id;
 const body: BrowserGoalCommand = { expectedRevision: operation === "update" ? frozenRevision.value : goal?.revision ?? 0, reason: reason.value.trim(), idempotencyKey: "" };
 if (operation === "create" || operation === "update") {
  body.candidateOnly=candidateOnly.value;body.ownerProtected=ownerProtected.value;body.profileId=goalProfileId.value;if(operation==="create")body.targetActorId=targetActorId.value;
  if(operation==="create")body.scope=goalScope.value;
  body.desiredOutcome = desired.value.trim(); body.motivation = motivation.value.trim(); body.successCriteria = criteria.value.split("\n").map(text => text.trim()).filter(Boolean); body.deadlinePolicy = deadlinePolicy.value;body.reviewPolicy={missed_opportunity_threshold:missedThreshold.value,ineffective_attempt_threshold:ineffectiveThreshold.value};body.resetReviewCounters=resetCounters.value;
  if (deadline.value.trim()) { const parsed = new Date(deadline.value); if (!/(Z|[+-]\d{2}:\d{2})$/.test(deadline.value) || !Number.isFinite(parsed.getTime())) { error.value = "期限需填写带时区的时间，例如 2026-10-10T18:00:00+08:00。"; return; } body.deadline = parsed.toISOString(); } else body.clearDeadline = true;
 }
 const fingerprint = JSON.stringify({ id, goalId: targetId, operation, body });
 if (!pending || pending.fingerprint !== fingerprint) pending = { fingerprint, key: `goal-command-${Date.now()}-${crypto.getRandomValues(new Uint32Array(4)).join("-")}` };
 body.idempotencyKey = pending.key;
 busy.value = true; error.value = "";
 try { const result = operation === "create" ? await client.createGoal(id, body) : await client.goalCommand(id, targetId!, operation, body); if (scope !== scopeRevision) return; pending = null; editing.value = false; reason.value = ""; await load(); const detail = await client.goalDetail(id, result.goal_id); if (scope !== scopeRevision) return; await inspect(detail); }
 catch (cause) { if (scope === scopeRevision) error.value = message(cause); }
 finally { if (scope === scopeRevision) busy.value = false; }
}
async function moreRevisions() {
 if (!selected.value || !revisionCursor.value || revisionBusy.value) return;
 const scope=scopeRevision; const selection=selectionRevision; const goal=selected.value; revisionBusy.value=true;
 try { const page=await client.goalHistory(props.fluctlightId,goal.id,revisionCursor.value); if(scope!==scopeRevision||selection!==selectionRevision||selected.value?.id!==goal.id)return; revisions.value.push(...page.items.filter(item=>!revisions.value.some(old=>old.id===item.id)));revisionCursor.value=page.next_cursor??""; }
 catch(cause) { if(scope===scopeRevision&&selection===selectionRevision)error.value=message(cause); }
 finally { if(scope===scopeRevision&&selection===selectionRevision)revisionBusy.value=false; }
}
async function moreEvidence() {
 if (!selected.value || !evidenceCursor.value || evidenceBusy.value) return;
 const scope=scopeRevision; const selection=selectionRevision; const goal=selected.value; evidenceBusy.value=true;
 try {
  const page=await client.goalEvidence(props.fluctlightId,goal.id,evidenceCursor.value);
  if(scope!==scopeRevision||selection!==selectionRevision||selected.value?.id!==goal.id)return;
  const previous=selected.value.evidence??[];
  selected.value={...selected.value,evidence:[...previous,...page.items.filter(item=>!previous.some(old=>old.id===item.id))]};
  evidenceCursor.value=page.next_cursor??"";
 } catch(cause) { if(scope===scopeRevision&&selection===selectionRevision)error.value=message(cause); }
 finally { if(scope===scopeRevision&&selection===selectionRevision)evidenceBusy.value=false; }
}
async function reloadDraftAuthority() {
 if(!editorGoalId.value||busy.value)return;
 const scope=scopeRevision;busy.value=true;
 try{const detail=await client.goalDetail(props.fluctlightId,editorGoalId.value);if(scope!==scopeRevision)return;selected.value=detail;frozenRevision.value=detail.revision;error.value="";pending=null;}
 catch(cause){if(scope===scopeRevision)error.value=message(cause);}
 finally{if(scope===scopeRevision)busy.value=false;}
}
async function refresh() { await load(); if(selected.value) await inspect(selected.value); }

watch(() => props.fluctlightId, () => { scopeRevision++; selectionRevision++; goals.value = []; selected.value = null; revisions.value=[]; revisionCursor.value=""; evidenceCursor.value=""; evidenceBusy.value=false; revisionBusy.value=false; editing.value=false; reason.value=""; pending=null; void load(); }, { immediate: true });
watch(historyTab, () => { scopeRevision++; selectionRevision++; selected.value=null; revisions.value=[]; revisionCursor.value=""; evidenceCursor.value=""; evidenceBusy.value=false; revisionBusy.value=false; editing.value=false; pending=null; void load(); });
</script>

<template>
 <section class="goal-panel" aria-label="目标与推进记录">
  <div class="goal-toolbar"><h3>目标与推进记录</h3><Button variant="outline" :disabled="busy" @click="refresh()">刷新</Button><Button v-if="!readOnly" :disabled="busy" @click="edit(true)">创建目标</Button></div>
  <div class="goal-toolbar"><Button :variant="historyTab ? 'outline' : 'default'" :disabled="busy" @click="historyTab=false">当前目标</Button><Button :variant="historyTab ? 'default' : 'outline'" :disabled="busy" @click="historyTab=true">已结束目标</Button></div>
  <p v-if="error" role="alert">{{ error }}</p><p v-if="busy" role="status">正在读取或提交目标…</p>
  <p v-if="!busy && !goals.length">{{ historyTab ? '暂无已结束目标。' : '暂无当前目标。' }}</p>
  <ul class="goal-list"><li v-for="goal in goals" :key="goal.id"><button type="button" :disabled="busy || editing" @click="inspect(goal)"><strong>{{ goal.desired_outcome }}</strong><span>{{ statusLabels[goal.status] }} · 标准版本 {{ goal.criteria_version }}</span></button></li></ul>
  <Button v-if="cursor" variant="outline" :disabled="busy" @click="load(true)">读取更多目标</Button>
  <article v-if="selected" class="goal-detail">
   <h4>{{ selected.desired_outcome }}</h4><p>{{ selected.motivation }}</p><p>{{ statusLabels[selected.status] }} · {{ selected.profile_id ? `人格：${selected.profile_id}` : '共享目标' }} · 修订 {{ selected.revision }}</p>
   <p v-if="selected.deadline">{{ selected.deadline_policy === 'hard' ? '硬期限' : '软期限（到期复核）' }}：{{ formatTimelineTime(selected.deadline) }}</p>
   <details><summary>生成来源、对象与前置依赖</summary><p>对象 Actor：{{record(selected).target_actor_id || '未指定'}} · {{record(selected).owner_protected ? 'Owner保留' : '可按正式策略复核'}}</p><pre>{{JSON.stringify(record(selected).planner_source ?? {},null,2)}}</pre><p v-for="dependency in records(record(selected).dependencies)" :key="String(dependency.prerequisite_id)">前置：{{dependency.desired_outcome}} · {{dependency.satisfied ? '已完成' : dependency.status}}</p><p>来源于旧目标只用于追溯；这里的前置依赖才会限制新关联行动。</p></details>
   <ol><li v-for="(text,index) in selected.success_criteria" :key="selected.criterion_ids[index]">{{ text }}<small> · {{ selected.criterion_ids[index] }}</small></li></ol>
   <dl><dt>下一步</dt><dd>{{ selected.execution?.next_step || '尚待规划或已结束' }}</dd><dt>等待条件</dt><dd>{{ selected.execution?.wait_condition || '无' }}</dd><dt>当前阻碍</dt><dd>{{ selected.execution?.blocker || '无' }}</dd><dt>评估状态</dt><dd>{{ record(selected.execution?.evaluation_request).status || '尚无评估请求' }}</dd></dl>
   <dl><dt>最近尝试</dt><dd>{{ record(selected.execution?.last_attempt).status || '没有执行尝试记录' }} · {{ record(selected.execution?.last_attempt).attempt_id || '' }}</dd><dt>实际结果</dt><dd>{{ record(selected.execution?.last_result).kind || '' }} · {{ record(selected.execution?.last_result).status || '尚无结果' }} · {{ record(selected.execution?.last_result).source_id || record(selected.execution?.last_result).outcome_id || '' }}</dd><dt>停止或重试原因</dt><dd>{{ selected.execution?.retry_reason || record(selected.execution?.last_attempt).error_code || record(selected.execution?.evaluation_request).error_code || '无' }}</dd><dt>下一次重试</dt><dd>{{ selected.execution?.next_retry_at || '未安排' }}</dd><dt>待结算</dt><dd>{{ selected.execution?.ready_for_settlement ? '标准已满足，等待恢复或明确结算' : '无' }}</dd></dl>
   <div v-for="stage in selected.stages" :key="String(stage.id)"><h5>阶段：{{ stage.purpose }}</h5><p>{{ stage.strategy }} · {{ stage.status }}</p></div>
   <div v-for="item in selected.commitments" :key="String(item.id)"><h5>短期预期：{{ item.expected_result }}</h5><p>{{ item.status }} · {{ item.blocker || '无阻碍' }}</p></div>
   <details><summary>逐项评估与事实来源</summary><div v-for="evaluation in selected.evaluations" :key="String(evaluation.id)"><p>{{ evaluation.impact }} · 标准版本 {{ evaluation.criteria_version }}</p><ul><li v-for="judgment in records(evaluation.judgments)" :key="String(judgment.criterion_id)">{{ judgment.criterion_id }}：{{ judgment.verdict }} · {{ judgment.reason }}</li></ul></div><ul><li v-for="evidence in selected.evidence" :key="String(evidence.id)">{{ record(evidence.source).source_kind }}：{{ record(evidence.source).source_id }} · {{ evidence.status }} · {{ evidence.reason }}</li></ul><Button v-if="evidenceCursor" variant="outline" :disabled="evidenceBusy" @click="moreEvidence">读取更早证据</Button></details>
   <details><summary>周期复盘</summary><p v-for="review in selected.reviews" :key="String(review.id)">{{ review.local_date }}（{{ review.timezone }}）· {{ reasonLabels[String(review.reason_category)] || review.status }} · {{ review.explanation }} · {{ review.next_step }}</p></details>
   <details><summary>修订与治理历史</summary><p v-for="revision in revisions" :key="String(revision.id)">{{ revision.created_at }} · {{ revision.source === 'owner' ? 'Owner 操作' : '运行时评估' }} · {{ revision.operation }} · {{ revision.reason }}</p><Button v-if="revisionCursor" variant="outline" :disabled="revisionBusy" @click="moreRevisions">读取更早修订</Button></details>
   <div v-if="!readOnly && !terminal && !editing" class="goal-actions"><label>操作说明<Input v-model="reason" maxlength="1000" /></label><div class="goal-toolbar"><Button variant="outline" :disabled="busy" @click="edit()">编辑定义与期限</Button><Button v-if="selected.status==='active'" :disabled="busy||!reason.trim()" @click="command('pause')">暂停</Button><Button v-if="selected.status==='paused'||selected.status==='candidate'" :disabled="busy||!reason.trim()" @click="command('resume')">恢复</Button><Button :disabled="busy||!reason.trim()" @click="command('reassess')">重新评估</Button><Button variant="outline" :disabled="busy||!reason.trim()" @click="command('cancel')">取消</Button><Button variant="outline" :disabled="busy||!reason.trim()" @click="command('abandon')">放弃</Button></div></div>
  </article>
  <form v-if="editing && !readOnly" class="goal-editor" @submit.prevent="command(editorGoalId ? 'update' : 'create')"><h4>{{ selected ? '编辑目标' : '创建目标' }}</h4><label>期望结果<Input v-model="desired" required maxlength="2000" /></label><label v-if="!editorGoalId"><input v-model="candidateOnly" type="checkbox" /> 仅保存候选（不占活动名额、不执行）</label><label v-if="!editorGoalId">人格作用域<select v-model="goalProfileId"><option value="">实例共享</option><option v-for="profile in profileOptions ?? []" :key="profile.id" :value="profile.id">{{profile.label}}</option></select></label><label v-if="!editorGoalId">对应Actor<select v-model="targetActorId"><option value="">未指定（关系目标默认当前Owner）</option><option v-for="actor in actors ?? []" :key="actor.id" :value="actor.id">{{actor.label}}</option></select></label><label v-if="!editorGoalId">目标含义<select v-model="goalScope"><option value="general">一般目标（含表达心意）</option><option value="relationship">双方关系目标（需双方明确确认）</option></select></label><label>动机<Textarea v-model="motivation" required /></label><label>成功标准（每行一项）<Textarea v-model="criteria" required rows="4" /></label><label>期限（可选，带时区）<Input v-model="deadline" placeholder="2026-10-10T18:00:00+08:00" /></label><label>期限策略<select v-model="deadlinePolicy"><option value="soft">软期限：到期复核</option><option value="hard">硬期限：到期停止新行动</option></select></label><label>连续错失机会复核阈值<Input v-model.number="missedThreshold" type="number" min="1" max="30" /></label><label>连续尝试无效复核阈值<Input v-model.number="ineffectiveThreshold" type="number" min="1" max="30" /></label><label><input v-model="resetCounters" type="checkbox" /> 重置当前复核计数（保留历史）</label><label><input v-model="ownerProtected" type="checkbox" /> 保留此目标（自动规划不能暂停或取消）</label><label>修改说明<Input v-model="reason" required maxlength="1000" /></label><p>修改标准后会重新评估真实证据。</p><Button v-if="editorGoalId" type="button" variant="outline" :disabled="busy" @click="reloadDraftAuthority">读取新版本并保留草稿</Button><div class="goal-toolbar"><Button type="submit" :disabled="busy">保存</Button><Button type="button" variant="outline" @click="editing=false">关闭草稿</Button></div></form>
 </section>
</template>

<style scoped>
.goal-panel {display:grid;gap:1rem;min-width:0}.goal-toolbar{display:flex;align-items:center;gap:.6rem;flex-wrap:wrap}.goal-list{display:grid;gap:.5rem;padding:0;list-style:none}.goal-list button{display:grid;gap:.25rem;text-align:left;width:100%;padding:.8rem;border:1px solid var(--border);border-radius:.5rem;overflow-wrap:anywhere}.goal-detail,.goal-editor{display:grid;gap:.8rem;border-top:1px solid var(--border);padding-top:1rem;overflow-wrap:anywhere}.goal-editor label,.goal-actions label{display:grid;gap:.3rem}.goal-detail dl{display:grid;grid-template-columns:minmax(5rem,auto) 1fr;gap:.35rem .8rem}.goal-detail small{opacity:.65;font-size:.75rem}.goal-detail summary{cursor:pointer}.goal-detail h4{font-weight:600}.goal-detail li{margin:.35rem 0}.goal-list span{font-size:.8rem;opacity:.75}
pre{white-space:pre-wrap;overflow-wrap:anywhere}
.goal-panel :deep(.bg-primary) { color: var(--primary-foreground); }
</style>
