<script setup lang="ts">
import { computed, ref } from 'vue';
import type { MediaAsset as MediaAssetData } from '../types';

const props = withDefaults(defineProps<{
  asset: MediaAssetData;
  alt?: string;
  simplified?: boolean;
}>(), { alt: '媒体内容', simplified: false });

const activated = ref(false);
const failed = ref(false);
const isVideo = computed(() => props.asset.kind === 'video');
const ratioStyle = computed(() => {
  const ratio = props.asset.aspectRatio || (props.asset.width && props.asset.height ? props.asset.width / props.asset.height : 16 / 10);
  return { aspectRatio: String(Math.max(0.5, Math.min(2.4, ratio))) };
});

function activate() {
  if (isVideo.value) activated.value = true;
}
</script>

<template>
  <div class="media-box" :class="{ 'media-box--failed': failed, 'media-box--simplified': simplified }" :style="ratioStyle" role="group">
    <template v-if="simplified">
      <span class="media-status">{{ isVideo ? '视频' : '图片' }}已生成（简化模式未加载）</span>
    </template>
    <template v-else-if="failed">
      <span class="media-status" role="status">{{ isVideo ? '视频' : '图片' }}暂时不可用</span>
    </template>
    <template v-else-if="isVideo && !activated">
      <button class="media-activate" type="button" :aria-label="`播放${alt}`" @click="activate">
        <span aria-hidden="true">▶</span>
        <small>点击播放</small>
      </button>
    </template>
    <video
      v-else-if="isVideo"
      class="media-element"
      controls
      preload="none"
      :src="asset.url || undefined"
      :aria-label="alt"
      @error="failed = true"
    >
      视频无法播放
    </video>
    <img
      v-else
      class="media-element"
      loading="lazy"
      decoding="async"
      :src="asset.url || undefined"
      :alt="alt"
      :width="asset.width || undefined"
      :height="asset.height || undefined"
      @error="failed = true"
    >
  </div>
</template>

