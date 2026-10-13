<script setup lang="ts">
import { ref, watch } from "vue";
import { ChevronDown, Search, RotateCcw } from "@lucide/vue";
import { Button } from "@/components/ui/button";
const props = defineProps<{ busy?: boolean; label?: string; appliedCount?: number; advancedOpen?: boolean }>();
const emit = defineEmits<{ apply: []; clear: [] }>();
const open = ref(Boolean(props.advancedOpen));
watch(() => props.advancedOpen, value => { if (value) open.value=true; });
</script>
<template>
  <form class="filter-panel" :aria-label="label ?? '筛选条件'" novalidate @submit.prevent="emit('apply')">
    <div class="filter-basic"><slot /></div>
    <details v-if="$slots.advanced" class="product-disclosure filter-advanced" :open="open" @toggle="open=($event.target as HTMLDetailsElement).open"><summary><span>高级条件</span><ChevronDown :size="18" aria-hidden="true" /></summary><div class="filter-basic"><slot name="advanced" /></div></details>
    <div class="form-actions"><Button type="submit" :disabled="busy"><Search :size="16" aria-hidden="true" />{{busy ? '正在查询…' : '应用过滤'}}</Button><Button type="button" variant="outline" :disabled="busy" @click="emit('clear')"><RotateCcw :size="16" aria-hidden="true" />清除</Button><slot name="actions" /></div>
    <p v-if="appliedCount" class="field-hint">已应用 {{appliedCount}} 项筛选条件</p>
  </form>
</template>
