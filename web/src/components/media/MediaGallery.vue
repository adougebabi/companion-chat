<script setup lang="ts">
import MediaAsset from './MediaAsset.vue';
import type { MediaAsset as MediaAssetData } from '../types';

withDefaults(defineProps<{
  assets?: MediaAssetData[] | null;
  simplified?: boolean;
  altPrefix?: string;
}>(), { assets: null, simplified: false, altPrefix: '生成的媒体' });
</script>

<template>
  <div v-if="assets?.length" class="media-gallery" aria-label="附件">
    <MediaAsset v-for="(asset, index) in assets" :key="asset.id || `${asset.url || 'asset'}-${index}`" :asset="asset" :simplified="simplified" :alt="`${altPrefix} ${index + 1}`" />
  </div>
</template>

