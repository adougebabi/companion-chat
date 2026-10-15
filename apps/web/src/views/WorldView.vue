<script setup lang="ts">
import { computed } from "vue";

import { asRecords } from "../lib/life-surface";
import { enumLabel, formatDisplayValue, formatZonedRange, resolveTimezone } from "../lib/fluctlight-display";
import { useCharacterSurface } from "./life/use-character-surface";

const { store, control, detail, readOnlyPreview } = useCharacterSurface();
const events = computed(() => asRecords(detail.value?.events));
const places = ["地图", "地点", "场景", "NPC 关系图", "实例事件模板开关"] as const;
const timezone = computed(() => {
  const identity = detail.value?.identity;
  const zone = identity && typeof identity === "object" && !Array.isArray(identity) ? (identity as Record<string, unknown>).timezone : undefined;
  return resolveTimezone(undefined, typeof zone === "string" ? zone : undefined);
});
const eventsKnown = computed(() => Boolean(detail.value && Object.prototype.hasOwnProperty.call(detail.value, "events")));
</script>

<template>
  <section class="page life-page" aria-labelledby="world-title">
    <header class="pagehead">
      <h1 id="world-title">世界</h1>
      <span class="badge b-plain">BP-10</span>
      <span v-if="readOnlyPreview" class="badge b-preview">不是业务数据</span>
    </header>
    <p class="banner info">浏览世界不会移动角色，也不会改写所在地点。</p>
    <p v-if="!store.fluctlightId && !readOnlyPreview" class="statebox">请先选择角色。这里不会显示其他角色的事件。</p>
    <template v-else>
      <article class="card">
        <h2>本实例已记录事件</h2>
        <p v-if="!detail && control.loading" class="small muted">正在读取事件…</p>
        <p v-else-if="!eventsKnown" class="statebox">本实例事件记录尚未接入。</p>
        <p v-else-if="!events.length" class="small muted">当前没有已记录事件。</p>
        <ul v-else class="detail-list">
          <li v-for="event in events" :key="String(event.id ?? event.kind)">
            <strong>{{ formatDisplayValue(event.kind) }}</strong>
            <small>{{ formatZonedRange(event.start_at, event.end_at, timezone) }} · {{ enumLabel(event.status) }}</small>
          </li>
        </ul>
        <p class="small muted">世界页只读这些记录。取消和新建仍在日程页，避免把浏览当成移动。</p>
      </article>
      <div class="life-grid">
        <article v-for="place in places" :key="place" class="card">
          <h2>{{ place }}</h2>
          <p class="statebox">{{ place }}尚未接入。打开这里不会改变角色位置。</p>
        </article>
      </div>
    </template>
  </section>
</template>
