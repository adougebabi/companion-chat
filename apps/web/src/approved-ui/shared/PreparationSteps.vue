<script setup>
import { ActionButton } from "./primitives.mjs";
defineProps({ stages: { type: Array, required: true }, busy: Boolean });
defineEmits(["recover"]);
const labels = {
  ready: "已完成",
  running: "进行中",
  pending: "等待中",
  failed: "失败",
  blocked: "等待前序恢复",
  unknown: "状态待确认",
};
</script>
<template>
  <ol class="preparation-list">
    <li
      v-for="(stage, index) in stages"
      :key="stage.key"
      :data-testid="'prep-step-' + stage.key"
      :data-state="labels[stage.state] || '状态待确认'"
    >
      <span class="preparation-number">{{
        stage.state === "ready" ? "✓" : index + 1
      }}</span>
      <div class="preparation-copy">
        <div class="row">
          <b>{{ stage.title }}</b
          ><span class="spacer" /><span
            :class="[
              'preparation-state',
              {
                'is-failed': stage.state === 'failed',
                'is-current': stage.state === 'running',
              },
            ]"
            role="status"
            >{{ labels[stage.state] || "状态待确认" }}</span
          >
        </div>
        <p :class="stage.state === 'failed' ? 'inline-error' : 'muted'">
          {{ stage.note }}
        </p>
        <div class="row">
          <a
            v-if="stage.state === 'failed' && stage.configSection"
            class="text-button"
            :href="
              '#/settings?section=' + encodeURIComponent(stage.configSection)
            "
            >去配置中心 ↗</a
          ><ActionButton
            v-if="stage.retryable"
            type="button"
            class="button"
            :disabled="busy"
            @click="$emit('recover', stage)"
            >{{ busy ? "处理中…" : "恢复此环节" }}</ActionButton
          ><a v-if="stage.href" class="text-button" :href="stage.href"
            >查看原任务与结果 ↗</a
          >
        </div>
      </div>
    </li>
  </ol>
</template>
