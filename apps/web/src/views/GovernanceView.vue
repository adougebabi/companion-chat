<script setup lang="ts">
import { computed, ref } from "vue";

import Button from "@/components/ui/button/Button.vue";
import Input from "@/components/ui/input/Input.vue";
import Textarea from "@/components/ui/textarea/Textarea.vue";
import { useConversationStore } from "../stores/conversations";
import { useControlCenterStore } from "../stores/control-center";
import { enumLabel, formatDisplayValue, formatZonedRange, labelFor, resolveTimezone } from "../lib/fluctlight-display";
import { fluctlightStatusLabel } from "../lib/fluctlight-status";

const emit = defineEmits<{ close: []; retired: [] }>();
const store = useConversationStore();
const controlCenter = useControlCenterStore();
const retirementReason = ref("");
const retirementConfirmation = ref("");
const governanceTimezone = computed(() => {
  const identity = controlCenter.fluctlightDetail?.identity;
  const timezone = identity && typeof identity === "object" && !Array.isArray(identity) ? (identity as Record<string, unknown>).timezone : undefined;
  return resolveTimezone(typeof timezone === "string" ? timezone : undefined);
});
const growthSlots = computed(() => {
  const detail = controlCenter.fluctlightDetail;
  if (!detail) return [] as Array<Record<string, unknown>>;
  const drives = Array.isArray(detail.drive_slots) ? detail.drive_slots as Array<Record<string, unknown>> : [];
  const preferences = Array.isArray(detail.preference_slots) ? detail.preference_slots as Array<Record<string, unknown>> : [];
  return [...drives, ...preferences];
});

async function retire() {
  const id = store.fluctlightId;
  const name = store.selectedFluctlightName;
  if (!id || !name) return;
  if (retirementConfirmation.value.trim() !== name) {
    controlCenter.error = "请输入完整的摇光名称以确认删除。";
    return;
  }
  const retired = await controlCenter.retireFluctlight(id, retirementReason.value);
  if (!retired) return;
  retirementReason.value = "";
  retirementConfirmation.value = "";
  await store.bootstrap();
  emit("retired");
}
function capabilityRequestStatus(value: unknown): string {
  const labels: Record<string, string> = { proposed: "待审核", reviewing: "评估中", accepted: "已接受", rejected: "已拒绝", fulfilled: "已接入", cancelled: "已取消" };
  return labels[String(value)] ?? String(value ?? "未知");
}
</script>

<template>
  <section v-if="store.selectedFluctlight" class="page governance-page" aria-labelledby="governance-title">
    <header class="page-header governance-header">
      <div>
        <Button class="back-link" variant="ghost" type="button" @click="emit('close')">← 返回实例</Button>
        <p class="eyebrow">编辑与治理</p>
        <h1 id="governance-title">{{ store.selectedFluctlightName }} 的编辑与治理</h1>
        <p class="page-lede">这里的修改会影响后续状态。身份、人格、生活脉络和私有动态请从对话标题栏打开详情。</p>
      </div>
      <span class="status-pill" :class="{ paused: controlCenter.fluctlightDetail?.status === 'paused' }">
        {{ fluctlightStatusLabel(controlCenter.fluctlightDetail?.status) }}
      </span>
    </header>

    <p v-if="controlCenter.error" class="error-banner" role="alert">{{ controlCenter.error }}</p>

    <template v-if="controlCenter.fluctlightDetail">
      <section class="governance-section governance-overview">
        <div class="section-heading"><span class="section-index">01</span><div><p class="eyebrow">概览</p><h2>当前状态</h2></div></div>
        <p class="field-note">暂停会阻止新的自主外部行为，历史事实和已观察到的状态不会被删除。</p>
        <div class="governance-inline-form">
          <label for="governance-reason">操作原因</label>
          <Input id="governance-reason" v-model="controlCenter.governanceReason" maxlength="1024" placeholder="填写暂停或恢复的原因" />
          <Button class="secondary-button" variant="outline" type="button" :disabled="controlCenter.saving || !controlCenter.governanceReason.trim()" @click="controlCenter.setFluctlightStatus(store.fluctlightId, controlCenter.fluctlightDetail?.status === 'paused' ? 'active' : 'paused')">
            {{ controlCenter.fluctlightDetail.status === "paused" ? "恢复自主性" : "暂停自主性" }}
          </Button>
        </div>
      </section>

      <details class="governance-section">
        <summary class="section-heading"><span class="section-index">02</span><div><p class="eyebrow">生活世界操作</p><h2>日程与事件</h2></div><span class="disclosure-icon" aria-hidden="true">⌄</span></summary>
        <p v-if="controlCenter.fluctlightDetail.schedule" class="field-note">当前已有接受的本地日程，可在详情页查看时间轴。</p>
        <p v-else class="field-note">日程待生成，当前没有接受的本地日计划。</p>
        <Button v-if="controlCenter.fluctlightDetail.schedule" class="secondary-button" variant="outline" type="button" :disabled="controlCenter.saving" @click="controlCenter.cancelSchedule(store.fluctlightId)">取消当前日程</Button>
        <form class="governance-form" @submit.prevent="controlCenter.acceptSchedule(store.fluctlightId)"><label for="schedule-draft">完整日程 JSON</label><Textarea id="schedule-draft" v-model="controlCenter.scheduleDraftJson" rows="6" spellcheck="false" placeholder='{"localDate":"2026-08-26","timezone":"Asia/Shanghai","items":[]}' /><Button class="secondary-button" variant="outline" type="submit" :disabled="controlCenter.saving || !controlCenter.scheduleDraftJson.trim()">提交完整日程</Button></form>
        <h3>事件与状态覆盖</h3>
        <form class="governance-form" @submit.prevent="controlCenter.createLifeEvent(store.fluctlightId)">
          <label for="event-kind">事件类型<Input id="event-kind" v-model="controlCenter.lifeEvent.kind" maxlength="128" required /></label>
          <div class="form-grid"><label for="event-start">开始时间<Input id="event-start" v-model="controlCenter.lifeEvent.startAt" type="datetime-local" required /></label><label for="event-end">结束时间<Input id="event-end" v-model="controlCenter.lifeEvent.endAt" type="datetime-local" required /></label></div>
          <div class="form-grid"><label for="event-scene">场景（可选）<Input id="event-scene" v-model="controlCenter.lifeEvent.scene" maxlength="512" /></label><label for="event-activity">活动（可选）<Input id="event-activity" v-model="controlCenter.lifeEvent.activity" maxlength="512" /></label></div>
          <label for="event-location">地点（可选）<Input id="event-location" v-model="controlCenter.lifeEvent.location" maxlength="512" /></label>
          <p class="field-note">事件创建需要在下方“证据引用”中填写至少一条可追溯引用。</p>
          <Button class="secondary-button" variant="outline" type="submit" :disabled="controlCenter.saving">创建确认事件</Button>
        </form>
        <ul v-if="(controlCenter.fluctlightDetail.events as unknown[])?.length" class="detail-list"><li v-for="event in controlCenter.fluctlightDetail.events as Array<Record<string, unknown>>" :key="String(event.id)"><strong>{{ formatDisplayValue(event.kind) }}</strong><small>{{ formatZonedRange(event.start_at, event.end_at, governanceTimezone) }} · {{ enumLabel(event.status) }}</small><Button v-if="event.status === 'confirmed'" class="text-button" variant="ghost" type="button" :disabled="controlCenter.saving" @click="controlCenter.cancelLifeEvent(store.fluctlightId, String(event.id))">取消事件</Button></li></ul>
        <form class="governance-form" @submit.prevent="controlCenter.setPresence(store.fluctlightId)"><div class="form-grid"><label for="user-presence">用户状态<Input id="user-presence" v-model="controlCenter.presence.userPresence" maxlength="128" /></label><label for="current-task">当前任务<Input id="current-task" v-model="controlCenter.presence.currentTask" maxlength="512" /></label></div><Button class="secondary-button" variant="outline" type="submit" :disabled="controlCenter.saving">更新状态</Button></form>
      </details>

      <details class="governance-section">
        <summary class="section-heading"><span class="section-index">03</span><div><p class="eyebrow">关系与记忆操作</p><h2>关系与记忆</h2></div><span class="disclosure-icon" aria-hidden="true">⌄</span></summary>
        <p v-if="!(controlCenter.fluctlightDetail.relationships as unknown[])?.length" class="field-note">尚未形成关系状态。</p>
        <ul v-else class="detail-list"><li v-for="relationship in controlCenter.fluctlightDetail.relationships as Array<Record<string, unknown>>" :key="String(relationship.target_actor_id)"><strong>{{ formatDisplayValue(relationship.target_actor_id) }}</strong><small>{{ enumLabel(relationship.trend) }} · {{ labelFor("revision") }} {{ formatDisplayValue(relationship.revision) }}</small><div class="inline-controls"><Input v-model="controlCenter.relationshipRollbackTargets[String(relationship.target_actor_id)]" aria-label="关系回滚目标版本" type="number" min="0" step="1" placeholder="目标版本" /><Button class="text-button" variant="ghost" type="button" :disabled="controlCenter.saving" @click="controlCenter.rollbackRelationship(store.fluctlightId, relationship)">回滚关系</Button></div></li></ul>
        <p v-if="!(controlCenter.fluctlightDetail.memories as unknown[])?.length" class="field-note">暂无可展示的记忆。</p>
        <ul v-else class="detail-list"><li v-for="memory in controlCenter.fluctlightDetail.memories as Array<Record<string, unknown>>" :key="String(memory.id)"><strong>{{ formatDisplayValue(memory.content) }}</strong><div class="inline-controls"><Input v-model="controlCenter.memoryEdits[String(memory.id)]" :aria-label="'修正记忆 ' + memory.id" maxlength="4096" placeholder="修正内容" /><Button class="text-button" variant="ghost" type="button" :disabled="controlCenter.saving" @click="controlCenter.reviseMemory(memory)">修正</Button><Button class="text-button danger-text" variant="ghost" type="button" :disabled="controlCenter.saving" @click="controlCenter.forgetMemory(memory)">遗忘</Button></div></li></ul>
        <label for="governance-evidence">证据引用</label><Input id="governance-evidence" v-model="controlCenter.governanceEvidence" maxlength="4096" placeholder="以逗号分隔，例如 event_123, message_456" />
        <h3>近期认知</h3><p v-if="!(controlCenter.fluctlightDetail.cognition_history as unknown[])?.length" class="field-note">还没有完成的认知行动。</p><ul v-else class="detail-list"><li v-for="action in controlCenter.fluctlightDetail.cognition_history as Array<Record<string, unknown>>" :key="String(action.id)">{{ enumLabel(action.action_type) }}<small>{{ enumLabel(action.status) }}</small></li></ul>
        <h3>近期唤醒</h3><p v-if="!(controlCenter.fluctlightDetail.wake_ups as unknown[])?.length" class="field-note">还没有完成定期唤醒。</p><ul v-else class="detail-list"><li v-for="wakeUp in controlCenter.fluctlightDetail.wake_ups as Array<Record<string, unknown>>" :key="String(wakeUp.id)"><strong>第 {{ formatDisplayValue(wakeUp.cycle) }} 次 · {{ enumLabel(wakeUp.action_type) }}</strong><small>注意：{{ formatDisplayValue(wakeUp.attention) }}</small><small>想法：{{ formatDisplayValue(wakeUp.thought) }}</small><small>愿望：{{ formatDisplayValue(wakeUp.desire) }}</small><small>行动判断：{{ formatDisplayValue(wakeUp.agency) }}</small></li></ul>
        <h3>人格动力与偏好</h3><p v-if="!growthSlots.length" class="field-note">还没有形成可追踪的动力或偏好槽位。</p><ul v-else class="detail-list"><li v-for="slot in growthSlots" :key="String(slot.id)"><strong>{{ formatDisplayValue(slot.label) !== "未设定" ? formatDisplayValue(slot.label) : labelFor(String(slot.key)) }}</strong><small>{{ labelFor("key") }}：{{ labelFor(String(slot.key)) }} · {{ labelFor("value_schema") }}：{{ formatDisplayValue(slot.value_schema) }} · {{ labelFor("revision") }}：{{ formatDisplayValue(slot.revision) }}</small><small>{{ labelFor("value") }}：{{ formatDisplayValue(slot.value) }}</small></li></ul>
      </details>

      <details class="governance-section">
        <summary class="section-heading"><span class="section-index">04</span><div><p class="eyebrow">CAPABILITY REQUESTS</p><h2>能力需求池</h2></div><span class="disclosure-icon" aria-hidden="true">⌄</span></summary>
        <p class="field-note">摇光发现当前没有的能力时，会通过 tool call 提交需求。这里先评估，再手动接入插件；需求本身不会伪装成已执行的动作。</p>
        <ul v-if="controlCenter.capabilityRequests.length" class="detail-list"><li v-for="request in controlCenter.capabilityRequests" :key="String(request.id)"><strong>{{ formatDisplayValue(request.title) }}</strong><small>{{ labelFor("capability_key") }}：{{ formatDisplayValue(request.capabilityKey) }} · {{ labelFor("aggregate_count") }}：{{ formatDisplayValue(request.aggregateCount) }} · {{ capabilityRequestStatus(request.status) }}</small><small>来源摇光：{{ formatDisplayValue(request.fluctlightId) }} · 证据：{{ formatDisplayValue(request.evidenceRefs) }}</small><small>{{ formatDisplayValue(request.description) }}</small><small>提出原因：{{ formatDisplayValue(request.rationale) }}</small><div class="inline-controls"><Button v-if="request.status === 'proposed'" class="text-button" variant="ghost" type="button" :disabled="controlCenter.saving" @click="controlCenter.reviewCapabilityRequest(String(request.id), 'reviewing')">开始评估</Button><Button v-if="request.status === 'reviewing'" class="text-button" variant="ghost" type="button" :disabled="controlCenter.saving" @click="controlCenter.reviewCapabilityRequest(String(request.id), 'accepted')">接受需求</Button><Input v-if="request.status === 'accepted' || request.status === 'fulfilled'" v-model="controlCenter.capabilityRequestVersions[String(request.id)]" aria-label="插件版本" placeholder="插件版本" maxlength="128" /><Button v-if="request.status === 'accepted'" class="text-button" variant="ghost" type="button" :disabled="controlCenter.saving || !controlCenter.capabilityRequestVersions[String(request.id)]?.trim()" @click="controlCenter.reviewCapabilityRequest(String(request.id), 'fulfilled', controlCenter.capabilityRequestVersions[String(request.id)])">标记已接入</Button><Button v-if="request.status === 'proposed' || request.status === 'reviewing'" class="text-button danger-text" variant="ghost" type="button" :disabled="controlCenter.saving" @click="controlCenter.reviewCapabilityRequest(String(request.id), 'rejected')">拒绝</Button></div></li></ul>
      </details>

      <details class="governance-section">
        <summary class="section-heading"><span class="section-index">05</span><div><p class="eyebrow">自治与修订</p><h2>自治与修订</h2></div><span class="disclosure-icon" aria-hidden="true">⌄</span></summary>
        <p v-if="!controlCenter.autonomyActions.length" class="field-note">当前没有待治理的自治动作。</p>
        <ul v-else class="detail-list"><li v-for="action in controlCenter.autonomyActions" :key="action.id"><strong>{{ enumLabel(action.action_type) }}</strong><small>{{ enumLabel(action.status) }}</small><div class="inline-controls"><Button v-if="action.status === 'frozen' || action.status === 'deferred'" class="text-button" variant="ghost" type="button" :disabled="controlCenter.saving || !controlCenter.governanceReason.trim()" @click="controlCenter.governAutonomyAction(action.id, 'paused', store.fluctlightId)">暂停</Button><Button v-if="action.status === 'frozen' || action.status === 'deferred' || action.status === 'paused'" class="text-button" variant="ghost" type="button" :disabled="controlCenter.saving || !controlCenter.governanceReason.trim()" @click="controlCenter.governAutonomyAction(action.id, 'cancelled', store.fluctlightId)">取消</Button></div></li></ul>
        <h3>身份与人格修订记录</h3>
        <p v-if="!(controlCenter.fluctlightDetail.foundation_revisions as unknown[])?.length" class="field-note">还没有修订记录。</p>
        <ul v-else class="detail-list"><li v-for="revision in controlCenter.fluctlightDetail.foundation_revisions as Array<Record<string, unknown>>" :key="String(revision.id)"><strong>版本 {{ formatDisplayValue(revision.revision) }} · {{ enumLabel(revision.source) }}</strong><small>{{ enumLabel(revision.status) }}<template v-if="revision.reason"> · {{ formatDisplayValue(revision.reason) }}</template></small><div v-if="revision.status === 'proposed'" class="inline-controls"><Button class="text-button" variant="ghost" type="button" :disabled="controlCenter.saving || !controlCenter.revisionReason.trim()" @click="controlCenter.acceptFoundationRevision(store.fluctlightId, String(revision.id))">接受</Button><Button class="text-button" variant="ghost" type="button" :disabled="controlCenter.saving || !controlCenter.revisionReason.trim()" @click="controlCenter.rejectFoundationRevision(store.fluctlightId, String(revision.id))">拒绝</Button></div></li></ul>
        <form class="governance-form" @submit.prevent="controlCenter.submitFoundationRevision(store.fluctlightId)"><label for="revision-json">基础修订 JSON<Textarea id="revision-json" v-model="controlCenter.revisionChangesJson" rows="4" placeholder='{"name":"新的名称"}' /></label><label for="revision-reason">修订原因<Input id="revision-reason" v-model="controlCenter.revisionReason" maxlength="1024" /></label><Button class="secondary-button" variant="outline" type="submit" :disabled="controlCenter.saving || !controlCenter.revisionChangesJson.trim() || !controlCenter.revisionReason.trim()">提出修订</Button></form>
        <form class="governance-form" @submit.prevent="controlCenter.rollbackFoundationRevision(store.fluctlightId)"><label for="rollback-revision">回滚目标版本<Input id="rollback-revision" v-model="controlCenter.rollbackTargetRevision" type="number" min="0" step="1" /></label><Button class="secondary-button" variant="outline" type="submit" :disabled="controlCenter.saving || !controlCenter.rollbackTargetRevision || !controlCenter.revisionReason.trim()">回滚到该版本</Button></form>
      </details>

      <section class="danger-zone" aria-labelledby="retirement-title">
        <p class="eyebrow">危险操作</p><h2 id="retirement-title">删除摇光</h2>
        <p>删除会从实例目录中移除并停止活动；历史会话与治理记录会保留用于审计，不能恢复到活动状态。</p>
        <form class="governance-form" @submit.prevent="retire"><label for="retirement-reason">删除原因<Input id="retirement-reason" v-model="retirementReason" maxlength="1024" required /></label><label for="retirement-confirmation">确认名称<Input id="retirement-confirmation" v-model="retirementConfirmation" :aria-label="'输入 ' + store.selectedFluctlightName + ' 确认删除'" maxlength="256" :placeholder="'输入 ' + store.selectedFluctlightName + ' 确认'" required /></label><p class="field-note">需要完整输入“{{ store.selectedFluctlightName }}”后才能启用删除。</p><Button class="danger-button" variant="destructive" type="submit" :disabled="controlCenter.saving || !retirementReason.trim() || retirementConfirmation.trim() !== store.selectedFluctlightName">{{ controlCenter.saving ? "正在删除..." : "删除摇光" }}</Button></form>
      </section>
    </template>
  </section>
</template>
