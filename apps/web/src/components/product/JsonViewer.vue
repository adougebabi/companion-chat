<script setup lang="ts">
import { computed, ref } from "vue";
import { ChevronDown, Copy, Check } from "@lucide/vue";
import { Button } from "@/components/ui/button";
const props = withDefaults(defineProps<{ value: unknown; label?: string; open?: boolean }>(), { label: "原始数据" });
const copied = ref(false), copyError = ref(false);
const text = computed(() => { try { return typeof props.value === "string" ? props.value : JSON.stringify(props.value, null, 2) ?? "未提供"; } catch { return "无法展示该字段"; } });
async function copy() {
  copied.value = false; copyError.value = false;
  try { await navigator.clipboard.writeText(text.value); copied.value = true; }
  catch { copyError.value = true; }
}
</script>
<template>
  <details class="json-viewer" :open="open">
    <summary><ChevronDown :size="16" aria-hidden="true" /><span>{{ label }}</span></summary>
    <div class="json-viewer-toolbar"><Button variant="ghost" size="sm" type="button" @click="copy"><component :is="copied ? Check : Copy" :size="15" aria-hidden="true" />{{ copied ? '已复制' : '复制' }}</Button><span v-if="copyError" role="status">无法复制，请选取文本。</span></div>
    <pre tabindex="0" :aria-label="label">{{ text }}</pre>
  </details>
</template>
