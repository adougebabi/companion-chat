<script setup>
import { computed, ref } from "vue";
const props = defineProps({ models: { type: Array, default: () => [] } }),
  query = ref("");
const models = computed(() =>
    props.models
      .map((m) => (typeof m === "string" ? m : m.id || m.name))
      .filter(Boolean),
  ),
  filtered = computed(() =>
    models.value.filter((m) =>
      m.toLowerCase().includes(query.value.trim().toLowerCase()),
    ),
  );
</script>
<template>
  <details class="approved-model-directory">
    <summary>可用模型 · {{ models.length }} 项</summary>
    <input
      v-model="query"
      type="search"
      aria-label="搜索可用模型"
      placeholder="搜索模型 ID"
    />
    <div class="chip-options scroll-lock-list" data-scroll-lock>
      <span
        v-for="model in filtered"
        :key="model"
        class="pill"
        :title="model"
        >{{ model }}</span
      ><small v-if="!models.length" class="muted"
        >尚未读取模型目录，可在用途配置中填写模型 ID。</small
      ><small v-else-if="!filtered.length" class="muted"
        >没有匹配的模型。</small
      >
    </div>
  </details>
</template>
