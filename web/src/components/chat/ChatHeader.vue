<script setup lang="ts">
import Avatar from '../Avatar.vue';
import type { PersonaSummary } from '../types';

defineProps<{ persona: PersonaSummary; debugInspector?: boolean }>();
const emit = defineEmits<{ (event: 'back'): void; (event: 'profile'): void; (event: 'tools'): void }>();
</script>

<template>
  <header class="pane-header chat-pane-header">
    <button class="text-icon" type="button" aria-label="返回联系人" title="返回联系人" @click="emit('back')">‹</button>
    <button class="chat-title" type="button" aria-label="查看摇光实例详情" @click="emit('profile')">
      <Avatar :persona="persona" size="small" />
      <span><h1>{{ persona.name }}</h1><p>{{ persona.currentSituation || persona.role || '正在过自己的日常' }}<template v-if="persona.mood"> · {{ persona.mood }}</template></p></span>
    </button>
    <div class="chat-tools"><button class="text-icon" type="button" :aria-label="debugInspector ? '打开检查器' : '打开设置'" :title="debugInspector ? '打开检查器' : '打开设置'" @click="emit('tools')">⋯</button></div>
  </header>
</template>

