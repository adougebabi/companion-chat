<script setup lang="ts">
import { computed, watch } from "vue";

import AsyncState from "../../components/product/AsyncState.vue";
import FeedbackNotice from "../../components/product/FeedbackNotice.vue";
import { asRecord, asRecords } from "../../lib/life-surface";
import { formatDisplayValue } from "../../lib/fluctlight-display";
import { apiOrigin } from "../../runtime-config";
import { userError } from "../../lib/user-feedback";
import { useCharacterSurface } from "./use-character-surface";

const emit = defineEmits<{ openSettings: [] }>();
const { store, control, detail, preview, readOnlyPreview } = useCharacterSurface();
const visual = computed(() => asRecord(detail.value?.visual_identity));
const timeline = computed(() => asRecords(visual.value?.timeline));
const prompts = computed(() => preview ? preview.prompts : control.diagnosticMediaPrompts.filter((item) => !store.fluctlightId || item.fluctlightId === store.fluctlightId));

watch(() => store.fluctlightId, () => {
  if (!preview) void control.loadDiagnostics();
}, { immediate: true });

function mediaUrl(assetId: string) {
  return new URL(`/api/media/${encodeURIComponent(assetId)}`, apiOrigin).toString();
}
function assetIds(event: Record<string, unknown>): string[] {
  return Array.isArray(event.asset_ids) ? event.asset_ids.filter((value): value is string => typeof value === "string" && value.length > 0) : [];
}
</script>

<template>
  <section class="page life-page" aria-labelledby="media-title">
    <header class="pagehead">
      <h1 id="media-title">媒体与视觉身份</h1>
      <span class="badge b-plain">BP-14</span>
      <span class="badge">生成图·非事实</span>
      <span v-if="readOnlyPreview" class="badge b-preview">不是业务数据</span>
    </header>
    <p class="banner warn">生成图只是画面，不改变穿着、位置或生活事实。角色媒体库列表尚未接入，这里只显示详情里的视觉身份资产和诊断提示词。</p>
    <p v-if="!store.fluctlightId && !readOnlyPreview" class="statebox">请先选择角色。这里不会显示其他角色的数据。</p>
    <template v-else>
      <FeedbackNotice :message="control.error ? userError(control.error) : ''" />
      <AsyncState :loading="!readOnlyPreview && control.loading && !detail" loading-label="正在读取视觉身份…" />
      <article v-if="detail" class="card">
        <h2>视觉身份</h2>
        <p v-if="!visual || !Object.keys(visual).length" class="statebox" data-test="identity-unwired">视觉身份尚未接入。</p>
        <template v-else>
          <p class="small">状态 {{ formatDisplayValue(visual.status) }} · revision {{ formatDisplayValue(visual.current_revision ?? 0) }}</p>
          <p class="small muted">修改角色视觉定义的接口尚未接入。系统设置里的工作流是渲染配置，不是这个角色的身份。</p>
          <div v-if="visual.canonical_asset_id || visual.character_sheet_asset_id" class="outfit">
            <figure v-if="visual.canonical_asset_id"><img :src="mediaUrl(String(visual.canonical_asset_id))" alt="角色参考图"><figcaption class="badge">生成图·非事实</figcaption></figure>
            <figure v-if="visual.character_sheet_asset_id"><img :src="mediaUrl(String(visual.character_sheet_asset_id))" alt="角色设定图"><figcaption class="badge">生成图·非事实</figcaption></figure>
          </div>
          <p v-else class="small muted">当前没有 canonical 参考图。</p>
          <ol v-if="timeline.length">
            <li v-for="event in timeline" :key="String(event.id ?? event.stage)">
              <strong>{{ formatDisplayValue(event.stage) }}</strong>
              <span>{{ formatDisplayValue(event.summary) }}</span>
              <div v-if="assetIds(event).length" class="outfit">
                <img v-for="assetId in assetIds(event)" :key="assetId" :src="mediaUrl(assetId)" alt="视觉身份生成图">
                <span class="badge">生成图·非事实</span>
              </div>
            </li>
          </ol>
          <p v-else class="small muted">还没有视觉身份时间轴。</p>
        </template>
        <button v-if="!readOnlyPreview" class="btn sm" type="button" @click="emit('openSettings')">打开媒体设置</button>
      </article>
      <article class="card">
        <h2>诊断中的媒体提示词</h2>
        <p class="small muted">这些是提示词记录，不是媒体库图片，也不代表发生过。</p>
        <AsyncState :loading="!readOnlyPreview && control.diagnosticsLoading" loading-label="正在读取媒体提示词…" />
        <p v-if="!prompts.length" class="small muted">当前角色没有媒体提示词记录。</p>
        <ul v-else>
          <li v-for="item in prompts" :key="item.id">
            <strong>{{ item.kind === "image" ? "图片生成" : item.kind || "媒体生成" }}</strong>
            <span class="badge">生成图·非事实</span>
            <p>{{ item.providerPrompt || item.submittedPrompt || "没有提示词正文" }}</p>
          </li>
        </ul>
      </article>
    </template>
  </section>
</template>
