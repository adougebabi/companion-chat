<script setup lang="ts">
import { computed } from "vue";

import Button from "@/components/ui/button/Button.vue";
import Input from "@/components/ui/input/Input.vue";
import Textarea from "@/components/ui/textarea/Textarea.vue";
import AsyncState from "../../components/product/AsyncState.vue";
import FeedbackNotice from "../../components/product/FeedbackNotice.vue";
import { asRecord, asRecords } from "../../lib/life-surface";
import { enumLabel, formatDisplayValue, labelFor } from "../../lib/fluctlight-display";
import { fluctlightStatusLabel } from "../../lib/fluctlight-status";
import { userError } from "../../lib/user-feedback";
import { useCharacterSurface } from "./use-character-surface";

const emit = defineEmits<{ openMedia: []; openAutonomy: [] }>();
const { store, control, detail, readOnlyPreview } = useCharacterSurface();
const identity = computed(() => asRecord(detail.value?.identity));
const personality = computed(() => asRecord(detail.value?.personality));
const revisions = computed(() => asRecords(detail.value?.foundation_revisions));
const claims = computed(() => asRecords(asRecord(detail.value?.developing_self)?.claims));
const identityRows = computed(() => Object.entries(identity.value ?? {}).filter(([, value]) => value !== null && value !== undefined && value !== ""));

const summaryKeys = ["name", "occupation", "background_story", "background", "timezone"];
</script>

<template>
  <section class="page life-page" aria-labelledby="profile-title">
    <header class="pagehead">
      <h1 id="profile-title">角色资料</h1>
      <span class="badge b-plain">BP-05</span>
      <span v-if="detail?.status" class="badge">{{ fluctlightStatusLabel(detail.status) }}</span>
      <span v-if="readOnlyPreview" class="badge b-preview">不是业务数据</span>
      <Button v-if="!readOnlyPreview" variant="outline" type="button" @click="emit('openAutonomy')">自治与权限</Button>
    </header>
    <p v-if="!store.fluctlightId && !readOnlyPreview" class="statebox">请先选择角色。这里不会显示其他角色的数据。</p>
    <template v-else>
      <FeedbackNotice :message="control.error ? userError(control.error) : ''" />
      <FeedbackNotice :message="control.governanceNotice" tone="success" />
      <AsyncState :loading="!readOnlyPreview && control.loading && !detail" loading-label="正在读取角色资料…" />
      <div v-if="detail" class="life-grid">
        <article class="card">
          <h2>{{ formatDisplayValue(identity?.name) }}</h2>
          <p v-if="!identityRows.length" class="small muted">尚未形成身份信息。</p>
          <dl v-else>
            <template v-for="key in summaryKeys" :key="key">
              <div v-if="identity?.[key] != null && identity[key] !== ''"><dt>{{ labelFor(key) }}</dt><dd>{{ formatDisplayValue(identity[key]) }}</dd></div>
            </template>
          </dl>
          <h3>人格</h3>
          <p v-if="!personality || !Object.keys(personality).length" class="small muted">尚未配置人格特征。</p>
          <p v-else class="small">{{ formatDisplayValue(personality) }}</p>
          <p class="small muted">当前基础 revision {{ formatDisplayValue(detail.current_revision) }}</p>
          <button class="btn sm" type="button" @click="emit('openMedia')">视觉身份</button>
        </article>
        <article class="card">
          <h2>人格提案</h2>
          <p class="small muted">提案是基础修订。接受、拒绝和回滚都带 expectedRevision 与原因。直接保存会先提出再接受。</p>
          <p v-if="!Object.prototype.hasOwnProperty.call(detail, 'foundation_revisions')" class="statebox">人格提案尚未接入。</p>
          <p v-else-if="!revisions.length" class="small muted">还没有修订记录。</p>
          <ul v-else>
            <li v-for="revision in revisions" :key="String(revision.id)">
              <strong>版本 {{ formatDisplayValue(revision.revision) }} · {{ enumLabel(revision.source) }}</strong>
              <small>{{ enumLabel(revision.status) }}<template v-if="revision.reason"> · {{ formatDisplayValue(revision.reason) }}</template></small>
              <div v-if="!readOnlyPreview && revision.status === 'proposed'" class="row">
                <Button variant="ghost" type="button" :disabled="control.saving" @click="control.acceptFoundationRevision(store.fluctlightId, String(revision.id))">接受</Button>
                <Button variant="ghost" type="button" :disabled="control.saving || !control.revisionReason.trim()" @click="control.rejectFoundationRevision(store.fluctlightId, String(revision.id))">拒绝</Button>
              </div>
            </li>
          </ul>
          <div v-if="!readOnlyPreview" class="stack">
            <Button variant="ghost" type="button" @click="control.populateCurrentFoundationJson()">载入当前完整属性 JSON</Button>
            <label for="life-revision-json">基础修订 JSON<Textarea id="life-revision-json" v-model="control.revisionChangesJson" rows="8" spellcheck="false" /></label>
            <label for="life-revision-reason">修订原因<Input id="life-revision-reason" v-model="control.revisionReason" maxlength="1024" placeholder="拒绝、回滚和仅提出修订时必填" /></label>
            <div class="row">
              <Button variant="outline" type="button" :disabled="control.saving || !control.revisionChangesJson.trim()" @click="control.applyFoundationChanges(store.fluctlightId)">保存并立即生效</Button>
              <Button variant="ghost" type="button" :disabled="control.saving || !control.revisionChangesJson.trim() || !control.revisionReason.trim()" @click="control.submitFoundationRevision(store.fluctlightId)">仅提出修订</Button>
            </div>
            <label for="life-rollback-revision">回滚目标版本<Input id="life-rollback-revision" v-model="control.rollbackTargetRevision" type="number" min="0" step="1" /></label>
            <Button variant="outline" type="button" :disabled="control.saving || !control.rollbackTargetRevision || !control.revisionReason.trim()" @click="control.rollbackFoundationRevision(store.fluctlightId)">回滚到该版本</Button>
          </div>
        </article>
        <article class="card">
          <h2>自我认知</h2>
          <p v-if="!claims.length" class="small muted">当前还没有形成自我认知 claim。</p>
          <ul v-else><li v-for="claim in claims" :key="String(claim.id)">{{ formatDisplayValue(claim.claim) }} <small>revision {{ formatDisplayValue(claim.revision) }}</small></li></ul>
        </article>
      </div>
    </template>
  </section>
</template>
