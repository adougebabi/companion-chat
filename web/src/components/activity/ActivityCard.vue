<script setup lang="ts">
import { computed } from 'vue';
import Avatar from '../Avatar.vue';
import MediaGallery from '../media/MediaGallery.vue';
import type { ActivityItem, PersonaSummary } from '../types';

const props = withDefaults(defineProps<{
  activity: ActivityItem;
  persona?: PersonaSummary | null;
  commenting?: boolean;
  simplifiedMedia?: boolean;
}>(), { persona: null, commenting: false, simplifiedMedia: false });
const emit = defineEmits<{
  (event: 'open-persona', id: string): void;
  (event: 'like', id: string): void;
  (event: 'hide', id: string): void;
  (event: 'comment', id: string): void;
  (event: 'cancel-comment'): void;
  (event: 'submit-comment', id: string, content: string): void;
  (event: 'chat', id: string): void;
}>();

const commentText = computed(() => props.activity.comments || []);
const owner = computed(() => props.persona || props.activity.persona || { id: props.activity.personaId || '', name: '摇光实例' });
const timeText = computed(() => {
  if (!props.activity.createdAt) return '';
  const date = new Date(props.activity.createdAt);
  return Number.isNaN(date.getTime()) ? '' : new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(date);
});

function submitComment(event: Event): void {
  const form = event.currentTarget as HTMLFormElement;
  emit('submit-comment', props.activity.id, new FormData(form).get('content')?.toString() || '');
}
</script>

<template>
  <article class="activity-card">
    <header>
      <button class="activity-author" type="button" :aria-label="`查看 ${owner.name} 的资料`" @click="emit('open-persona', owner.id)">
        <Avatar :persona="owner" size="small" />
        <span><b>{{ owner.name }}</b><small>{{ timeText }}<template v-if="owner.currentSituation || owner.role"> · {{ owner.currentSituation || owner.role }}</template></small></span>
      </button>
      <button class="more-button" type="button" aria-label="隐藏动态" title="隐藏动态" @click="emit('hide', activity.id)">···</button>
    </header>
    <p class="activity-body">{{ activity.content }}</p>
    <MediaGallery v-if="activity.mediaMode !== 'none' && activity.media?.length" :assets="activity.media" :simplified="simplifiedMedia" :alt-prefix="`${owner.name} 分享的媒体`" />
    <div v-else-if="activity.mediaMode && activity.mediaMode !== 'none'" class="activity-media skeleton" :class="{ failed: activity.mediaStatus === 'failed' }" role="status">
      {{ activity.mediaStatus === 'failed' ? '图片生成暂不可用' : '图片生成中' }}
    </div>
    <footer>
      <button class="reaction" :class="{ liked: activity.liked }" type="button" :aria-pressed="activity.liked ? 'true' : 'false'" :aria-label="activity.liked ? '取消赞这条动态' : '赞这条动态'" @click="emit('like', activity.id)">♡ <span>{{ activity.liked ? '已赞' : '赞' }}</span></button>
      <button type="button" @click="emit('comment', activity.id)">评论</button>
      <button type="button" @click="emit('chat', owner.id)">私聊</button>
    </footer>
    <div v-if="commentText.length" class="comment-list" aria-label="评论">
      <p v-for="comment in commentText" :key="comment.id || `${comment.authorName}-${comment.createdAt}`"><b>{{ comment.authorName || '你' }}</b><span>{{ comment.content }}</span></p>
    </div>
    <form v-if="commenting" class="comment-form" @submit.prevent="submitComment">
      <label><span class="sr-only">评论内容</span><input name="content" maxlength="500" placeholder="写下你的评论" aria-label="评论内容" /></label>
      <button type="submit" aria-label="发布评论">↑</button>
      <button class="quiet" type="button" aria-label="取消评论" @click="emit('cancel-comment')">取消</button>
    </form>
  </article>
</template>
