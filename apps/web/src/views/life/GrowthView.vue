<script setup lang="ts">
import { computed } from "vue";

import Button from "@/components/ui/button/Button.vue";
import Input from "@/components/ui/input/Input.vue";
import AsyncState from "../../components/product/AsyncState.vue";
import FeedbackNotice from "../../components/product/FeedbackNotice.vue";
import { asRecords } from "../../lib/life-surface";
import { formatDisplayValue } from "../../lib/fluctlight-display";
import { userError } from "../../lib/user-feedback";
import { useCharacterSurface } from "./use-character-surface";

const { store, control, detail, readOnlyPreview } = useCharacterSurface();
const memories = computed(() => asRecords(detail.value?.memories));
const hasMemories = computed(() => Boolean(detail.value && Object.prototype.hasOwnProperty.call(detail.value, "memories")));
const claims = computed(() => asRecords(asRecordClaims(detail.value?.developing_self)));

function asRecordClaims(value: unknown) {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>).claims : undefined;
}
</script>

<template>
  <section class="page life-page" aria-labelledby="growth-title">
    <header class="pagehead">
      <h1 id="growth-title">记忆成长</h1>
      <span class="badge b-plain">BP-13</span>
      <span v-if="readOnlyPreview" class="badge b-preview">不是业务数据</span>
    </header>
    <p class="small muted">修正和遗忘发送 expectedRevision 与证据引用。版本变化时保留填写内容。日记、反思时间线和“告诉她”不在这条写入里。</p>
    <p v-if="!store.fluctlightId && !readOnlyPreview" class="statebox">请先选择角色。这里不会显示其他角色的数据。</p>
    <template v-else>
      <FeedbackNotice :message="control.error ? userError(control.error) : ''" />
      <AsyncState :loading="!readOnlyPreview && control.loading && !detail" loading-label="正在读取记忆…" />
      <p v-if="detail && !hasMemories" class="statebox">记忆尚未接入。</p>
      <p v-else-if="hasMemories && !memories.length" class="small muted">暂无可展示的记忆。</p>
      <ul v-else class="stack">
        <li v-for="memory in memories" :key="String(memory.id)" class="card">
          <p>{{ formatDisplayValue(memory.content) }}</p>
          <p class="small muted">revision {{ formatDisplayValue(memory.revision) }}<template v-if="memory.kind"> · {{ memory.kind }}</template></p>
          <div v-if="!readOnlyPreview" class="row">
            <Input v-model="control.memoryEdits[String(memory.id)]" :aria-label="'修正记忆 ' + memory.id" maxlength="4096" placeholder="修正内容" />
            <Button variant="ghost" type="button" :disabled="control.saving" @click="control.reviseMemory(memory)">修正</Button>
            <Button variant="ghost" type="button" :disabled="control.saving" @click="control.forgetMemory(memory)">遗忘</Button>
          </div>
        </li>
      </ul>
      <label v-if="!readOnlyPreview" for="life-memory-evidence">证据引用<Input id="life-memory-evidence" v-model="control.governanceEvidence" maxlength="4096" placeholder="以逗号分隔，至少一条" /></label>
      <article class="card">
        <h2>自我认知</h2>
        <p class="small muted">这些 claim 带自己的 revision。回滚或标记不准确需要原因，不会改写 Core Persona。</p>
        <p v-if="!claims.length" class="small muted">当前还没有形成自我认知 claim。</p>
        <ul v-else>
          <li v-for="claim in claims" :key="String(claim.id)">
            <strong>{{ formatDisplayValue(claim.claim) }}</strong>
            <small> revision {{ formatDisplayValue(claim.revision) }}</small>
            <div v-if="!readOnlyPreview" class="row">
              <Button variant="ghost" type="button" :disabled="control.saving || !control.governanceReason.trim()" @click="control.rollbackDevelopingSelf(store.fluctlightId, claim)">回滚上一版</Button>
              <Button variant="ghost" type="button" :disabled="control.saving || !control.governanceReason.trim()" @click="control.forgetDevelopingSelf(store.fluctlightId, claim)">标记不准确</Button>
            </div>
          </li>
        </ul>
        <label v-if="!readOnlyPreview" for="life-growth-reason">原因<Input id="life-growth-reason" v-model="control.governanceReason" maxlength="1024" placeholder="回滚或标记不准确时必填" /></label>
      </article>
      <p class="statebox">反思详情尚未接入。</p>
    </template>
  </section>
</template>
