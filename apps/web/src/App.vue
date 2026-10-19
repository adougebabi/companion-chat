<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue';
import { mountProduct } from './v3/proto/app';
const host = ref<HTMLElement | null>(null);
let active = true;
let dispose: undefined | (() => void);
onMounted(async () => {
  const cleanup = await mountProduct(host.value);
  if (active) dispose = cleanup;
  else cleanup?.();
});
onBeforeUnmount(() => { active = false; dispose?.(); });
</script>
<template><div id="v3-root" ref="host" /></template>
