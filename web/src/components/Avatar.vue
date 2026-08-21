<script setup lang="ts">
import { computed } from 'vue';
import type { PersonaSummary } from './types';

const props = withDefaults(defineProps<{
  persona?: Partial<PersonaSummary> | null;
  size?: 'small' | 'medium' | 'large';
}>(), { persona: null, size: 'medium' });

const initials = computed(() => Array.from(props.persona?.name || '?').slice(0, 1).join(''));
const avatarStyle = computed(() => ({ '--avatar': props.persona?.color || '#76bce9' }));
</script>

<template>
  <span class="avatar" :class="`avatar--${size}`" :style="avatarStyle" aria-hidden="true">{{ initials }}</span>
</template>

