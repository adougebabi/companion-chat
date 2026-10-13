<script setup lang="ts">
import AsyncState from "./AsyncState.vue";
import FeedbackNotice from "./FeedbackNotice.vue";
import { Button } from "@/components/ui/button";
defineProps<{ error?: Error }>();
</script>
<template>
  <section class="page workspace-state">
    <FeedbackNotice v-if="error" message="页面暂时无法打开，请重新加载后重试。" :detail="error.message"><Button as="a" href="" variant="outline">重新加载页面</Button></FeedbackNotice>
    <AsyncState v-else loading loading-label="正在打开工作面…" />
  </section>
</template>
