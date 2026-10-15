<script setup lang="ts">
import { computed } from "vue";

import ActorContextEditor from "../../components/instances/ActorContextEditor.vue";
import GoalPanel from "../../components/instances/GoalPanel.vue";
import GoalPlanningPanel from "../../components/instances/GoalPlanningPanel.vue";
import AsyncState from "../../components/product/AsyncState.vue";
import FeedbackNotice from "../../components/product/FeedbackNotice.vue";
import { editableActorOptions, goalProfileOptions, recordedCommitments } from "../../lib/life-surface";
import { userError } from "../../lib/user-feedback";
import { useCharacterSurface } from "./use-character-surface";

const { store, control, detail, readOnlyPreview } = useCharacterSurface();
const actors = computed(() => editableActorOptions(detail.value, store.fluctlights.map((item) => ({ id: item.id, name: String(item.identity.name ?? item.id) })), store.authenticatedActorId));
const profiles = computed(() => goalProfileOptions(detail.value));
const commitments = computed(() => recordedCommitments(detail.value));
</script>

<template>
  <section class="page life-page" aria-labelledby="life-goals-title">
    <header class="pagehead">
      <h1 id="life-goals-title">目标</h1>
      <span class="badge b-plain">BP-07</span>
      <span v-if="readOnlyPreview" class="badge b-preview">不是业务数据</span>
    </header>
    <p v-if="!store.fluctlightId && !readOnlyPreview" class="statebox">请先选择角色。这里不会显示其他角色的数据。</p>
    <template v-else-if="readOnlyPreview">
      <p class="small muted">目标写入仍使用 expectedVersion、expectedFactsRevision、稳定幂等键和原因。预览不提交命令。</p>
      <article class="card">
        <h2>约定</h2>
        <p v-if="commitments.state === 'missing'">约定尚未接入。</p>
        <p v-else-if="!commitments.items.length" class="small muted">当前没有记录的约定。</p>
        <ul v-else><li v-for="item in commitments.items" :key="String(item.id ?? item.goal_id)">{{ item.title || item.summary || item.goal_outcome }}</li></ul>
      </article>
    </template>
    <template v-else>
      <FeedbackNotice :message="control.error ? userError(control.error) : ''" />
      <AsyncState :loading="control.loading && !detail" loading-label="正在读取目标上下文…" />
      <p class="small muted">排序、依赖和激活走现有目标集命令。冲突保留草稿。约定只展示目标上的 commitments，不从标题猜测。</p>
      <article v-if="detail" class="card">
        <h2>约定</h2>
        <p v-if="commitments.state === 'missing'">约定尚未接入。</p>
        <p v-else-if="!commitments.items.length" class="small muted">当前没有记录的约定。</p>
        <ul v-else><li v-for="item in commitments.items" :key="String(item.id ?? item.goal_id)">{{ item.title || item.summary || item.goal_outcome }}</li></ul>
      </article>
      <GoalPlanningPanel v-if="store.fluctlightId" :fluctlight-id="store.fluctlightId" :timezone="control.diagnosticDisplayTimezone" />
      <GoalPanel v-if="store.fluctlightId" :fluctlight-id="store.fluctlightId" :actors="actors" :profile-options="profiles" />
      <ActorContextEditor v-if="store.fluctlightId" :fluctlight-id="store.fluctlightId" :actors="actors" :owner-actor-id="store.authenticatedActorId" />
    </template>
  </section>
</template>
