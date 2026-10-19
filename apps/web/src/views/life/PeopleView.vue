<script setup lang="ts">
import { computed } from "vue";

import Button from "@/components/ui/button/Button.vue";
import Input from "@/components/ui/input/Input.vue";
import Textarea from "@/components/ui/textarea/Textarea.vue";
import ActorContextEditor from "../../components/instances/ActorContextEditor.vue";
import AsyncState from "../../components/product/AsyncState.vue";
import ChoiceSelect from "../../components/product/ChoiceSelect.vue";
import FeedbackNotice from "../../components/product/FeedbackNotice.vue";
import { asRecords, editableActorOptions, relationshipDraftKey } from "../../lib/life-surface";
import { enumLabel, formatDisplayValue } from "../../lib/fluctlight-display";
import { userError } from "../../lib/user-feedback";
import { useCharacterSurface } from "./use-character-surface";

const { store, control, detail, readOnlyPreview } = useCharacterSurface();
const relationships = computed(() => asRecords(detail.value?.relationships));
const actors = computed(() => editableActorOptions(detail.value, store.fluctlights.map((item) => ({ id: item.id, name: String(item.identity.name ?? item.id) })), store.authenticatedActorId));
const hasRelationships = computed(() => Boolean(detail.value && Object.prototype.hasOwnProperty.call(detail.value, "relationships")));
</script>

<template>
  <section class="page life-page" aria-labelledby="people-title">
    <header class="pagehead">
      <h1 id="people-title">人物关系</h1>
      <span class="badge b-plain">BP-12</span>
      <span v-if="readOnlyPreview" class="badge b-preview">不是业务数据</span>
    </header>
    <nav class="row"><a class="btn" :href="`#/c/${store.fluctlightId}/people/owner`">关于你</a><a class="btn" :href="`#/c/${store.fluctlightId}/people/list`">人物 / NPC 目录</a><a class="btn ghost" :href="`#/c/${store.fluctlightId}/world`">运行世界</a></nav>
    <p class="banner info">资料、NPC 设定和角色对某个 Actor 的关系分别保存。所有者修正会保留版本与来源。</p>
    <p v-if="!store.fluctlightId && !readOnlyPreview" class="statebox">请先选择角色。这里不会显示其他角色的数据。</p>
    <template v-else>
      <FeedbackNotice :message="control.error ? userError(control.error) : ''" />
      <FeedbackNotice :message="control.governanceNotice" tone="success" />
      <AsyncState :loading="!readOnlyPreview && control.loading && !detail" loading-label="正在读取关系…" />
      <p v-if="detail && !hasRelationships" class="statebox">关系尚未接入。</p>
      <p v-else-if="hasRelationships && !relationships.length" class="small muted">尚未形成关系状态。</p>
      <ul v-else class="stack">
        <li v-for="relationship in relationships" :key="relationshipDraftKey(relationship)" class="card">
          <strong>{{ formatDisplayValue(relationship.target_actor_id) }}</strong>
          <span v-if="relationship.is_current_user" class="badge">当前用户</span>
          <p class="small muted">{{ relationship.target_actor_type === "human" ? "Human" : relationship.target_actor_type === "fluctlight" ? "Fluctlight" : "Actor" }} · profile {{ formatDisplayValue(relationship.profile_id ?? "shared") }} · {{ enumLabel(relationship.trend) }} · revision {{ formatDisplayValue(relationship.revision) }}</p>
          <template v-if="!readOnlyPreview && control.relationshipEditDrafts[relationshipDraftKey(relationship)]">
            <div class="form-grid">
              <label :for="'life-rel-trend-' + relationshipDraftKey(relationship)">趋势
                <ChoiceSelect :id="'life-rel-trend-' + relationshipDraftKey(relationship)" v-model="control.relationshipEditDrafts[relationshipDraftKey(relationship)].trend" :options="[{ value: 'improving', label: '升温' }, { value: 'stable', label: '稳定' }, { value: 'declining', label: '降温' }]" />
              </label>
              <label :for="'life-rel-summary-' + relationshipDraftKey(relationship)">关系摘要<Input :id="'life-rel-summary-' + relationshipDraftKey(relationship)" v-model="control.relationshipEditDrafts[relationshipDraftKey(relationship)].summary" maxlength="32000" /></label>
            </div>
            <label :for="'life-rel-role-' + relationshipDraftKey(relationship)">关系角色 JSON<Textarea :id="'life-rel-role-' + relationshipDraftKey(relationship)" v-model="control.relationshipEditDrafts[relationshipDraftKey(relationship)].role" rows="3" spellcheck="false" /></label>
            <label :for="'life-rel-metrics-' + relationshipDraftKey(relationship)">关系指标 JSON<Textarea :id="'life-rel-metrics-' + relationshipDraftKey(relationship)" v-model="control.relationshipEditDrafts[relationshipDraftKey(relationship)].metrics" rows="3" spellcheck="false" /></label>
            <label :for="'life-rel-emotion-' + relationshipDraftKey(relationship)">情绪关联 JSON<Textarea :id="'life-rel-emotion-' + relationshipDraftKey(relationship)" v-model="control.relationshipEditDrafts[relationshipDraftKey(relationship)].emotionalAssociation" rows="3" spellcheck="false" /></label>
            <div class="row">
              <Button variant="outline" type="button" :disabled="control.saving" @click="control.editRelationship(store.fluctlightId, relationship)">保存关系</Button>
              <Input v-model="control.relationshipRollbackTargets[relationshipDraftKey(relationship)]" aria-label="关系回滚目标版本" type="number" min="0" step="1" placeholder="回滚目标版本" />
              <Button variant="ghost" type="button" :disabled="control.saving" @click="control.rollbackRelationship(store.fluctlightId, relationship)">回滚关系</Button>
            </div>
          </template>
        </li>
      </ul>
      <label v-if="!readOnlyPreview" for="life-people-evidence">证据引用<Input id="life-people-evidence" v-model="control.governanceEvidence" maxlength="4096" placeholder="以逗号分隔" /></label>
      <label v-if="!readOnlyPreview" for="life-people-reason">原因<Input id="life-people-reason" v-model="control.governanceReason" maxlength="1024" placeholder="编辑关系必须填写原因" /></label>
      <p v-if="!readOnlyPreview" class="small muted">保存发送 expectedRevision、原因和证据。回滚还要目标 revision。版本冲突时保留草稿。</p>
      <ActorContextEditor v-if="!readOnlyPreview && store.fluctlightId" :fluctlight-id="store.fluctlightId" :actors="actors" :owner-actor-id="store.authenticatedActorId" />
    </template>
  </section>
</template>
