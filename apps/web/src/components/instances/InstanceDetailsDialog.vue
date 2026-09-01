<script setup lang="ts">
import { computed } from "vue";

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
const goals = computed(() => asRecords(detail.value.goals));
const intentions = computed(() => asRecords(detail.value.intentions));
const relationships = computed(() => asRecords(detail.value.relationships));
const memories = computed(() => asRecords(detail.value.memories));
const schedule = computed(() => asRecord(detail.value.schedule));
const scheduleItems = computed(() => asRecords(schedule.value.items));
const scheduleTimezone = computed(() => resolveTimezone(
  typeof schedule.value.timezone === "string" ? schedule.value.timezone : undefined,
  typeof identity.value.timezone === "string" ? identity.value.timezone : undefined,
));

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
              <li v-for="item in scheduleItems" :key="String(item.id)">
                <time :datetime="String(item.start_at ?? '')">{{ formatZonedRange(item.start_at, item.end_at, scheduleTimezone) }}</time>
                <div><strong>{{ formatDisplayValue(item.activity) }}</strong><span>{{ formatDisplayValue(item.scene) }}<template v-if="item.status"> · {{ enumLabel(item.status) }}</template></span></div>
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
