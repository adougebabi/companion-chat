<script setup lang="ts">
import { AlertCircle, CheckCircle2, Info } from "@lucide/vue";
withDefaults(defineProps<{ message?: string; title?: string; tone?: "error" | "success" | "info"; detail?: string }>(), { tone: "error" });
</script>
<template>
  <div v-if="message || title" class="feedback-notice" :class="`feedback-${tone}`" :role="tone === 'error' ? 'alert' : 'status'">
    <component :is="tone === 'error' ? AlertCircle : tone === 'success' ? CheckCircle2 : Info" :size="18" aria-hidden="true" />
    <div class="feedback-content">
      <strong v-if="title">{{ title }}</strong><p v-if="message">{{ message }}</p>
      <div v-if="$slots.default" class="feedback-actions"><slot /></div>
      <details v-if="detail" class="technical-detail"><summary>技术详情</summary><pre>{{ detail }}</pre></details>
    </div>
  </div>
</template>
