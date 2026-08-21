<script setup lang="ts">
withDefaults(defineProps<{ message?: string | null; tone?: 'error' | 'info' | 'success' }>(), { message: null, tone: 'info' });
const emit = defineEmits<{ (event: 'dismiss'): void }>();
</script>

<template>
  <p v-if="message" class="feedback-banner" :class="`feedback-banner--${tone}`" role="alert">
    <span>{{ message }}</span><button type="button" aria-label="关闭提示" @click="emit('dismiss')">×</button>
  </p>
</template>

