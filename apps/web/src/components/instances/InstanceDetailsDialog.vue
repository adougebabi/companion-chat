<script setup lang="ts">
import GoalPlanningPanel from "./GoalPlanningPanel.vue";
import GoalPanel from "./GoalPanel.vue";
import { X, ChevronDown } from "@lucide/vue";
import JsonViewer from "../product/JsonViewer.vue";
import AsyncState from "../product/AsyncState.vue";
import FeedbackNotice from "../product/FeedbackNotice.vue";
import { summaryFields, humanValue } from "../../lib/profile-presentation";
import { shortId, userError } from "../../lib/user-feedback";
import { wardrobeCategoryLabel, wardrobeSlotLabel } from "../../lib/wardrobe-form";
import { actorUserBackgroundFields } from "../../lib/actor-user-background";
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { BrowserClient, type BrowserWardrobeItem } from "@fluctlight/browser-client";

import Badge from "@/components/ui/badge/Badge.vue";
import Button from "@/components/ui/button/Button.vue";
import Dialog from "@/components/ui/dialog/Dialog.vue";
import DialogClose from "@/components/ui/dialog/DialogClose.vue";
import DialogContent from "@/components/ui/dialog/DialogContent.vue";
import DialogDescription from "@/components/ui/dialog/DialogDescription.vue";
import DialogFooter from "@/components/ui/dialog/DialogFooter.vue";
import DialogHeader from "@/components/ui/dialog/DialogHeader.vue";
import DialogTitle from "@/components/ui/dialog/DialogTitle.vue";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

import { useConversationStore } from "../../stores/conversations";
import { useControlCenterStore } from "../../stores/control-center";
import { enumLabel, formatDisplayValue, formatTimelineTime, formatZonedRange, isCustomLabel, labelFor, resolveTimezone } from "../../lib/fluctlight-display";
import { fluctlightStatusLabel } from "../../lib/fluctlight-status";
import { apiOrigin } from "../../runtime-config";

type JsonRecord = Record<string, unknown>;

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ close: []; manage: [] }>();
const store = useConversationStore();
const controlCenter = useControlCenterStore();
const wardrobeClient = new BrowserClient(apiOrigin);
const dialogOpen = computed(() => props.open && Boolean(store.selectedFluctlight));

function mediaUrl(assetId: string): string {
  return new URL(`/api/media/${encodeURIComponent(assetId)}`, apiOrigin).toString();
}

function asRecord(value: unknown): JsonRecord {
  return value && typeof value === "object" && !Array.isArray(value) ? value as JsonRecord : {};
}

function asRecords(value: unknown): JsonRecord[] {
  return Array.isArray(value) ? value.filter((item): item is JsonRecord => Boolean(item) && typeof item === "object" && !Array.isArray(item)) : [];
}

function relationshipKey(value: JsonRecord): string {
  return `${String(value.target_actor_id ?? "")}::${String(value.profile_id ?? "shared")}`;
}

function jsonDisplay(value: unknown): string {
  if (value === undefined) return "未提供";
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return "无法展示该字段";
  }
}

const hasCurrentFluctlightDetail = computed(() => Boolean(controlCenter.fluctlightDetail) && controlCenter.fluctlightDetailFluctlightId === store.selectedFluctlight?.id);
const detail = computed(() => hasCurrentFluctlightDetail.value ? asRecord(controlCenter.fluctlightDetail) : {});
const corePersona = computed(() => asRecord(detail.value.core_persona));
const developingSelf = computed(() => asRecord(detail.value.developing_self));
const actorUserBackground = computed(() => asRecord(asRecord(detail.value.actor_user).background));
const currentState = computed(() => asRecord(detail.value.current_state));
const wornItems = computed(() => asRecords(asRecord(currentState.value.appearance).worn_items));
const wardrobeItems = ref<BrowserWardrobeItem[]>([]);
const wardrobeFluctlightId = ref("");
const wardrobeNextCursor = ref("");
const wardrobeHasMore = ref(false);
const wardrobeInventoryComplete = ref(false);
const wardrobeLoading = ref(false);
const wardrobeError = ref("");
const wardrobeLoaded = ref(false);
const wardrobeExpanded = ref(false);
let wardrobeRequestId = 0;

function resetWardrobe(fluctlightId = "") {
  wardrobeRequestId += 1;
  wardrobeFluctlightId.value = fluctlightId;
  wardrobeItems.value = [];
  wardrobeNextCursor.value = "";
  wardrobeHasMore.value = false;
  wardrobeInventoryComplete.value = false;
  wardrobeLoading.value = false;
  wardrobeError.value = "";
  wardrobeLoaded.value = false;
}

async function loadWardrobePage() {
  const fluctlightId = store.selectedFluctlight?.id;
  if (!fluctlightId || wardrobeLoading.value) return;
  if (wardrobeFluctlightId.value !== fluctlightId) resetWardrobe(fluctlightId);
  const cursor = wardrobeLoaded.value ? wardrobeNextCursor.value : "";
  const requestId = ++wardrobeRequestId;
  wardrobeLoading.value = true;
  wardrobeError.value = "";
  try {
    const page = await wardrobeClient.wardrobe(fluctlightId, cursor);
    if (requestId !== wardrobeRequestId || store.selectedFluctlight?.id !== fluctlightId) return;
    const items = Array.isArray(page?.items) ? page.items : [];
    wardrobeItems.value = cursor ? [...wardrobeItems.value, ...items] : items;
    wardrobeNextCursor.value = typeof page?.next_cursor === "string" ? page.next_cursor : "";
    wardrobeHasMore.value = Boolean(page?.has_more);
    wardrobeInventoryComplete.value = Boolean(page?.inventory_complete);
    wardrobeLoaded.value = true;
  } catch {
    if (requestId === wardrobeRequestId) wardrobeError.value = "衣柜读取失败，请重试。";
  } finally {
    if (requestId === wardrobeRequestId) wardrobeLoading.value = false;
  }
}

function onWardrobeToggle(event: Event) {
  if (!(event.target instanceof HTMLDetailsElement)) return;
  wardrobeExpanded.value = event.target.open;
  if (!event.target.open) return;
  const fluctlightId = store.selectedFluctlight?.id ?? "";
  if (wardrobeFluctlightId.value !== fluctlightId) resetWardrobe(fluctlightId);
  if (!wardrobeLoaded.value) void loadWardrobePage();
}

watch(() => props.open, (open) => { if (!open) { wardrobeExpanded.value = false; resetWardrobe(); } });
watch(() => store.selectedFluctlight?.id, () => {
  resetWardrobe();
  if (props.open && wardrobeExpanded.value) void loadWardrobePage();
});
const identity = computed(() => {
  const detailedIdentity = asRecord(detail.value.identity);
  return Object.keys(detailedIdentity).length ? detailedIdentity : asRecord(store.selectedFluctlight?.identity);
});
const personality = computed(() => asRecord(detail.value.personality));
const behavioralPolicy = computed(() => asRecord(detail.value.behavioral_policy));
const lifeProfile = computed(() => asRecord(detail.value.life_profile));
const primaryIdentity = computed(()=>summaryFields(identity.value,['name','age','gender','occupation','birthplace','location','timezone','background_story','background','core_values','worldview','birthday']));
const primaryPersonality = computed(()=>summaryFields(personality.value));
const primaryPolicy = computed(()=>summaryFields(behavioralPolicy.value));
const primaryLife = computed(()=>summaryFields(lifeProfile.value));
const appearance = computed(()=>asRecord(lifeProfile.value.appearance));
const appearanceFeatures = computed(()=>summaryFields(asRecord(appearance.value.physical_features)));
const visibleUserFields = computed(()=>actorUserBackgroundFields.filter(field=>actorUserBackground.value[field.key]!=null && actorUserBackground.value[field.key]!==''));
function actorName(id:unknown,current=false){const text=String(id??'');if(current || text===store.authenticatedActorId)return '你';return String(store.fluctlights.find(item=>item.id===text)?.identity.name ?? shortId(text));}
const innerState = computed(() => asRecord(detail.value.inner_state));
const context = computed(() => asRecord(detail.value.context));
const pad = computed(() => asRecord(innerState.value.pad));
const mood = computed(() => asRecord(innerState.value.mood));
const momentum = computed(() => asRecord(innerState.value.momentum));
const regulation = computed(() => asRecord(innerState.value.regulation));
const goals = computed(() => asRecords(detail.value.goals));
const intentions = computed(() => asRecords(detail.value.intentions));
const relationships = computed(() => asRecords(detail.value.relationships));
const memories = computed(() => asRecords(detail.value.memories));
const driveSlots = computed(() => asRecords(detail.value.drive_slots));
const preferenceSlots = computed(() => asRecords(detail.value.preference_slots));
const growthSlots = computed(() => {
  const slots = [...driveSlots.value, ...preferenceSlots.value];
  return slots.length ? slots : asRecords(innerState.value.drives);
});
const conflicts = computed(() => asRecords(innerState.value.conflicts));
const schedule = computed(() => asRecord(detail.value.schedule));
const scheduleItems = computed(() => asRecords(schedule.value.items));
const now = ref(Date.now());
let clockTimer: number | undefined;
let visualIdentityTimer: number | undefined;
const scheduleTimezone = computed(() => resolveTimezone(
  typeof schedule.value.timezone === "string" ? schedule.value.timezone : undefined,
  typeof identity.value.timezone === "string" ? identity.value.timezone : undefined,
));

function isCurrentScheduleItem(item: JsonRecord): boolean {
  const start = typeof item.start_at === "string" ? Date.parse(item.start_at) : Number.NaN;
  const end = typeof item.end_at === "string" ? Date.parse(item.end_at) : Number.NaN;
  return Number.isFinite(start) && Number.isFinite(end) && start <= now.value && now.value < end;
}

function metricText(value: unknown): string {
  if (value === null || value === undefined || value === "") return "未设定";
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed.toFixed(2) : "未设定";
}

function metricPercent(value: unknown): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return 0;
  const normalized = parsed < 0 ? (parsed + 1) / 2 : parsed;
  return Math.max(0, Math.min(100, normalized * 100));
}

function slotName(slot: JsonRecord): string {
  const label = formatDisplayValue(slot.label);
  return label === "未设定" ? labelFor(String(slot.key ?? "")) : label;
}

function slotValue(slot: JsonRecord): string {
  if (slot.value !== undefined && slot.value !== null) return formatDisplayValue(slot.value);
  if (slot.pressure !== undefined && slot.pressure !== null) return metricText(slot.pressure);
  return "未设定";
}

const atmosphere = computed(() => {
  for (const candidate of [context.value.atmosphere, context.value.ambience, context.value.environment]) {
    const value = formatDisplayValue(candidate);
    if (value !== "未设定") return value;
  }
  const scene = formatDisplayValue(context.value.scene);
  const activity = formatDisplayValue(context.value.activity);
  if (scene !== "未设定" && activity !== "未设定") return `正在${activity}，位于${scene}。`;
  return "暂无独立氛围描述";
});

const contextPresence = computed(() => asRecord(context.value.presence));
const initializationSource = computed(() => asRecord(detail.value.initialization_source));
const hasInitializationSource = computed(() => Object.keys(initializationSource.value).length > 0);
const initializationFoundationRevision = computed(() => asRecords(detail.value.foundation_revisions).find((revision) => revision.source === "initialization" && revision.status === "accepted") ?? {});
const visualIdentity = computed(() => asRecord(detail.value.visual_identity));
const visualIdentityStageRanks: Record<string, number> = {
  session_created: 10, seed_requested: 20, seed_ready: 30, image_requested: 40, image_ready: 50,
  vision_requested: 60, vision_ready: 70, patch_requested: 80, patch_ready: 90, regenerate: 100,
  accepted: 110, character_sheet_requested: 120, character_sheet_ready: 130, completed: 140, failed: 150,
};
const visualIdentityTimeline = computed(() => asRecords(visualIdentity.value.timeline).slice().sort((left, right) => {
  const leftTime = typeof left.occurred_at === "string" ? Date.parse(left.occurred_at) : Number.NaN;
  const rightTime = typeof right.occurred_at === "string" ? Date.parse(right.occurred_at) : Number.NaN;
  if (Number.isFinite(leftTime) && Number.isFinite(rightTime) && leftTime !== rightTime) return leftTime - rightTime;
  const leftOrder = Number(left.stage_order);
  const rightOrder = Number(right.stage_order);
  if (Number.isFinite(leftOrder) && Number.isFinite(rightOrder) && leftOrder !== rightOrder) return leftOrder - rightOrder;
  const leftRank = visualIdentityStageRanks[String(left.stage ?? "")] ?? 1000;
  const rightRank = visualIdentityStageRanks[String(right.stage ?? "")] ?? 1000;
  if (leftRank !== rightRank) return leftRank - rightRank;
  return String(left.id ?? left.stage ?? "").localeCompare(String(right.id ?? right.stage ?? ""));
}));
const visualIdentityActiveEventIndex = computed(() => {
  for (let index = visualIdentityTimeline.value.length - 1; index >= 0; index -= 1) {
    const status = String(visualIdentityTimeline.value[index].status ?? "");
    const stage = String(visualIdentityTimeline.value[index].stage ?? "");
    if (status === "running" || status === "queued" || stage === "regenerate") return index;
  }
  return -1;
});
function visualIdentityEventStatus(event: JsonRecord, index: number): string {
  const status = String(event.status ?? "");
  // A requested/running event is a historical start marker once any later
  // stage exists. Showing it as still running makes a completed seed look
  // stuck even though seed_ready/image_requested are already present.
  if ((status === "running" || status === "queued") && index < visualIdentityTimeline.value.length - 1) return "completed";
  return status;
}
const visualIdentityConstraints = computed(() => asRecord(visualIdentity.value.renderer_constraints));

function visualIdentityAssetIds(event: JsonRecord): string[] {
  return Array.isArray(event.asset_ids) ? event.asset_ids.filter((value): value is string => typeof value === "string" && value.length > 0) : [];
}

function visualIdentityStatusLabel(status: unknown): string {
  const labels: Record<string, string> = {
    missing: "尚未创建", queued: "已排队", running: "进行中", awaiting_review: "等待复核", active: "已建立", failed: "失败", renderer_config_pending: "等待渲染配置",
    completed: "完成", rejected_not_self: "不是自己", regenerating: "再次生成", image_queued: "等待图片", image_completed: "图片完成", character_sheet_queued: "等待角色卡",
  };
  return labels[String(status ?? "")] ?? String(status ?? "未提供");
}

function visualIdentityStageLabel(stage: unknown): string {
  const labels: Record<string, string> = {
    session_created: "初始化", seed_requested: "生成角色设计图提示词", seed_ready: "角色设计图提示词完成", image_requested: "生成角色设计图", image_ready: "候选角色设计图完成", vision_requested: "视觉理解中", vision_ready: "视觉理解完成", patch_requested: "身份评审中", patch_ready: "身份补丁完成", regenerate: "再次生成", accepted: "成为 canonical", character_sheet_requested: "生成 character sheet", character_sheet_ready: "character sheet 完成", completed: "工作流完成", failed: "工作流失败",
  };
  return labels[String(stage ?? "")] ?? String(stage ?? "阶段");
}

function rendererConstraintText(value: unknown): string {
  return value === "not_applicable" ? "不适用" : formatDisplayValue(value);
}

const visualIdentityPollingStatuses = new Set([
  "queued", "running", "awaiting_review", "regenerating",
  "image_queued", "image_completed", "character_sheet_queued"
]);

onMounted(() => {
  clockTimer = window.setInterval(() => { now.value = Date.now(); }, 30_000);
  visualIdentityTimer = window.setInterval(() => {
    const fluctlightId = store.selectedFluctlight?.id;
    const visualStatus = String(asRecord(detail.value.visual_identity).status ?? "");
    if (props.open && document.visibilityState === "visible" && !store.sending && fluctlightId && visualIdentityPollingStatuses.has(visualStatus)) {
      void controlCenter.loadFluctlightDetail(fluctlightId);
    }
  }, 5_000);
});
onBeforeUnmount(() => {
  if (clockTimer !== undefined) window.clearInterval(clockTimer);
  if (visualIdentityTimer !== undefined) window.clearInterval(visualIdentityTimer);
});

function close() { emit("close"); }
function onDialogOpenChange(open: boolean) { if (!open && props.open) close(); }
</script>

<template>
  <Dialog :open="dialogOpen" @update:open="onDialogOpenChange">
    <DialogContent v-if="store.selectedFluctlight" class="detail-dialog max-w-none gap-0 p-0 sm:max-w-none" :show-close-button="false" aria-modal="true" aria-labelledby="fluctlight-modal-title">
      <DialogHeader class="detail-dialog-header">
        <div class="modal-identity">
          <span class="modal-avatar" aria-hidden="true">{{ String(store.selectedFluctlightName ?? "F").slice(0, 1) }}</span>
          <div id="fluctlight-modal-title"><p class="eyebrow">摇光实例</p><DialogTitle>{{ store.selectedFluctlightName }}</DialogTitle><DialogDescription class="sr-only">查看当前摇光实例的只读详情。</DialogDescription></div>
        </div>
        <DialogClose as-child><Button variant="ghost" class="icon-button modal-close" type="button" aria-label="关闭摇光详情"><X :size="20" aria-hidden="true" /></Button></DialogClose>
      </DialogHeader>

      <div class="detail-dialog-body">
        <div class="detail-status-strip">
          <Badge class="status-pill" variant="secondary" :class="{ paused: store.selectedFluctlight.status === 'paused', muted: store.selectedFluctlight.status === 'retired' }"><i class="legend-dot" :class="store.selectedFluctlight.status === 'paused' ? 'paused' : 'online'" />{{ fluctlightStatusLabel(store.selectedFluctlight.status) }}</Badge>
		  <span>{{ hasCurrentFluctlightDetail ? "状态已同步" : "正在读取状态" }}</span>
        </div>

		<AsyncState v-if="controlCenter.loading && !hasCurrentFluctlightDetail" loading loading-label="正在加载摇光详情…" />
        <FeedbackNotice v-else-if="controlCenter.error && !hasCurrentFluctlightDetail" :message="userError(controlCenter.error)"><Button variant="outline" type="button" @click="controlCenter.loadFluctlightDetail(store.selectedFluctlight.id)">重新读取</Button></FeedbackNotice>
        <Tabs v-else default-value="identity" class="detail-tabs w-full">
          <TabsList class="detail-tabs-list grid grid-cols-4 w-full mb-3">
            <TabsTrigger value="identity">基础资料</TabsTrigger>
            <TabsTrigger value="state">此刻状态</TabsTrigger>
            <TabsTrigger value="world">生活脉络</TabsTrigger>
            <TabsTrigger value="memory">关系记忆</TabsTrigger>
          </TabsList>

          <TabsContent value="identity" class="detail-tab-pane mt-0 space-y-4">
            <section class="detail-block">
              <div class="detail-block-heading"><p class="eyebrow">身份与人格</p><h3>身份核心</h3></div>
              <dl v-if="Object.keys(primaryIdentity).length" class="identity-facts">
                <template v-for="(value, key) in primaryIdentity" :key="String(key)"><dt>{{ labelFor(String(key)) }}</dt><dd><span>{{ humanValue(value) }}</span><small v-if="isCustomLabel(String(key))" class="raw-field-name">字段名：{{ String(key) }}</small></dd></template>
              </dl>
              <p v-else class="field-note">尚未形成身份信息。</p>
              <JsonViewer :value="identity" label="完整身份属性（含未设定及扩展字段）" />
              <h3>人格特征</h3>
              <dl v-if="Object.keys(primaryPersonality).length" class="identity-facts">
                <template v-for="(value, key) in primaryPersonality" :key="String(key)"><dt>{{ labelFor(String(key)) }}</dt><dd><span>{{ humanValue(value) }}</span><small v-if="isCustomLabel(String(key))" class="raw-field-name">字段名：{{ String(key) }}</small></dd></template>
              </dl>
              <p v-else class="field-note">尚未配置人格特征。</p>
              <JsonViewer :value="personality" label="人格原始属性与更新策略" />
              <h3>表达策略</h3>
              <dl v-if="Object.keys(primaryPolicy).length" class="identity-facts">
                <template v-for="(value, key) in primaryPolicy" :key="String(key)"><dt>{{ labelFor(String(key)) }}</dt><dd><span>{{ humanValue(value) }}</span><small v-if="isCustomLabel(String(key))" class="raw-field-name">字段名：{{ String(key) }}</small></dd></template>
              </dl>
              <p v-else class="field-note">尚未配置表达策略。</p>
              <JsonViewer :value="behavioralPolicy" label="完整表达策略" />
            </section>

            <section class="detail-block visual-identity-block">
              <div class="detail-block-heading"><p class="eyebrow">VISUAL IDENTITY</p><h3>视觉身份工作流</h3></div>
              <div class="detail-status-strip visual-identity-status">
                <Badge class="status-pill" variant="secondary">{{ visualIdentityStatusLabel(visualIdentity.status) }}</Badge>
                <span>Revision {{ formatDisplayValue(visualIdentity.current_revision ?? 0) }}</span>
                <span v-if="visualIdentity.active_session_id">Session {{ String(visualIdentity.active_session_id).slice(0, 12) }}</span>
              </div>
              <JsonViewer v-if="Object.keys(visualIdentityConstraints).length" :value="visualIdentityConstraints" label="渲染参数（高级）" />
              <p v-if="!visualIdentityTimeline.length" class="field-note">初始化后，这里会显示“生成 → 视觉理解 → 评审 → 再生成/完成”的时间轴。</p>
              <ol v-else class="timeline-list visual-identity-timeline" aria-label="视觉身份处理时间轴">
                <li v-for="(event, index) in visualIdentityTimeline" :key="`${String(event.id ?? event.stage)}-${String(event.occurred_at ?? '')}`" :class="{ active: index === visualIdentityActiveEventIndex }">
                  <time :datetime="String(event.occurred_at ?? '')">{{ formatTimelineTime(event.occurred_at, scheduleTimezone) }}</time>
                  <div class="visual-identity-event-content"><strong>{{ visualIdentityStageLabel(event.stage) }}<span class="timeline-now-badge">{{ visualIdentityStatusLabel(visualIdentityEventStatus(event, index)) }}</span></strong><span>{{ formatDisplayValue(event.summary) }}</span>
                    <div v-if="visualIdentityAssetIds(event).length" class="visual-identity-assets"><img v-for="assetId in visualIdentityAssetIds(event)" :key="assetId" :src="mediaUrl(assetId)" :alt="`${visualIdentityStageLabel(event.stage)}图片`" loading="lazy" /></div>
                  </div>
                </li>
              </ol>
              <div v-if="visualIdentity.canonical_asset_id || visualIdentity.character_sheet_asset_id" class="visual-identity-canonical">
                <div v-if="visualIdentity.canonical_asset_id"><span>角色参考图</span><img :src="mediaUrl(String(visualIdentity.canonical_asset_id))" alt="角色参考图" loading="lazy" /></div>
                <div v-if="visualIdentity.character_sheet_asset_id"><span>角色设定图</span><img :src="mediaUrl(String(visualIdentity.character_sheet_asset_id))" alt="角色设定图" loading="lazy" /></div>
              </div>
            </section>

            <section class="detail-block">
              <div class="detail-block-heading"><p class="eyebrow">ABOUT YOU</p><h3>用户背景</h3></div>
              <p class="field-note">关于你本人的已确认资料。可在“进入编辑与治理”中设置；未提供的信息保持未知。</p>
              <p v-if="!visibleUserFields.length" class="field-note">尚未提供背景资料，可在编辑与治理中补充。</p>
              <dl v-else class="detail-list"><div v-for="field in visibleUserFields" :key="field.key"><dt>{{ field.label }}</dt><dd>{{ actorUserBackground[field.key] == null ? '未确认' : field.key === 'meeting_confirmed' ? (actorUserBackground[field.key] ? '已约定' : '未约定') : formatDisplayValue(actorUserBackground[field.key]) }}</dd></div></dl>
            </section>

            <section class="detail-block">
              <details class="detail-state-drawer">
                <summary>
                  <span><strong>分层 Persona（系统架构）</strong><small>查看 Core Persona、Developing Self 与当前快照结构</small></span>
                  <ChevronDown class="disclosure-icon" :size="18" aria-hidden="true" />
                </summary>
                <div class="detail-state-drawer-body">
                  <div class="persona-json-grid">
                    <JsonViewer :value="corePersona" label="Core Persona · 受保护" />
                    <JsonViewer :value="developingSelf" label="Developing Self · 证据化" />
                    <JsonViewer :value="currentState" label="Current State · 当前快照" />
                  </div>
                </div>
              </details>
            </section>

            <section v-if="hasInitializationSource" class="detail-block initialization-source-block">
              <details class="detail-state-drawer initialization-source-drawer">
                <summary>
                  <span><strong>初始化来源（仅所有者）</strong><small>查看原始角色卡、人格分类、提取覆盖与 Foundation 关联</small></span>
                  <ChevronDown class="disclosure-icon" :size="18" aria-hidden="true" />
                </summary>
                <div class="detail-state-drawer-body">
                  <section class="detail-state-section">
                    <div class="detail-state-heading"><h4>分析与模型</h4><span>{{ formatDisplayValue(initializationSource.classification) }}</span></div>
                    <dl class="detail-context-facts">
                      <div><dt>分析 ID</dt><dd>{{ formatDisplayValue(initializationSource.id) }}</dd></div>
                      <div><dt>诊断关联</dt><dd>{{ formatDisplayValue(initializationSource.correlation_id) }}</dd></div>
                      <div><dt>Provider</dt><dd>{{ formatDisplayValue(initializationSource.provider_endpoint_id) }}</dd></div>
                      <div><dt>模型</dt><dd>{{ formatDisplayValue(initializationSource.model_id) }}</dd></div>
                      <div><dt>Prompt 版本</dt><dd>{{ formatDisplayValue(initializationSource.prompt_version) }}</dd></div>
                      <div><dt>Schema 版本</dt><dd>{{ formatDisplayValue(initializationSource.schema_version) }}</dd></div>
                      <div><dt>分析时间</dt><dd>{{ formatDisplayValue(initializationSource.created_at) }}</dd></div>
                      <div><dt>Foundation 关联时间</dt><dd>{{ formatDisplayValue(initializationSource.linked_at) }}</dd></div>
                      <div><dt>Foundation 修订</dt><dd>{{ formatDisplayValue(initializationFoundationRevision.id) }}</dd></div>
                      <div><dt>Foundation revision</dt><dd>{{ formatDisplayValue(initializationFoundationRevision.revision) }}</dd></div>
                      <div class="detail-context-wide"><dt>来源摘要</dt><dd>{{ formatDisplayValue(initializationSource.source_digest) }}</dd></div>
                      <div class="detail-context-wide"><dt>投影摘要</dt><dd>{{ formatDisplayValue(initializationSource.projection_digest) }}</dd></div>
                    </dl>
                  </section>
                  <section class="detail-state-section">
                    <div class="detail-state-heading"><h4>人格分类与提取覆盖</h4><span>语义审计</span></div>
                    <div class="persona-json-grid">
                      <div class="persona-json-card"><div class="detail-state-heading"><h4>分类证据</h4><span>{{ formatDisplayValue(initializationSource.classification) }}</span></div><pre>{{ jsonDisplay(initializationSource.classification_evidence) }}</pre></div>
                      <div class="persona-json-card"><div class="detail-state-heading"><h4>提取覆盖</h4><span>Coverage</span></div><pre>{{ jsonDisplay(initializationSource.coverage) }}</pre></div>
                      <div class="persona-json-card"><div class="detail-state-heading"><h4>字段推导依据</h4><span>Derivations</span></div><pre>{{ jsonDisplay(initializationSource.field_derivations) }}</pre></div>
                    </div>
                  </section>
                  <section class="detail-state-section">
                    <div class="detail-state-heading"><h4>结构化投影与 Foundation</h4><span>激活前分析快照</span></div>
                    <p class="field-note">该只读投影保留模型分析结果；关联时间标记它已绑定到创建时接受的 Foundation 初始修订，后续治理修订不会改写这份来源。</p>
                    <pre>{{ jsonDisplay(initializationSource.structured_projection) }}</pre>
                  </section>
                  <section class="detail-state-section">
                    <div class="detail-state-heading"><h4>原始角色卡</h4><span>Owner-only</span></div>
                    <pre>{{ String(initializationSource.source_text ?? "") }}</pre>
                  </section>
                </div>
              </details>
            </section>
          </TabsContent>

          <TabsContent value="state" class="detail-tab-pane mt-0 space-y-4">
            <section class="detail-block">
              <div class="detail-block-heading"><p class="eyebrow">此刻</p><h3>当前状态</h3></div>
              <div class="detail-stat-grid">
                <div><span>情绪</span><strong>{{ formatDisplayValue(asRecord(innerState.mood).label) }}</strong></div>
                <div><span>场景</span><strong>{{ formatDisplayValue(context.scene) }}</strong></div>
                <div><span>活动</span><strong>{{ formatDisplayValue(context.activity) }}</strong></div>
              </div>
              <details class="detail-state-drawer">
                <summary>
                  <span><strong>状态数值与当前氛围</strong><small>查看 PAD、动力、调节状态和场景描述</small></span>
                  <ChevronDown class="disclosure-icon" :size="18" aria-hidden="true" />
                </summary>
                <div class="detail-state-drawer-body">
                  <section class="detail-state-section">
                    <div class="detail-state-heading"><h4>当前情境</h4><span>{{ enumLabel(context.source) }}</span></div>
                    <dl class="detail-context-facts">
                      <div><dt>场景</dt><dd>{{ formatDisplayValue(context.scene) }}</dd></div>
                      <div><dt>活动</dt><dd>{{ formatDisplayValue(context.activity) }}</dd></div>
                      <div><dt>地点</dt><dd>{{ formatDisplayValue(context.location) }}</dd></div>
                      <div class="detail-context-wide"><dt>氛围</dt><dd>{{ atmosphere }}</dd></div>
                      <div v-if="contextPresence.current_task || contextPresence.user_presence" class="detail-context-wide"><dt>在场状态</dt><dd>{{ formatDisplayValue(contextPresence) }}</dd></div>
                    </dl>
                  </section>

                  <section class="detail-state-section">
                    <div class="detail-state-heading"><h4>情感数值</h4><span>0–1</span></div>
                    <div class="state-metric-grid">
                      <div class="state-metric"><div><span>愉悦度</span><strong>{{ metricText(pad.pleasure) }}</strong></div><div class="state-meter" role="progressbar" aria-label="愉悦度" :aria-valuenow="metricPercent(pad.pleasure)" aria-valuemin="0" aria-valuemax="100"><i :style="{ width: `${metricPercent(pad.pleasure)}%` }" /></div></div>
                      <div class="state-metric"><div><span>唤醒度</span><strong>{{ metricText(pad.arousal) }}</strong></div><div class="state-meter" role="progressbar" aria-label="唤醒度" :aria-valuenow="metricPercent(pad.arousal)" aria-valuemin="0" aria-valuemax="100"><i :style="{ width: `${metricPercent(pad.arousal)}%` }" /></div></div>
                      <div class="state-metric"><div><span>掌控感</span><strong>{{ metricText(pad.dominance) }}</strong></div><div class="state-meter" role="progressbar" aria-label="掌控感" :aria-valuenow="metricPercent(pad.dominance)" aria-valuemin="0" aria-valuemax="100"><i :style="{ width: `${metricPercent(pad.dominance)}%` }" /></div></div>
                    </div>
                    <div class="detail-mini-grid">
                      <div><span>当前情绪</span><strong>{{ formatDisplayValue(mood.label) }}</strong></div>
                      <div><span>情绪强度</span><strong>{{ metricText(mood.intensity) }}</strong></div>
                    </div>
                  </section>

                  <section class="detail-state-section">
                    <div class="detail-state-heading"><h4>行动与调节</h4><span>当前状态</span></div>
                    <div class="detail-mini-grid">
                      <div><span>行动动能</span><strong>{{ metricText(momentum.value) }}</strong><small>{{ formatDisplayValue(momentum.trend) }}</small></div>
                      <div><span>调节稳定性</span><strong>{{ metricText(regulation.stability) }}</strong><small>压力 {{ metricText(regulation.stress) }}</small></div>
                      <div><span>状态修订</span><strong>{{ formatDisplayValue(innerState.revision) }}</strong><small>最近更新 {{ formatDisplayValue(innerState.last_updated_at) }}</small></div>
                    </div>
                    <p v-if="conflicts.length" class="detail-state-note">内部冲突：{{ formatDisplayValue(conflicts) }}</p>
                  </section>

                  <section class="detail-state-section">
                    <div class="detail-state-heading"><h4>人格动力与偏好</h4><span>{{ growthSlots.length }} 项</span></div>
                    <p v-if="!growthSlots.length" class="field-note">还没有形成可追踪的动力或偏好。</p>
                    <ul v-else class="state-slot-list">
                      <li v-for="slot in growthSlots" :key="String(slot.id ?? slot.key)"><div><strong>{{ slotName(slot) }}</strong><span>{{ slot.value_schema ? formatDisplayValue(slot.value_schema) : "动力" }}</span></div><p>{{ slotValue(slot) }}</p><small v-if="slot.revision !== undefined">修订 {{ formatDisplayValue(slot.revision) }}</small></li>
                    </ul>
                  </section>
                </div>
              </details>
            </section>
          </TabsContent>

          <TabsContent value="world" class="detail-tab-pane mt-0 space-y-4">
            <section class="detail-block">
              <div class="detail-block-heading"><p class="eyebrow">生活世界</p><h3>生活脉络</h3></div>
              <dl v-if="Object.keys(primaryLife).length" class="identity-facts detail-facts-compact">
                <template v-for="(value, key) in primaryLife" :key="String(key)"><dt>{{ labelFor(String(key)) }}</dt><dd><span>{{ humanValue(value) }}</span><small v-if="isCustomLabel(String(key))" class="raw-field-name">字段名：{{ String(key) }}</small></dd></template>
              </dl>
              <div v-if="appearance.description || Object.keys(appearanceFeatures).length || appearance.daily_outfit_preferences" class="appearance-summary">
                <h4>外貌与穿衣偏好</h4><p v-if="appearance.description">{{humanValue(appearance.description)}}</p><dl v-if="Object.keys(appearanceFeatures).length" class="identity-facts"><template v-for="(value,key) in appearanceFeatures" :key="String(key)"><dt>{{labelFor(String(key))}}</dt><dd>{{humanValue(value)}}</dd></template></dl><p v-if="appearance.daily_outfit_preferences" class="field-note">衣物偏好：{{humanValue(appearance.daily_outfit_preferences)}}</p>
              </div>
              <JsonViewer :value="lifeProfile" label="完整生活属性与扩展字段" />
              <div class="detail-summary-grid">
                <div><span>目标记录</span><strong>{{ goals.length }}</strong></div>
                <div><span>关系</span><strong>{{ relationships.length }}</strong></div>
                <div><span>可展示记忆</span><strong>{{ memories.length }}</strong></div>
              </div>
              <details class="detail-state-drawer wardrobe-drawer" @toggle="onWardrobeToggle">
                <summary><span><strong>衣柜与物品</strong><small>查看已记录物品与当前穿着</small></span><ChevronDown class="disclosure-icon" :size="18" aria-hidden="true" /></summary>
                <div class="detail-state-drawer-body">
                  <section class="detail-state-section">
                    <div class="detail-state-heading"><h4>当前穿着</h4><span>{{ (wornItems ?? []).length }} 件</span></div>
                    <p v-if="!(wornItems ?? []).length" class="field-note">当前没有已确认的穿着记录。</p>
                    <ul v-else class="modal-detail-list"><li v-for="item in wornItems" :key="String(item.id)"><strong>{{ formatDisplayValue(item.description) }}</strong><small>{{ wardrobeSlotLabel(item.slot) }} · {{ enumLabel(item.availability) }}</small></li></ul>
                  </section>
                  <section class="detail-state-section">
                    <div class="detail-state-heading"><h4>已记录物品</h4><span>{{ (wardrobeItems ?? []).length }} 件{{ wardrobeHasMore ? "以上" : "" }}</span></div>
                    <p v-if="wardrobeLoaded && !wardrobeInventoryComplete" class="field-note">这是已记录清单；未列出的物品不代表不存在。</p>
                    <p v-if="wardrobeLoading && !wardrobeLoaded" class="field-note">正在读取衣柜...</p>
                    <p v-if="wardrobeError" class="field-note" role="alert">{{ wardrobeError }}</p>
                    <p v-if="wardrobeLoaded && !(wardrobeItems ?? []).length" class="field-note">尚无已记录的物品。</p>
                    <ul v-if="(wardrobeItems ?? []).length" class="modal-detail-list"><li v-for="item in wardrobeItems" :key="item.id"><strong>{{ item.description }}</strong><small>{{ wardrobeCategoryLabel(item.category) }} · {{ wardrobeSlotLabel(item.slot) }} · {{ enumLabel(item.ownership) }} · {{ enumLabel(item.availability) }}</small></li></ul>
                    <Button v-if="wardrobeError || wardrobeHasMore" variant="outline" type="button" :disabled="wardrobeLoading" @click="loadWardrobePage">{{ wardrobeLoading ? "读取中..." : wardrobeError ? "重试" : "查看更多物品" }}</Button>
                  </section>
                </div>
              </details>

              <h3>目标与意图</h3>
              <p v-if="!goals.length && !intentions.length" class="field-note">当前没有目标或待执行意图。</p>
              <ul v-else class="modal-detail-list">
                <li v-for="goal in goals" :key="String(goal.id)"><strong>{{ formatDisplayValue(goal.description) }}</strong><small>{{ enumLabel(goal.status) }} · {{ formatDisplayValue(goal.progress) }}</small></li>
                <li v-for="intention in intentions" :key="String(intention.id)"><strong>{{ formatDisplayValue(intention.action) }}</strong><small>{{ enumLabel(intention.status) }}</small></li>
              </ul>

              <details class="detail-state-drawer">
                <summary><span><strong>高级目标规划（完整监控视图）</strong><small>查看目标集合自动规划与生命周期详情</small></span><ChevronDown class="disclosure-icon" :size="18" aria-hidden="true" /></summary>
                <div class="detail-state-drawer-body">
                  <GoalPlanningPanel :fluctlight-id="store.fluctlightId ?? ''" :timezone="controlCenter.diagnosticDisplayTimezone" read-only />
                  <GoalPanel :fluctlight-id="store.selectedFluctlight.id" read-only />
                </div>
              </details>

              <h3>当前上下文</h3>
              <p class="field-note">{{ enumLabel(context.source) }}<template v-if="context.location"> · {{ formatDisplayValue(context.location) }}</template></p>
              <h3>今日日程 <small class="timezone-note">{{ scheduleTimezone }}</small></h3>
              <p v-if="!Object.keys(schedule).length || !scheduleItems.length" class="field-note">日程待生成，当前没有接受的本地日计划。</p>
              <ol v-else class="timeline-list detail-timeline">
                  <li v-for="item in scheduleItems" :key="String(item.id)" :class="{ active: isCurrentScheduleItem(item) }" :aria-current="isCurrentScheduleItem(item) ? 'time' : undefined">
                    <time :datetime="String(item.start_at ?? '')">{{ formatZonedRange(item.start_at, item.end_at, scheduleTimezone) }}</time>
                  <div><strong>{{ formatDisplayValue(item.activity) }}<span v-if="isCurrentScheduleItem(item)" class="timeline-now-badge">当前时段</span></strong><span>{{ formatDisplayValue(item.scene) }}<template v-if="item.status"> · {{ enumLabel(item.status) }}</template><template v-if="item.action_status"> · 意图 {{ enumLabel(item.action_status) }}</template></span></div>
                </li>
              </ol>
            </section>
          </TabsContent>

          <TabsContent value="memory" class="detail-tab-pane mt-0 space-y-4">
            <section class="detail-block">
              <div class="detail-block-heading"><p class="eyebrow">关系与记忆</p><h3>关系状态</h3></div>
              <p v-if="!relationships.length" class="field-note">尚未形成关系状态。</p>
              <ul v-else class="modal-detail-list"><li v-for="relationship in relationships" :key="relationshipKey(relationship)"><strong>{{ actorName(relationship.target_actor_id, Boolean(relationship.is_current_user)) }}<span v-if="relationship.is_current_user" class="status-pill">当前用户</span></strong><small>{{ relationship.target_actor_type === "human" ? "用户" : relationship.target_actor_type === "fluctlight" ? "摇光实例" : "交流对象" }} · profile {{ formatDisplayValue(relationship.profile_id ?? "shared") }} · {{ formatDisplayValue((relationship.role as Record<string, unknown> | undefined)?.label ?? (relationship.role as Record<string, unknown> | undefined)?.primary ?? "unknown") }} · {{ enumLabel(relationship.trend) }}<template v-if="relationship.summary"> · {{ formatDisplayValue(relationship.summary) }}</template></small></li></ul>
              <h3>可展示记忆</h3>
              <p v-if="!memories.length" class="field-note">暂无可展示的记忆。</p>
              <ul v-else class="modal-detail-list"><li v-for="memory in memories" :key="String(memory.id)"><strong>{{ formatDisplayValue(memory.content) }}</strong><small>{{ enumLabel(memory.type) }} · {{ enumLabel(memory.status) }}</small></li></ul>
            </section>
          </TabsContent>
        </Tabs>
      </div>

      <DialogFooter class="detail-dialog-footer m-0">
        <Button class="secondary-button" variant="outline" type="button" @click="emit('manage')">进入编辑与治理</Button>
        <DialogClose as-child><Button class="primary-button" type="button">完成</Button></DialogClose>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
