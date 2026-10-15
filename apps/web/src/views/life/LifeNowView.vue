<script setup lang="ts">
import { computed } from "vue";

import FeedbackNotice from "../../components/product/FeedbackNotice.vue";
import AsyncState from "../../components/product/AsyncState.vue";
import { moodRead, momentumRead, nowActivity, recordedCommitments, sceneImageUrl, scheduleRead, usedItems, wornItems } from "../../lib/life-surface";
import { wardrobeSlotLabel } from "../../lib/wardrobe-form";
import { formatZonedRange, resolveTimezone } from "../../lib/fluctlight-display";
import { userError } from "../../lib/user-feedback";
import { useCharacterSurface } from "./use-character-surface";

const emit = defineEmits<{ open: [view: "life" | "people" | "growth" | "media" | "chat", section?: string] }>();
const { store, control, detail, readOnlyPreview } = useCharacterSurface();
const activity = computed(() => nowActivity(detail.value));
const scene = computed(() => sceneImageUrl(detail.value));
const mood = computed(() => moodRead(detail.value));
const energy = computed(() => momentumRead(detail.value));
const schedule = computed(() => scheduleRead(detail.value));
const wearing = computed(() => wornItems(detail.value));
const using = computed(() => usedItems(detail.value));
const commitments = computed(() => recordedCommitments(detail.value));
const timezone = computed(() => resolveTimezone(schedule.value.state === "recorded" ? schedule.value.timezone : undefined));
const upcoming = computed(() => {
  if (schedule.value.state !== "recorded") return [];
  const now = Date.now();
  return schedule.value.items.filter((item) => {
    const end = typeof item.end_at === "string" ? Date.parse(item.end_at) : Number.NaN;
    return !Number.isFinite(end) || end >= now;
  }).slice(0, 3);
});
</script>

<template>
  <section class="page life-page" aria-labelledby="life-now-title">
    <header class="pagehead">
      <h1 id="life-now-title">此刻</h1>
      <span class="badge b-plain">BP-06</span>
      <span v-if="readOnlyPreview" class="badge b-preview">不是业务数据</span>
    </header>
    <p v-if="!store.fluctlightId && !readOnlyPreview" class="statebox">请先选择角色。这里不会显示其他角色的数据。</p>
    <template v-else>
      <FeedbackNotice :message="control.error ? userError(control.error) : ''" />
      <AsyncState :loading="!readOnlyPreview && control.loading && !detail" loading-label="正在读取此刻状态…" />
      <section class="hero" data-test="now-hero">
        <img v-if="scene" class="scene" :src="scene" alt="当前场景">
        <div v-else class="hero-missing">场景图尚未接入</div>
        <div class="over">
          <span class="av" aria-hidden="true">{{ (store.selectedFluctlightName || "角").slice(0, 1) }}</span>
          <div>
            <div class="small muted">{{ store.selectedFluctlightName || "当前角色" }} 正在</div>
            <h2 data-test="now-doing">{{ activity.activity || "当前活动尚未接入" }}</h2>
            <p class="small">{{ activity.place || "地点尚未接入" }} · 远程查看，不会移动角色</p>
          </div>
        </div>
      </section>
      <div class="life-grid">
        <article class="card">
          <h3>下一项日程</h3>
          <p v-if="schedule.state === 'missing'" class="statebox">日程尚未接入</p>
          <p v-else-if="schedule.state === 'empty'" class="small muted">当前没有接受的本地日程。</p>
          <ul v-else class="detail-list">
            <li v-for="item in upcoming" :key="String(item.id ?? item.start_at)">
              <strong>{{ String(item.activity ?? "未命名活动") }}</strong>
              <small>{{ formatZonedRange(item.start_at, item.end_at, timezone) }}<template v-if="item.location"> · {{ item.location }}</template></small>
            </li>
          </ul>
          <p v-if="schedule.state === 'recorded' && !upcoming.length" class="small muted">今天没有更晚的计划项。</p>
          <button class="btn sm" type="button" @click="emit('open', 'life', 'schedule')">完整日程</button>
        </article>
        <article class="card" data-test="mood-card">
          <h3>心情 / 精力</h3>
          <p v-if="mood.state === 'missing'">心情尚未接入。不会用初始人格补齐。</p>
          <p v-else>心情：{{ mood.label }}<span v-if="mood.evidence === null" class="small muted"> · 证据字段未接入，不标推测</span><span v-else class="small muted"> · 证据：{{ mood.evidence.join("、") || "无" }}</span></p>
          <p v-if="energy.state === 'missing'">精力尚未接入。</p>
          <p v-else>精力：{{ energy.label }}</p>
        </article>
        <article class="card">
          <h3>当前穿着</h3>
          <p v-if="wearing === null" class="statebox" data-test="wearing-unwired">穿着尚未接入。衣柜部位不是穿着。</p>
          <p v-else-if="!wearing.length" class="small muted">当前没有记录的穿着。</p>
          <ul v-else class="outfit" data-test="now-outfit">
            <li v-for="item in wearing" :key="String(item.id)"><span class="badge">穿着</span> {{ item.description || item.id }} <small class="muted">{{ wardrobeSlotLabel(item.slot) }} · 所有权另计</small></li>
          </ul>
          <h3>正在使用</h3>
          <p v-if="using === null">使用尚未接入。</p>
          <p v-else-if="!using.length" class="small muted">当前没有记录的使用。</p>
          <ul v-else><li v-for="item in using" :key="String(item.id)"><span class="badge">使用</span> {{ item.description || item.id }}</li></ul>
          <button class="btn sm" type="button" @click="emit('open', 'life', 'wardrobe')">打开衣柜</button>
        </article>
        <article class="card">
          <h3>待兑现的约定</h3>
          <p v-if="commitments.state === 'missing'">约定尚未接入。不会从目标标题猜测。</p>
          <p v-else-if="!commitments.items.length" class="small muted">当前没有记录的约定。</p>
          <ul v-else><li v-for="item in commitments.items" :key="String(item.id ?? item.goal_id)"><strong>{{ item.title || item.summary || item.goal_outcome || "未命名约定" }}</strong></li></ul>
          <button class="btn sm" type="button" @click="emit('open', 'life', 'goals')">打开目标</button>
        </article>
        <article class="card">
          <h3>此地附近</h3>
          <p data-test="nearby-unwired">此地附近尚未接入。关系列表不是在场人物。</p>
        </article>
      </div>
    </template>
  </section>
</template>
