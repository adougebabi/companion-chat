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
import { fluctlightStatusLabel } from "../../lib/fluctlight-status";

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ close: []; manage: [] }>();
const store = useConversationStore();
const controlCenter = useControlCenterStore();
const dialogOpen = computed(() => props.open && Boolean(store.selectedFluctlight));

const displayLabels: Record<string, string> = {
  id: "标识", name: "名称", age: "年龄", gender: "性别", occupation: "职业", residence: "居住地",
  timezone: "时区", birthday: "生日", background: "背景", biography: "经历", notes: "备注",
};
function labelFor(key: string) { return displayLabels[key] ?? key; }
function valueText(value: unknown) { return Array.isArray(value) ? value.join("、") : String(value ?? "未设定"); }

function close() { emit("close"); }
function onDialogOpenChange(open: boolean) { if (!open && props.open) close(); }
</script>

<template>
  <Dialog :open="dialogOpen" @update:open="onDialogOpenChange">
    <DialogContent v-if="store.selectedFluctlight" class="detail-dialog max-w-none gap-0 p-0 sm:max-w-none" :show-close-button="false" aria-modal="true" aria-labelledby="fluctlight-modal-title">
      <DialogHeader class="detail-dialog-header">
          <div class="modal-identity">
            <span class="modal-avatar" aria-hidden="true">{{ String(store.selectedFluctlightName ?? "F").slice(0, 1) }}</span>
            <div id="fluctlight-modal-title"><p class="eyebrow">FLUCTLIGHT</p><DialogTitle>{{ store.selectedFluctlightName }}</DialogTitle><DialogDescription class="sr-only">查看当前 Fluctlight 的只读详情。</DialogDescription></div>
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
              <div class="detail-block-heading"><p class="eyebrow">IDENTITY</p><h3>基本身份信息</h3></div>
              <dl class="identity-facts">
                <template v-for="(value, key) in store.selectedFluctlight.identity" :key="String(key)">
                  <dt>{{ labelFor(String(key)) }}</dt><dd>{{ valueText(value) }}</dd>
                </template>
              </dl>
            </section>

            <section v-if="controlCenter.fluctlightDetail" class="detail-block">
              <div class="detail-block-heading"><p class="eyebrow">PRESENT</p><h3>此刻</h3></div>
              <div class="detail-stat-grid">
                <div><span>情绪</span><strong>{{ String(((controlCenter.fluctlightDetail.inner_state as Record<string, any>).mood?.label) ?? "未形成") }}</strong></div>
                <div><span>场景</span><strong>{{ String((controlCenter.fluctlightDetail.context as Record<string, any>)?.scene ?? "待确认") }}</strong></div>
                <div><span>活动</span><strong>{{ String((controlCenter.fluctlightDetail.context as Record<string, any>)?.activity ?? "待规划") }}</strong></div>
              </div>
            </section>

            <section v-if="controlCenter.fluctlightDetail" class="detail-block">
              <div class="detail-block-heading"><p class="eyebrow">LIFE</p><h3>生活脉络</h3></div>
              <div class="detail-summary-grid">
                <div><span>活跃目标</span><strong>{{ (controlCenter.fluctlightDetail.goals as unknown[])?.length ?? 0 }}</strong></div>
                <div><span>关系</span><strong>{{ (controlCenter.fluctlightDetail.relationships as unknown[])?.length ?? 0 }}</strong></div>
                <div><span>可展示记忆</span><strong>{{ (controlCenter.fluctlightDetail.memories as unknown[])?.length ?? 0 }}</strong></div>
              </div>
              <details v-if="(controlCenter.fluctlightDetail.goals as unknown[])?.length" class="detail-disclosure"><summary>查看当前目标</summary><ul class="modal-detail-list"><li v-for="goal in controlCenter.fluctlightDetail.goals as Array<Record<string, unknown>>" :key="String(goal.id)">{{ String(goal.description ?? "未命名目标") }}<small>{{ String(goal.status ?? "") }} · {{ String(goal.progress ?? "") }}</small></li></ul></details>
              <details v-if="(controlCenter.fluctlightDetail.relationships as unknown[])?.length" class="detail-disclosure"><summary>查看关系</summary><ul class="modal-detail-list"><li v-for="relationship in controlCenter.fluctlightDetail.relationships as Array<Record<string, unknown>>" :key="String(relationship.target_actor_id)">{{ String(relationship.target_actor_id) }}<small>{{ String(relationship.trend ?? "") }}</small></li></ul></details>
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
