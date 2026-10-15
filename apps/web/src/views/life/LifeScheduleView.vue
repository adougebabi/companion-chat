<script setup lang="ts">
import { computed } from "vue";

import Button from "@/components/ui/button/Button.vue";
import Input from "@/components/ui/input/Input.vue";
import Textarea from "@/components/ui/textarea/Textarea.vue";
import AsyncState from "../../components/product/AsyncState.vue";
import FeedbackNotice from "../../components/product/FeedbackNotice.vue";
import FormField from "../../components/product/FormField.vue";
import { useFormErrors } from "../../composables/use-form-errors";
import { asRecords, scheduleRead } from "../../lib/life-surface";
import { enumLabel, formatDisplayValue, formatZonedRange, resolveTimezone } from "../../lib/fluctlight-display";
import { userError } from "../../lib/user-feedback";
import { zonedLocalInputToISO } from "../../lib/zoned-local-input";
import { useCharacterSurface } from "./use-character-surface";

const { store, control, detail, readOnlyPreview } = useCharacterSurface();
const { errors, validate, focus } = useFormErrors();
const schedule = computed(() => scheduleRead(detail.value));
const events = computed(() => asRecords(detail.value?.events));
const timezone = computed(() => {
  const identity = detail.value?.identity;
  const identityZone = identity && typeof identity === "object" && !Array.isArray(identity) ? (identity as Record<string, unknown>).timezone : undefined;
  return resolveTimezone(schedule.value.state === "recorded" ? schedule.value.timezone : undefined, typeof identityZone === "string" ? identityZone : undefined);
});

async function submitLifeEvent() {
  if (readOnlyPreview.value || control.saving) return;
  if (!await validate([
    { id: "life-event-kind", label: "事件类型", value: control.lifeEvent.kind, required: true, maxLength: 128 },
    { id: "life-event-start", label: "开始时间", value: control.lifeEvent.startAt, required: true },
    { id: "life-event-end", label: "结束时间", value: control.lifeEvent.endAt, required: true },
    { id: "life-event-evidence", label: "证据引用", value: control.governanceEvidence.split(",").map((value) => value.trim()).filter(Boolean).join(","), required: true },
  ])) return;
  try {
    const start = zonedLocalInputToISO(control.lifeEvent.startAt, timezone.value);
    const end = zonedLocalInputToISO(control.lifeEvent.endAt, timezone.value);
    if (Date.parse(end) <= Date.parse(start)) {
      errors.value["life-event-end"] = "结束时间必须晚于开始时间。";
      await focus("life-event-end");
      return;
    }
  } catch {
    errors.value["life-event-start"] = "时间在当前时区内无效，请重新选择。";
    await focus("life-event-start");
    return;
  }
  await control.createLifeEvent(store.fluctlightId);
}
</script>

<template>
  <section class="page life-page" aria-labelledby="life-schedule-title">
    <header class="pagehead">
      <h1 id="life-schedule-title">日程</h1>
      <span class="badge b-plain">BP-06</span>
      <span v-if="readOnlyPreview" class="badge b-preview">不是业务数据</span>
    </header>
    <p v-if="!store.fluctlightId && !readOnlyPreview" class="statebox">请先选择角色。这里不会显示其他角色的数据。</p>
    <template v-else>
      <FeedbackNotice :message="control.error ? userError(control.error) : ''" />
      <FeedbackNotice :message="control.governanceNotice" tone="success" />
      <AsyncState :loading="!readOnlyPreview && control.loading && !detail" loading-label="正在读取日程…" />
      <p class="small muted">计划来自已接受的本地日程。实际来自事件，两边没有外键时不会猜成同一行。提交和取消沿用当前 revision、生活上下文 revision 和稳定幂等键；冲突时保留草稿。</p>
      <p v-if="schedule.state === 'missing'" class="statebox">日程尚未接入。</p>
      <p v-else-if="schedule.state === 'empty'" class="small muted">当前没有接受的本地日程。</p>
      <div v-else class="card">
        <p class="small muted">{{ schedule.localDate || "日期未记录" }} · {{ schedule.timezone || timezone }} · revision {{ schedule.revision ?? "未记录" }}</p>
        <div class="planfact small muted"><b>时间</b><b>计划</b><b>状态</b></div>
        <div v-for="item in schedule.items" :key="String(item.id ?? item.start_at)" class="planfact">
          <b>{{ formatZonedRange(item.start_at, item.end_at, timezone) }}</b>
          <div>{{ item.activity || "未命名活动" }}<div class="small muted">{{ item.scene || "场景未记录" }}<template v-if="item.location"> · {{ item.location }}</template></div></div>
          <div>{{ enumLabel(item.status) }}</div>
        </div>
        <p v-if="!schedule.items.length" class="small muted">这份日程没有计划项。</p>
        <Button v-if="!readOnlyPreview" variant="outline" type="button" :disabled="control.saving" @click="control.cancelSchedule(store.fluctlightId)">取消当前日程</Button>
      </div>
      <article class="card">
        <h2>实际事件</h2>
        <p v-if="!Object.prototype.hasOwnProperty.call(detail ?? {}, 'events')" class="statebox">事件尚未接入。</p>
        <p v-else-if="!events.length" class="small muted">当前没有事件记录。</p>
        <ul v-else class="detail-list">
          <li v-for="event in events" :key="String(event.id)">
            <strong>{{ formatDisplayValue(event.kind) }}</strong>
            <small>{{ formatZonedRange(event.start_at, event.end_at, timezone) }} · {{ enumLabel(event.status) }} · revision {{ formatDisplayValue(event.revision) }}</small>
            <Button v-if="!readOnlyPreview && event.status === 'confirmed'" variant="ghost" type="button" :disabled="control.saving" @click="control.cancelLifeEvent(store.fluctlightId, String(event.id))">取消事件</Button>
          </li>
        </ul>
      </article>
      <form v-if="!readOnlyPreview" class="card stack" @submit.prevent="control.acceptSchedule(store.fluctlightId)">
        <h2>提交完整日程</h2>
        <p class="small muted">逐条建议日程的接口尚未接入。这里只提交完整本地日 JSON。</p>
        <label for="schedule-draft">完整日程 JSON<Textarea id="schedule-draft" v-model="control.scheduleDraftJson" rows="6" spellcheck="false" placeholder='{"localDate":"2026-10-15","timezone":"Asia/Shanghai","items":[]}' /></label>
        <FormField id="life-schedule-evidence" label="证据引用" hint="至少一条，逗号分隔。失败后草稿和证据都保留。" v-slot="{ controlAttrs }"><Input v-bind="controlAttrs" v-model="control.governanceEvidence" maxlength="4096" placeholder="event_123, message_456" /></FormField>
        <Button variant="outline" type="submit" :disabled="control.saving || !control.scheduleDraftJson.trim()">提交完整日程</Button>
      </form>
      <form v-if="!readOnlyPreview" class="card stack" novalidate @submit.prevent="submitLifeEvent">
        <h2>确认事件</h2>
        <FormField id="life-event-kind" label="事件类型" required v-model:error="errors['life-event-kind']" v-slot="{ controlAttrs }"><Input v-bind="controlAttrs" v-model="control.lifeEvent.kind" maxlength="128" /></FormField>
        <div class="form-grid">
          <FormField id="life-event-start" label="开始时间" required v-model:error="errors['life-event-start']" v-slot="{ controlAttrs }"><Input v-bind="controlAttrs" v-model="control.lifeEvent.startAt" type="datetime-local" /></FormField>
          <FormField id="life-event-end" label="结束时间" required v-model:error="errors['life-event-end']" v-slot="{ controlAttrs }"><Input v-bind="controlAttrs" v-model="control.lifeEvent.endAt" type="datetime-local" /></FormField>
        </div>
        <p class="small muted">时间按 {{ timezone }} 输入。创建使用生活上下文 revision 和幂等键。</p>
        <label for="life-event-scene">场景（可选）<Input id="life-event-scene" v-model="control.lifeEvent.scene" maxlength="512" /></label>
        <label for="life-event-activity">活动（可选）<Input id="life-event-activity" v-model="control.lifeEvent.activity" maxlength="512" /></label>
        <label for="life-event-location">地点（可选）<Input id="life-event-location" v-model="control.lifeEvent.location" maxlength="512" /></label>
        <FormField id="life-event-evidence" label="证据引用" required v-model:error="errors['life-event-evidence']" v-slot="{ controlAttrs }"><Input v-bind="controlAttrs" v-model="control.governanceEvidence" placeholder="event_123, message_456" /></FormField>
        <Button variant="outline" type="submit" :disabled="control.saving">创建确认事件</Button>
      </form>
    </template>
  </section>
</template>
