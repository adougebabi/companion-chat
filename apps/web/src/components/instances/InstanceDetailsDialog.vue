<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";

import Badge from "@/components/ui/badge/Badge.vue";
import Button from "@/components/ui/button/Button.vue";
import Dialog from "@/components/ui/dialog/Dialog.vue";
import DialogClose from "@/components/ui/dialog/DialogClose.vue";
import DialogContent from "@/components/ui/dialog/DialogContent.vue";
import DialogDescription from "@/components/ui/dialog/DialogDescription.vue";
import DialogFooter from "@/components/ui/dialog/DialogFooter.vue";
import DialogHeader from "@/components/ui/dialog/DialogHeader.vue";
import DialogTitle from "@/components/ui/dialog/DialogTitle.vue";

import { useConversationStore } from "../../stores/conversations";
import { useControlCenterStore } from "../../stores/control-center";
import { enumLabel, formatDisplayValue, formatZonedRange, isCustomLabel, labelFor, resolveTimezone } from "../../lib/fluctlight-display";
import { fluctlightStatusLabel } from "../../lib/fluctlight-status";

type JsonRecord = Record<string, unknown>;

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ close: []; manage: [] }>();
const store = useConversationStore();
const controlCenter = useControlCenterStore();
const dialogOpen = computed(() => props.open && Boolean(store.selectedFluctlight));

function asRecord(value: unknown): JsonRecord {
  return value && typeof value === "object" && !Array.isArray(value) ? value as JsonRecord : {};
}

function asRecords(value: unknown): JsonRecord[] {
  return Array.isArray(value) ? value.filter((item): item is JsonRecord => Boolean(item) && typeof item === "object" && !Array.isArray(item)) : [];
}

const detail = computed(() => asRecord(controlCenter.fluctlightDetail));
const identity = computed(() => {
  const detailedIdentity = asRecord(detail.value.identity);
  return Object.keys(detailedIdentity).length ? detailedIdentity : asRecord(store.selectedFluctlight?.identity);
});
const personality = computed(() => asRecord(detail.value.personality));
const behavioralPolicy = computed(() => asRecord(detail.value.behavioral_policy));
const lifeProfile = computed(() => asRecord(detail.value.life_profile));
const innerState = computed(() => asRecord(detail.value.inner_state));
const context = computed(() => asRecord(detail.value.context));
const pad = computed(() => asRecord(innerState.value.pad));
const mood = computed(() => asRecord(innerState.value.mood));
const momentum = computed(() => asRecord(innerState.value.momentum));
const regulation = computed(() => asRecord(innerState.value.regulation));
const goals = computed(() => asRecords(detail.value.goals));
const intentions = computed(() => asRecords(detail.value.intentions));
const relationships = computed(() => asRecords(detail.value.relationships));
const memories = computed(() => asRecords(detail.value.memories));
const driveSlots = computed(() => asRecords(detail.value.drive_slots));
const preferenceSlots = computed(() => asRecords(detail.value.preference_slots));
const growthSlots = computed(() => {
  const slots = [...driveSlots.value, ...preferenceSlots.value];
  return slots.length ? slots : asRecords(innerState.value.drives);
});
const conflicts = computed(() => asRecords(innerState.value.conflicts));
const schedule = computed(() => asRecord(detail.value.schedule));
const scheduleItems = computed(() => asRecords(schedule.value.items));
const now = ref(Date.now());
let clockTimer: number | undefined;
const scheduleTimezone = computed(() => resolveTimezone(
  typeof schedule.value.timezone === "string" ? schedule.value.timezone : undefined,
  typeof identity.value.timezone === "string" ? identity.value.timezone : undefined,
));

function isCurrentScheduleItem(item: JsonRecord): boolean {
  const start = typeof item.start_at === "string" ? Date.parse(item.start_at) : Number.NaN;
  const end = typeof item.end_at === "string" ? Date.parse(item.end_at) : Number.NaN;
  return Number.isFinite(start) && Number.isFinite(end) && start <= now.value && now.value < end;
}

function metricText(value: unknown): string {
  if (value === null || value === undefined || value === "") return "未设定";
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed.toFixed(2) : "未设定";
}

function metricPercent(value: unknown): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return 0;
  const normalized = parsed < 0 ? (parsed + 1) / 2 : parsed;
  return Math.max(0, Math.min(100, normalized * 100));
}

function slotName(slot: JsonRecord): string {
  const label = formatDisplayValue(slot.label);
  return label === "未设定" ? labelFor(String(slot.key ?? "")) : label;
}

function slotValue(slot: JsonRecord): string {
  if (slot.value !== undefined && slot.value !== null) return formatDisplayValue(slot.value);
  if (slot.pressure !== undefined && slot.pressure !== null) return metricText(slot.pressure);
  return "未设定";
}

const atmosphere = computed(() => {
  for (const candidate of [context.value.atmosphere, context.value.ambience, context.value.environment]) {
    const value = formatDisplayValue(candidate);
    if (value !== "未设定") return value;
  }
  const scene = formatDisplayValue(context.value.scene);
  const activity = formatDisplayValue(context.value.activity);
  if (scene !== "未设定" && activity !== "未设定") return `正在${activity}，位于${scene}。`;
  return "暂无独立氛围描述";
});

const contextPresence = computed(() => asRecord(context.value.presence));

onMounted(() => {
  clockTimer = window.setInterval(() => { now.value = Date.now(); }, 30_000);
});
onBeforeUnmount(() => {
  if (clockTimer !== undefined) window.clearInterval(clockTimer);
});

function close() { emit("close"); }
function onDialogOpenChange(open: boolean) { if (!open && props.open) close(); }
</script>

<template>
  <Dialog :open="dialogOpen" @update:open="onDialogOpenChange">
    <DialogContent v-if="store.selectedFluctlight" class="detail-dialog max-w-none gap-0 p-0 sm:max-w-none" :show-close-button="false" aria-modal="true" aria-labelledby="fluctlight-modal-title">
      <DialogHeader class="detail-dialog-header">
        <div class="modal-identity">
          <span class="modal-avatar" aria-hidden="true">{{ String(store.selectedFluctlightName ?? "F").slice(0, 1) }}</span>
          <div id="fluctlight-modal-title"><p class="eyebrow">摇光实例</p><DialogTitle>{{ store.selectedFluctlightName }}</DialogTitle><DialogDescription class="sr-only">查看当前摇光实例的只读详情。</DialogDescription></div>
        </div>
        <DialogClose as-child><Button variant="ghost" class="icon-button modal-close" type="button" aria-label="关闭摇光详情">×</Button></DialogClose>
      </DialogHeader>

      <div class="detail-dialog-body">
        <div class="detail-status-strip">
          <Badge class="status-pill" variant="secondary" :class="{ paused: store.selectedFluctlight.status === 'paused', muted: store.selectedFluctlight.status === 'retired' }"><i class="legend-dot" :class="store.selectedFluctlight.status === 'paused' ? 'paused' : 'online'" />{{ fluctlightStatusLabel(store.selectedFluctlight.status) }}</Badge>
          <span>{{ controlCenter.fluctlightDetail ? "状态已同步" : "正在读取状态" }}</span>
        </div>

        <div v-if="controlCenter.loading && !controlCenter.fluctlightDetail" class="detail-loading">正在加载摇光详情...</div>
        <template v-else>
          <section class="detail-block">
            <div class="detail-block-heading"><p class="eyebrow">身份与人格</p><h3>身份核心</h3></div>
            <dl v-if="Object.keys(identity).length" class="identity-facts">
              <template v-for="(value, key) in identity" :key="String(key)"><dt>{{ labelFor(String(key)) }}</dt><dd><span>{{ formatDisplayValue(value) }}</span><small v-if="isCustomLabel(String(key))" class="raw-field-name">字段名：{{ String(key) }}</small></dd></template>
            </dl>
            <p v-else class="field-note">尚未形成身份信息。</p>
            <h3>人格特征</h3>
            <dl v-if="Object.keys(personality).length" class="identity-facts">
              <template v-for="(value, key) in personality" :key="String(key)"><dt>{{ labelFor(String(key)) }}</dt><dd><span>{{ formatDisplayValue(value) }}</span><small v-if="isCustomLabel(String(key))" class="raw-field-name">字段名：{{ String(key) }}</small></dd></template>
            </dl>
            <p v-else class="field-note">尚未配置人格特征。</p>
            <h3>表达策略</h3>
            <dl v-if="Object.keys(behavioralPolicy).length" class="identity-facts">
              <template v-for="(value, key) in behavioralPolicy" :key="String(key)"><dt>{{ labelFor(String(key)) }}</dt><dd><span>{{ formatDisplayValue(value) }}</span><small v-if="isCustomLabel(String(key))" class="raw-field-name">字段名：{{ String(key) }}</small></dd></template>
            </dl>
            <p v-else class="field-note">尚未配置表达策略。</p>
          </section>

          <section class="detail-block">
            <div class="detail-block-heading"><p class="eyebrow">此刻</p><h3>当前状态</h3></div>
            <div class="detail-stat-grid">
              <div><span>情绪</span><strong>{{ formatDisplayValue(asRecord(innerState.mood).label) }}</strong></div>
              <div><span>场景</span><strong>{{ formatDisplayValue(context.scene) }}</strong></div>
              <div><span>活动</span><strong>{{ formatDisplayValue(context.activity) }}</strong></div>
            </div>
            <details class="detail-state-drawer">
              <summary>
                <span><strong>状态数值与当前氛围</strong><small>查看 PAD、动力、调节状态和场景描述</small></span>
                <span class="disclosure-icon" aria-hidden="true">⌄</span>
              </summary>
              <div class="detail-state-drawer-body">
                <section class="detail-state-section">
                  <div class="detail-state-heading"><h4>当前情境</h4><span>{{ enumLabel(context.source) }}</span></div>
                  <dl class="detail-context-facts">
                    <div><dt>场景</dt><dd>{{ formatDisplayValue(context.scene) }}</dd></div>
                    <div><dt>活动</dt><dd>{{ formatDisplayValue(context.activity) }}</dd></div>
                    <div><dt>地点</dt><dd>{{ formatDisplayValue(context.location) }}</dd></div>
                    <div class="detail-context-wide"><dt>氛围</dt><dd>{{ atmosphere }}</dd></div>
                    <div v-if="contextPresence.current_task || contextPresence.user_presence" class="detail-context-wide"><dt>在场状态</dt><dd>{{ formatDisplayValue(contextPresence) }}</dd></div>
                  </dl>
                </section>

                <section class="detail-state-section">
                  <div class="detail-state-heading"><h4>情感数值</h4><span>0–1</span></div>
                  <div class="state-metric-grid">
                    <div class="state-metric"><div><span>愉悦度</span><strong>{{ metricText(pad.pleasure) }}</strong></div><div class="state-meter" role="progressbar" aria-label="愉悦度" :aria-valuenow="metricPercent(pad.pleasure)" aria-valuemin="0" aria-valuemax="100"><i :style="{ width: `${metricPercent(pad.pleasure)}%` }" /></div></div>
                    <div class="state-metric"><div><span>唤醒度</span><strong>{{ metricText(pad.arousal) }}</strong></div><div class="state-meter" role="progressbar" aria-label="唤醒度" :aria-valuenow="metricPercent(pad.arousal)" aria-valuemin="0" aria-valuemax="100"><i :style="{ width: `${metricPercent(pad.arousal)}%` }" /></div></div>
                    <div class="state-metric"><div><span>掌控感</span><strong>{{ metricText(pad.dominance) }}</strong></div><div class="state-meter" role="progressbar" aria-label="掌控感" :aria-valuenow="metricPercent(pad.dominance)" aria-valuemin="0" aria-valuemax="100"><i :style="{ width: `${metricPercent(pad.dominance)}%` }" /></div></div>
                  </div>
                  <div class="detail-mini-grid">
                    <div><span>当前情绪</span><strong>{{ formatDisplayValue(mood.label) }}</strong></div>
                    <div><span>情绪强度</span><strong>{{ metricText(mood.intensity) }}</strong></div>
                  </div>
                </section>

                <section class="detail-state-section">
                  <div class="detail-state-heading"><h4>行动与调节</h4><span>当前状态</span></div>
                  <div class="detail-mini-grid">
                    <div><span>行动动能</span><strong>{{ metricText(momentum.value) }}</strong><small>{{ formatDisplayValue(momentum.trend) }}</small></div>
                    <div><span>调节稳定性</span><strong>{{ metricText(regulation.stability) }}</strong><small>压力 {{ metricText(regulation.stress) }}</small></div>
                    <div><span>状态修订</span><strong>{{ formatDisplayValue(innerState.revision) }}</strong><small>最近更新 {{ formatDisplayValue(innerState.last_updated_at) }}</small></div>
                  </div>
                  <p v-if="conflicts.length" class="detail-state-note">内部冲突：{{ formatDisplayValue(conflicts) }}</p>
                </section>

                <section class="detail-state-section">
                  <div class="detail-state-heading"><h4>人格动力与偏好</h4><span>{{ growthSlots.length }} 项</span></div>
                  <p v-if="!growthSlots.length" class="field-note">还没有形成可追踪的动力或偏好。</p>
                  <ul v-else class="state-slot-list">
                    <li v-for="slot in growthSlots" :key="String(slot.id ?? slot.key)"><div><strong>{{ slotName(slot) }}</strong><span>{{ slot.value_schema ? formatDisplayValue(slot.value_schema) : "动力" }}</span></div><p>{{ slotValue(slot) }}</p><small v-if="slot.revision !== undefined">修订 {{ formatDisplayValue(slot.revision) }}</small></li>
                  </ul>
                </section>
              </div>
            </details>
          </section>

          <section class="detail-block">
            <div class="detail-block-heading"><p class="eyebrow">生活世界</p><h3>生活脉络</h3></div>
            <dl v-if="Object.keys(lifeProfile).length" class="identity-facts detail-facts-compact">
              <template v-for="(value, key) in lifeProfile" :key="String(key)"><dt>{{ labelFor(String(key)) }}</dt><dd><span>{{ formatDisplayValue(value) }}</span><small v-if="isCustomLabel(String(key))" class="raw-field-name">字段名：{{ String(key) }}</small></dd></template>
            </dl>
            <div class="detail-summary-grid">
              <div><span>活跃目标</span><strong>{{ goals.length }}</strong></div>
              <div><span>关系</span><strong>{{ relationships.length }}</strong></div>
              <div><span>可展示记忆</span><strong>{{ memories.length }}</strong></div>
            </div>
            <h3>目标与意图</h3>
            <p v-if="!goals.length && !intentions.length" class="field-note">当前没有目标或待执行意图。</p>
            <ul v-else class="modal-detail-list">
              <li v-for="goal in goals" :key="String(goal.id)"><strong>{{ formatDisplayValue(goal.description) }}</strong><small>{{ enumLabel(goal.status) }} · {{ formatDisplayValue(goal.progress) }}</small></li>
              <li v-for="intention in intentions" :key="String(intention.id)"><strong>{{ formatDisplayValue(intention.action) }}</strong><small>{{ enumLabel(intention.status) }}</small></li>
            </ul>
            <h3>当前上下文</h3>
            <p class="field-note">{{ enumLabel(context.source) }}<template v-if="context.location"> · {{ formatDisplayValue(context.location) }}</template></p>
            <h3>今日日程 <small class="timezone-note">{{ scheduleTimezone }}</small></h3>
            <p v-if="!Object.keys(schedule).length || !scheduleItems.length" class="field-note">日程待生成，当前没有接受的本地日计划。</p>
            <ol v-else class="timeline-list detail-timeline">
                <li v-for="item in scheduleItems" :key="String(item.id)" :class="{ active: isCurrentScheduleItem(item) }" :aria-current="isCurrentScheduleItem(item) ? 'time' : undefined">
                  <time :datetime="String(item.start_at ?? '')">{{ formatZonedRange(item.start_at, item.end_at, scheduleTimezone) }}</time>
                <div><strong>{{ formatDisplayValue(item.activity) }}<span v-if="isCurrentScheduleItem(item)" class="timeline-now-badge">进行中</span></strong><span>{{ formatDisplayValue(item.scene) }}<template v-if="item.status"> · {{ enumLabel(item.status) }}</template></span></div>
              </li>
            </ol>
          </section>

          <section class="detail-block">
            <div class="detail-block-heading"><p class="eyebrow">关系与记忆</p><h3>关系状态</h3></div>
            <p v-if="!relationships.length" class="field-note">尚未形成关系状态。</p>
            <ul v-else class="modal-detail-list"><li v-for="relationship in relationships" :key="String(relationship.target_actor_id)"><strong>{{ formatDisplayValue(relationship.target_actor_id) }}</strong><small>{{ enumLabel(relationship.trend) }}<template v-if="relationship.summary"> · {{ formatDisplayValue(relationship.summary) }}</template></small></li></ul>
            <h3>可展示记忆</h3>
            <p v-if="!memories.length" class="field-note">暂无可展示的记忆。</p>
            <ul v-else class="modal-detail-list"><li v-for="memory in memories" :key="String(memory.id)"><strong>{{ formatDisplayValue(memory.content) }}</strong><small>{{ enumLabel(memory.type) }} · {{ enumLabel(memory.status) }}</small></li></ul>
          </section>
        </template>
      </div>

      <DialogFooter class="detail-dialog-footer m-0">
        <Button class="secondary-button" variant="outline" type="button" @click="emit('manage')">进入编辑与治理</Button>
        <DialogClose as-child><Button class="primary-button" type="button">完成</Button></DialogClose>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
