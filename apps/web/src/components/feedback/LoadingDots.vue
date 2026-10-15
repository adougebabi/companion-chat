<script setup lang="ts">
withDefaults(defineProps<{ label?: string }>(), { label: "正在等待" });
</script>
<template>
  <span class="loading-dots" role="status">{{ label }}<i aria-hidden="true" /><i aria-hidden="true" /><i aria-hidden="true" /></span>
</template>
