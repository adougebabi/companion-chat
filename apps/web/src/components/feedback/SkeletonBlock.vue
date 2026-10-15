<script setup lang="ts">
withDefaults(defineProps<{ lines?: number }>(), { lines: 3 });
</script>
<template>
  <div class="skeleton-block" aria-hidden="true">
    <i v-for="line in lines" :key="line" :style="{ width: line === lines ? '62%' : '100%' }" />
  </div>
</template>
