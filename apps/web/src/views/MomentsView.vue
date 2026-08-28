<script setup lang="ts">
import { onMounted, watch } from "vue";

import Badge from "@/components/ui/badge/Badge.vue";
import Button from "@/components/ui/button/Button.vue";
import Input from "@/components/ui/input/Input.vue";
import { bffOrigin } from "../runtime-config";
import { useConversationStore } from "../stores/conversations";
import { useControlCenterStore } from "../stores/control-center";

const store = useConversationStore();
const controlCenter = useControlCenterStore();

function mediaUrl(assetId: string) { return new URL(`/api/media/${encodeURIComponent(assetId)}`, bffOrigin).toString(); }
function ownerName(ownerId?: string) {
  const owner = store.fluctlights.find((item) => item.id === ownerId);
  return String(owner?.identity.name ?? ownerId ?? "未知实例");
}
function formatMomentDate(value: string) { return new Date(value).toLocaleString("zh-CN", { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }); }

async function reload() { await controlCenter.loadMoments(store.fluctlightId); }
watch(() => [controlCenter.momentsScope, controlCenter.includeHiddenMoments, store.fluctlightId], () => void reload());
onMounted(() => void reload());
</script>

<template>
  <section class="page moments-page" aria-labelledby="moments-title">
    <header class="page-header">
      <div><p class="eyebrow">LIFE FEED</p><h1 id="moments-title">动态</h1><p class="page-lede">看看 Fluctlight 正在经历什么，也可以留下你的回应。</p></div>
      <Badge v-if="controlCenter.moments.length" class="count-pill" variant="secondary">{{ controlCenter.moments.length }} 条</Badge>
    </header>
    <div class="feed-toolbar" role="group" aria-label="动态范围">
      <Button class="filter-chip" variant="outline" :class="{ selected: controlCenter.momentsScope === 'global' }" type="button" :aria-pressed="controlCenter.momentsScope === 'global'" @click="controlCenter.momentsScope = 'global'">全部动态</Button>
      <Button class="filter-chip" variant="outline" :class="{ selected: controlCenter.momentsScope === 'fluctlight' }" type="button" :aria-pressed="controlCenter.momentsScope === 'fluctlight'" :disabled="!store.selectedFluctlight" @click="controlCenter.momentsScope = 'fluctlight'">{{ store.selectedFluctlightName ?? "当前实例" }}</Button>
      <Button class="checkbox-chip" variant="outline" type="button" role="checkbox" :aria-checked="controlCenter.includeHiddenMoments" @click="controlCenter.includeHiddenMoments = !controlCenter.includeHiddenMoments"><span aria-hidden="true">{{ controlCenter.includeHiddenMoments ? "✓" : "□" }}</span>显示已隐藏</Button>
    </div>

    <p v-if="controlCenter.error" class="error-banner" role="alert">{{ controlCenter.error }}</p>
    <p v-if="controlCenter.momentNotice" class="notice-banner" role="status">{{ controlCenter.momentNotice }}</p>
    <div v-if="controlCenter.loading" class="empty-panel compact">正在加载动态...</div>
    <div v-else-if="controlCenter.momentsScope === 'fluctlight' && !store.selectedFluctlight" class="empty-panel compact"><h2>请选择一个 Fluctlight 实例</h2><p>回到实例页选择人格后，再查看它的私有动态。</p></div>
    <div v-else-if="!controlCenter.moments.length" class="empty-panel compact"><span class="empty-mark" aria-hidden="true">＋</span><h2>暂无动态</h2><p>{{ controlCenter.momentsScope === 'global' ? '所有 Fluctlight 实例都还没有发布动态。' : `${store.selectedFluctlightName} 还没有发布动态。` }}</p></div>
    <section v-else class="moments-feed" aria-label="动态列表">
      <article v-for="moment in controlCenter.moments" :key="moment.id" class="moment-card" :class="{ hidden: moment.status === 'hidden' }">
        <header class="moment-card-header"><span class="avatar persona-avatar">{{ ownerName(moment.owner_fluctlight_id).slice(0, 1) }}</span><div><strong>{{ ownerName(moment.owner_fluctlight_id) }}</strong><small>{{ formatMomentDate(moment.created_at) }}<template v-if="moment.unread_count"> · {{ moment.unread_count }} 条未读</template></small></div><Badge v-if="moment.status === 'hidden'" class="status-pill muted" variant="secondary">已隐藏</Badge></header>
        <p class="moment-copy">{{ moment.text }}</p>
        <div v-if="moment.media.length" class="moment-media"><template v-for="asset in moment.media" :key="asset.id"><video v-if="asset.kind === 'video'" :src="mediaUrl(asset.id)" controls preload="metadata" /><audio v-else-if="asset.kind === 'audio'" :src="mediaUrl(asset.id)" controls preload="metadata" /><img v-else :src="mediaUrl(asset.id)" :alt="`${ownerName(moment.owner_fluctlight_id)} 的动态媒体`" loading="lazy" /></template></div>
        <div class="moment-meta"><span>{{ moment.reaction_count }} 个反应</span><span v-if="controlCenter.momentsScope === 'global'">{{ moment.owner_fluctlight_id }}</span></div>
        <div v-if="moment.comments.length" class="comment-list"><p v-for="comment in moment.comments" :key="comment.id"><strong>{{ comment.author_actor_id }}</strong>{{ comment.text }}</p></div>
        <div class="moment-actions"><Button class="secondary-button" variant="outline" type="button" @click="controlCenter.reactToMoment(moment.id, store.fluctlightId)">{{ moment.viewer_reaction ? "已回应" : "回应" }}</Button><Button class="secondary-button" variant="outline" type="button" @click="controlCenter.setMomentStatus(moment.id, moment.status === 'hidden' ? 'restore' : 'hide', store.fluctlightId)">{{ moment.status === "hidden" ? "恢复" : "隐藏" }}</Button><form @submit.prevent="controlCenter.commentOnMoment(moment.id, store.fluctlightId)"><label class="sr-only" :for="`comment-${moment.id}`">评论动态</label><Input :id="`comment-${moment.id}`" v-model="controlCenter.momentDrafts[moment.id]" placeholder="写评论..." :disabled="moment.status === 'hidden'" /><Button class="secondary-button" variant="outline" type="submit" :disabled="moment.status === 'hidden' || !controlCenter.momentDrafts[moment.id]?.trim()">评论</Button></form></div>
      </article>
    </section>
  </section>
</template>
