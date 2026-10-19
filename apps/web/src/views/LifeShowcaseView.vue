<script setup lang="ts">
import { computed, provide, reactive, ref } from "vue";

import type { BrowserWardrobeItem } from "@fluctlight/browser-client";
import BottomNav from "../components/layout/BottomNav.vue";
import WorkspaceNav from "../components/layout/WorkspaceNav.vue";
import type { LifeRecord } from "../lib/life-surface";
import GrowthView from "./life/GrowthView.vue";
import LifeGoalsView from "./life/LifeGoalsView.vue";
import LifeNowView from "./life/LifeNowView.vue";
import LifeScheduleView from "./life/LifeScheduleView.vue";
import LifeWardrobeView from "./life/LifeWardrobeView.vue";
import MediaView from "./life/MediaView.vue";
import PeopleView from "./life/PeopleView.vue";
import ProfileView from "./life/ProfileView.vue";
import { lifePreviewKey, type LifePreview } from "./life/use-character-surface";

const pages = ["此刻", "日程", "目标", "衣柜", "人物", "记忆", "资料", "媒体"] as const;
type PageName = (typeof pages)[number];
const scenes = ["正常", "未接入", "空"] as const;
type SceneName = (typeof scenes)[number];
const pageFromQuery = pages.find((item) => item === new URLSearchParams(window.location.search).get("page"));
const sceneFromQuery = scenes.find((item) => item === new URLSearchParams(window.location.search).get("scene"));
const page = ref<PageName>(pageFromQuery ?? "此刻");
const scene = ref<SceneName>(sceneFromQuery ?? "正常");
const drawerOpen = ref(new URLSearchParams(window.location.search).get("drawer") === "1");

const wardrobe: BrowserWardrobeItem[] = [
  { id: "coat", item_kind:"wearable", category: "coat", slot: "outer", description: "旧外套", ownership: "owned", availability: "available", source_kind: "initialization", revision: 1 },
  { id: "scarf", item_kind:"wearable", category: "accessory", slot: "accessory", description: "借来的围巾", ownership: "borrowed", availability: "available", source_kind: "accepted_event", revision: 2 },
  { id: "brush", item_kind:"object", category: "art_supply", slot: "", description: "细头画笔", ownership: "owned", availability: "available", source_kind: "purchase_result", revision: 1 },
];

const recorded: LifeRecord = {
  id: "fixture",
  status: "active",
  current_revision: 3,
  identity: { name: "样本角色", occupation: "书籍编辑", timezone: "Asia/Shanghai", background_story: "样本简介，只用于组件场景。" },
  personality: { openness: 0.4 },
  schedule: { id: "sch", revision: 2, timezone: "Asia/Shanghai", local_date: "2026-10-15", items: [{ id: "item-1", start_at: "2026-10-15T09:00:00Z", end_at: "2026-10-15T18:00:00Z", activity: "整理书架", scene: "公寓", location: "客厅", status: "planned" }] },
  context: { activity: "整理书架", location: "公寓客厅", context_revision: "life-1" },
  inner_state: { mood: { label: "平静" }, momentum: { value: 0.4 } },
  current_state: { appearance: { worn_items: [{ id: "coat", description: "旧外套", slot: "outer", ownership: "owned" }], used_items: [{ id: "brush", description: "细头画笔" }], wearing_state: "known" } },
  goals: [{ id: "goal-1", desired_outcome: "看完一本书", commitments: [{ id: "promise-1", title: "下周去花店" }] }],
  relationships: [{ target_actor_id: "owner-fixture", target_actor_type: "human", profile_id: "shared", trend: "stable", revision: 4, summary: "认识", is_current_user: true }],
  memories: [{ id: "memory-1", content: "海边书店暂停装修", revision: 2 }],
  developing_self: { claims: [{ id: "claim-1", claim: "想把画画坚持下去", revision: 1 }] },
  foundation_revisions: [{ id: "revision-1", revision: 2, source: "reflection", status: "proposed", reason: "她提出更主动分享" }],
  visual_identity: { status: "active", current_revision: 1, timeline: [{ id: "stage-1", stage: "completed", summary: "组件场景里的时间轴，没有图片。" }] },
  events: [{ id: "event-1", kind: "reading", start_at: "2026-10-15T01:00:00Z", end_at: "2026-10-15T01:40:00Z", status: "confirmed", revision: 1 }],
};
const missing: LifeRecord = { id: "fixture", status: "active", identity: { name: "样本角色" }, current_revision: 1 };
const empty: LifeRecord = {
  ...recorded,
  schedule: null,
  goals: [],
  relationships: [],
  memories: [],
  events: [],
  foundation_revisions: [],
  developing_self: { claims: [] },
  current_state: { appearance: { worn_items: [], used_items: [] } },
  inner_state: {},
  context: {},
  visual_identity: { status: "missing", current_revision: 0, timeline: [] },
};

const preview = reactive<LifePreview>({ detail: recorded, wardrobe, prompts: [] });
provide(lifePreviewKey, preview);

const activeView = computed(() => page.value === "人物" ? "people" : page.value === "记忆" ? "growth" : page.value === "资料" ? "profile" : page.value === "媒体" ? "media" : "life");
const lifeSection = computed(() => page.value === "日程" ? "schedule" : page.value === "目标" ? "goals" : page.value === "衣柜" ? "wardrobe" : "now");

function applyScene(next: SceneName) {
  scene.value = next;
  preview.detail = next === "未接入" ? missing : next === "空" ? empty : recorded;
  preview.wardrobe = next === "未接入" ? [] : wardrobe;
}
applyScene(scene.value);
</script>

<template>
  <div class="sage-shell showcase" data-test="life-showcase">
    <header class="topbar">
      <span class="brand"><span class="brand-dot" aria-hidden="true"></span><span class="hide-n">摇光</span></span>
      <span class="charsw"><span class="av" aria-hidden="true">样</span><b>样本角色</b></span>
      <div class="scopebar"><span class="badge b-角色范围">角色范围</span><span class="hide-n">样本角色 · 组件测试</span></div>
      <span class="spacer"></span>
      <span class="badge b-preview">不是业务数据</span>
    </header>
    <div class="layout">
      <aside class="sidenav"><WorkspaceNav :active-view="activeView" :life-section="lifeSection" character-name="样本角色" /></aside>
      <section class="main">
        <div class="chips" role="tablist" aria-label="生活页面">
          <button v-for="item in pages" :key="item" class="chip" :class="{ on: page === item }" type="button" @click="page = item">{{ item }}</button>
        </div>
        <div class="chips" role="tablist" aria-label="数据状态">
          <button v-for="item in scenes" :key="item" class="chip" :class="{ on: scene === item }" type="button" @click="applyScene(item)">{{ item }}</button>
        </div>
        <LifeNowView v-if="page === '此刻'" />
        <LifeScheduleView v-else-if="page === '日程'" />
        <LifeGoalsView v-else-if="page === '目标'" />
        <LifeWardrobeView v-else-if="page === '衣柜'" />
        <PeopleView v-else-if="page === '人物'" />
        <GrowthView v-else-if="page === '记忆'" />
        <ProfileView v-else-if="page === '资料'" />
        <MediaView v-else />
        <button class="btn" type="button" @click="drawerOpen = true">打开抽屉</button>
      </section>
    </div>
    <div v-if="drawerOpen" class="scrim" @click="drawerOpen = false"></div>
    <aside v-if="drawerOpen" class="drawer" aria-label="导航抽屉">
      <header><h2>导航</h2><button class="btn sm" type="button" @click="drawerOpen = false">关闭</button></header>
      <div class="body"><WorkspaceNav :active-view="activeView" :life-section="lifeSection" character-name="样本角色" /></div>
    </aside>
    <BottomNav :active-view="activeView" :life-section="lifeSection" @open-menu="drawerOpen = true" />
  </div>
</template>
