<script setup lang="ts">
import Button from "@/components/ui/button/Button.vue";

import LoadingDots from "./LoadingDots.vue";

withDefaults(defineProps<{ busy?: boolean; busyLabel?: string; disabled?: boolean; variant?: "default" | "outline" | "ghost" }>(), {
  busyLabel: "正在提交",
  variant: "default",
});
defineEmits<{ click: [] }>();
</script>
<template>
  <Button class="async-button" :variant="variant" type="button" :disabled="disabled || busy" :aria-busy="busy ? 'true' : undefined" @click="$emit('click')">
    <LoadingDots v-if="busy" :label="busyLabel" />
    <slot v-else />
  </Button>
</template>
