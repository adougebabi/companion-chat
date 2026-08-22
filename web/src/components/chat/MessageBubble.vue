<script setup lang="ts">
import { computed } from 'vue';
import MediaGallery from '../media/MediaGallery.vue';
import type { Message } from '../types';

const props = defineProps<{ message: Message; simplifiedMedia?: boolean }>();
const isUser = computed(() => props.message.role === 'user');
const isTyping = computed(() => props.message.transient === 'typing');
const messageText = computed(() => props.message.text || '');
const hasAttachments = computed(() => Boolean(props.message.attachments?.length));
const timeText = computed(() => {
  if (!props.message.createdAt) return '';
  const date = new Date(props.message.createdAt);
  if (Number.isNaN(date.getTime())) return '';
  return new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(date);
});
</script>

<template>
  <article class="message" :data-message-id="message.id" :class="isUser ? 'outgoing' : 'incoming'" :aria-live="isTyping ? 'polite' : undefined">
    <div class="bubble">
      <span v-if="isTyping && !messageText" class="pending-text">输入中…</span>
      <div v-else-if="messageText" class="message-copy">{{ messageText }}</div>
      <MediaGallery v-if="hasAttachments" :assets="message.attachments" :simplified="simplifiedMedia" />
      <div v-if="message.generation && !hasAttachments" class="message-media skeleton" :class="{ failed: message.generation.status === 'failed' }" role="status">
        <span v-if="message.generation.status === 'failed'">{{ message.generation.kind === 'video' ? '视频' : '图片' }}暂时不可用</span>
        <span v-else>{{ message.generation.kind === 'video' ? '视频' : '图片' }}生成中</span>
        <small v-if="message.generation.request">{{ message.generation.request }}</small>
      </div>
      <span v-if="!messageText && !hasAttachments && !message.generation && !isTyping" class="pending-text">暂无内容</span>
      <time v-if="timeText" :datetime="message.createdAt || undefined">{{ timeText }}</time>
    </div>
  </article>
</template>
