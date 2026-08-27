<script setup lang="ts">
import { ref } from "vue";

import { useConversationStore } from "../stores/conversations";
import { useControlCenterStore } from "../stores/control-center";

const emit = defineEmits<{ close: []; retired: [] }>();
const store = useConversationStore();
const controlCenter = useControlCenterStore();
const retirementReason = ref("");
const retirementConfirmation = ref("");

const labels: Record<string, string> = {
  id: "标识", name: "名称", age: "年龄", gender: "性别", occupation: "职业", residence: "居住地", timezone: "时区", birthday: "生日", background: "背景", biography: "经历", notes: "备注",
  openness: "开放性", conscientiousness: "尽责性", extraversion: "外向性", agreeableness: "宜人性", neuroticism: "情绪敏感度", curiosity: "好奇心", independence: "独立性", patience: "耐心", empathy: "共情", assertiveness: "主张性", humor: "幽默感", sociability: "社交性", risk_tolerance: "风险偏好", update_policy: "更新策略", response_style: "回复风格", message_length: "消息长度", emoji_frequency: "表情频率", punctuation_style: "标点风格", humor_style: "幽默风格", sarcasm_tendency: "讽刺倾向", directness: "直接性", initiative: "主动性", topic_initiation: "发起话题", silence_tolerance: "沉默容忍度", response_delay: "回复延迟", emotional_expression: "情绪表达", conflict_style: "冲突风格", refusal_style: "拒绝风格", intimacy_expression: "亲密表达",
};
function labelFor(key: string) { return labels[key] ?? key; }
function stringify(value: unknown) { return Array.isArray(value) ? value.join("、") : String(value ?? "未设定"); }

async function retire() {
  const id = store.fluctlightId;
  const name = store.selectedFluctlightName;
  if (!id || !name) return;
  if (retirementConfirmation.value.trim() !== name) {
    controlCenter.error = "请输入完整的摇光名称以确认删除。";
    return;
  }
  const retired = await controlCenter.retireFluctlight(id, retirementReason.value);
  if (!retired) return;
  retirementReason.value = "";
  retirementConfirmation.value = "";
  await store.bootstrap();
  emit("retired");
}
</script>

<template>
  <section v-if="store.selectedFluctlight" class="page governance-page" aria-labelledby="governance-title">
    <header class="page-header governance-header">
      <div>
        <button class="back-link" type="button" @click="emit('close')">← 返回实例</button>
        <p class="eyebrow">EDIT &amp; GOVERN</p>
        <h1 id="governance-title">{{ store.selectedFluctlightName }} 的编辑与治理</h1>
        <p class="page-lede">这里的修改会影响后续状态。查看摘要、生活脉络和私有动态，请从对话标题栏打开详情。</p>
      </div>
      <span class="status-pill" :class="{ paused: controlCenter.fluctlightDetail?.status === 'paused' }">
        {{ controlCenter.fluctlightDetail?.status === "paused" ? "已暂停" : "可对话" }}
      </span>
    </header>

    <p v-if="controlCenter.error" class="error-banner" role="alert">{{ controlCenter.error }}</p>

    <template v-if="controlCenter.fluctlightDetail">
      <section class="governance-section governance-overview">
        <div class="section-heading"><span class="section-index">01</span><div><p class="eyebrow">OVERVIEW</p><h2>当前状态</h2></div></div>
        <p class="field-note">暂停会阻止新的自主外部行为，历史事实和已观察到的状态不会被删除。</p>
        <div class="governance-inline-form">
          <label for="governance-reason">操作原因</label>
          <input id="governance-reason" v-model="controlCenter.governanceReason" maxlength="1024" placeholder="填写暂停或恢复的原因" />
          <button class="secondary-button" type="button" :disabled="controlCenter.saving || !controlCenter.governanceReason.trim()" @click="controlCenter.setFluctlightStatus(store.fluctlightId, controlCenter.fluctlightDetail?.status === 'paused' ? 'active' : 'paused')">
            {{ controlCenter.fluctlightDetail.status === "paused" ? "恢复自主性" : "暂停自主性" }}
          </button>
        </div>
      </section>

      <details class="governance-section" open>
        <summary class="section-heading"><span class="section-index">02</span><div><p class="eyebrow">IDENTITY</p><h2>身份与人格</h2></div><span class="disclosure-icon" aria-hidden="true">⌄</span></summary>
        <h3>身份设定</h3>
        <dl class="governance-facts"><template v-for="(value, key) in store.selectedFluctlight.identity" :key="String(key)"><dt>{{ labelFor(String(key)) }}</dt><dd>{{ stringify(value) }}</dd></template></dl>
        <h3>人格与表达</h3>
        <dl class="governance-facts"><template v-for="(value, key) in controlCenter.fluctlightDetail.personality as Record<string, unknown>" :key="String(key)"><dt>{{ labelFor(String(key)) }}</dt><dd>{{ typeof value === "object" ? "已配置" : stringify(value) }}</dd></template></dl>
        <h3>表达策略</h3>
        <dl class="governance-facts"><template v-for="(value, key) in controlCenter.fluctlightDetail.behavioral_policy as Record<string, unknown>" :key="String(key)"><dt>{{ labelFor(String(key)) }}</dt><dd>{{ stringify(value) }}</dd></template></dl>
      </details>

      <details class="governance-section">
        <summary class="section-heading"><span class="section-index">03</span><div><p class="eyebrow">LIFE WORLD</p><h2>生活世界</h2></div><span class="disclosure-icon" aria-hidden="true">⌄</span></summary>
        <div class="state-summary-grid"><div><span>情绪</span><strong>{{ String(((controlCenter.fluctlightDetail.inner_state as Record<string, any>).mood?.label) ?? "未形成") }}</strong></div><div><span>场景</span><strong>{{ String((controlCenter.fluctlightDetail.context as Record<string, any>)?.scene ?? "待确认") }}</strong></div><div><span>活动</span><strong>{{ String((controlCenter.fluctlightDetail.context as Record<string, any>)?.activity ?? "待规划") }}</strong></div></div>
        <h3>目标与意图</h3>
        <p v-if="!(controlCenter.fluctlightDetail.goals as unknown[])?.length" class="field-note">当前没有活跃目标。</p>
        <ul v-else class="detail-list"><li v-for="goal in controlCenter.fluctlightDetail.goals as Array<Record<string, unknown>>" :key="String(goal.id)"><strong>{{ String(goal.description ?? "未命名目标") }}</strong><small>{{ String(goal.status ?? "") }} · {{ String(goal.progress ?? "") }}</small></li></ul>
        <p v-if="!(controlCenter.fluctlightDetail.intentions as unknown[])?.length" class="field-note">当前没有待执行意图。</p>
        <ul v-else class="detail-list"><li v-for="intention in controlCenter.fluctlightDetail.intentions as Array<Record<string, unknown>>" :key="String(intention.id)"><strong>{{ String(intention.action ?? "未命名意图") }}</strong><small>{{ String(intention.status ?? "") }}</small></li></ul>
        <h3>今日 Schedule</h3>
        <p v-if="!controlCenter.fluctlightDetail.schedule" class="field-note">日程待生成，当前没有接受的本地日计划。</p>
        <ul v-else class="timeline-list"><li v-for="item in (controlCenter.fluctlightDetail.schedule as Record<string, any>).items" :key="String(item.id)"><time>{{ String(item.start_at ?? "").slice(11, 16) }}</time><div><strong>{{ String(item.activity ?? "休息") }}</strong><span>{{ String(item.scene ?? "") }}</span></div></li></ul>
        <button v-if="controlCenter.fluctlightDetail.schedule" class="secondary-button" type="button" :disabled="controlCenter.saving" @click="controlCenter.cancelSchedule(store.fluctlightId)">取消当前日程</button>
        <form class="governance-form" @submit.prevent="controlCenter.acceptSchedule(store.fluctlightId)"><label for="schedule-draft">完整日程 JSON</label><textarea id="schedule-draft" v-model="controlCenter.scheduleDraftJson" rows="6" spellcheck="false" placeholder='{"localDate":"2026-08-26","timezone":"Asia/Shanghai","items":[]}' /><button class="secondary-button" type="submit" :disabled="controlCenter.saving || !controlCenter.scheduleDraftJson.trim()">提交完整日程</button></form>
        <h3>Event 与 Presence</h3>
        <form class="governance-form" @submit.prevent="controlCenter.createLifeEvent(store.fluctlightId)">
          <label for="event-kind">Event 类型<input id="event-kind" v-model="controlCenter.lifeEvent.kind" maxlength="128" required /></label>
          <div class="form-grid"><label for="event-start">开始时间<input id="event-start" v-model="controlCenter.lifeEvent.startAt" type="datetime-local" required /></label><label for="event-end">结束时间<input id="event-end" v-model="controlCenter.lifeEvent.endAt" type="datetime-local" required /></label></div>
          <div class="form-grid"><label for="event-scene">场景（可选）<input id="event-scene" v-model="controlCenter.lifeEvent.scene" maxlength="512" /></label><label for="event-activity">活动（可选）<input id="event-activity" v-model="controlCenter.lifeEvent.activity" maxlength="512" /></label></div>
          <label for="event-location">地点（可选）<input id="event-location" v-model="controlCenter.lifeEvent.location" maxlength="512" /></label>
          <p class="field-note">Event 创建需要在下方“证据引用”中填写至少一条可追溯引用。</p>
          <button class="secondary-button" type="submit" :disabled="controlCenter.saving">创建确认 Event</button>
        </form>
        <ul v-if="(controlCenter.fluctlightDetail.events as unknown[])?.length" class="detail-list"><li v-for="event in controlCenter.fluctlightDetail.events as Array<Record<string, unknown>>" :key="String(event.id)"><strong>{{ String(event.kind) }}</strong><small>{{ String(event.start_at).slice(0, 16) }} — {{ String(event.end_at).slice(0, 16) }} · {{ String(event.status) }}</small><button v-if="event.status === 'confirmed'" class="text-button" type="button" :disabled="controlCenter.saving" @click="controlCenter.cancelLifeEvent(store.fluctlightId, String(event.id))">取消 Event</button></li></ul>
        <form class="governance-form" @submit.prevent="controlCenter.setPresence(store.fluctlightId)"><div class="form-grid"><label for="user-presence">用户 Presence<input id="user-presence" v-model="controlCenter.presence.userPresence" maxlength="128" /></label><label for="current-task">当前任务<input id="current-task" v-model="controlCenter.presence.currentTask" maxlength="512" /></label></div><button class="secondary-button" type="submit" :disabled="controlCenter.saving">更新 Presence</button></form>
      </details>

      <details class="governance-section">
        <summary class="section-heading"><span class="section-index">04</span><div><p class="eyebrow">MEMORY</p><h2>关系与记忆</h2></div><span class="disclosure-icon" aria-hidden="true">⌄</span></summary>
        <p v-if="!(controlCenter.fluctlightDetail.relationships as unknown[])?.length" class="field-note">尚未形成关系状态。</p>
        <ul v-else class="detail-list"><li v-for="relationship in controlCenter.fluctlightDetail.relationships as Array<Record<string, unknown>>" :key="String(relationship.target_actor_id)"><strong>{{ String(relationship.target_actor_id) }}</strong><small>{{ String(relationship.trend ?? "") }} · r{{ String(relationship.revision ?? "") }}</small><div class="inline-controls"><input v-model="controlCenter.relationshipRollbackTargets[String(relationship.target_actor_id)]" aria-label="关系回滚目标 revision" type="number" min="0" step="1" placeholder="目标 revision" /><button class="text-button" type="button" :disabled="controlCenter.saving" @click="controlCenter.rollbackRelationship(store.fluctlightId, relationship)">回滚关系</button></div></li></ul>
        <p v-if="!(controlCenter.fluctlightDetail.memories as unknown[])?.length" class="field-note">暂无可展示的记忆。</p>
        <ul v-else class="detail-list"><li v-for="memory in controlCenter.fluctlightDetail.memories as Array<Record<string, unknown>>" :key="String(memory.id)"><strong>{{ String(memory.content) }}</strong><div class="inline-controls"><input v-model="controlCenter.memoryEdits[String(memory.id)]" :aria-label="'修正记忆 ' + memory.id" maxlength="4096" placeholder="修正内容" /><button class="text-button" type="button" :disabled="controlCenter.saving" @click="controlCenter.reviseMemory(memory)">修正</button><button class="text-button danger-text" type="button" :disabled="controlCenter.saving" @click="controlCenter.forgetMemory(memory)">遗忘</button></div></li></ul>
        <label for="governance-evidence">证据引用</label><input id="governance-evidence" v-model="controlCenter.governanceEvidence" maxlength="4096" placeholder="以逗号分隔，例如 event_123, message_456" />
        <h3>近期认知</h3><p v-if="!(controlCenter.fluctlightDetail.cognition_history as unknown[])?.length" class="field-note">还没有完成的认知行动。</p><ul v-else class="detail-list"><li v-for="action in controlCenter.fluctlightDetail.cognition_history as Array<Record<string, unknown>>" :key="String(action.id)">{{ String(action.action_type) }}<small>{{ String(action.status) }}</small></li></ul>
      </details>

      <details class="governance-section">
        <summary class="section-heading"><span class="section-index">05</span><div><p class="eyebrow">AUTONOMY</p><h2>自治与修订</h2></div><span class="disclosure-icon" aria-hidden="true">⌄</span></summary>
        <p v-if="!controlCenter.autonomyActions.length" class="field-note">当前没有待治理的自治动作。</p>
        <ul v-else class="detail-list"><li v-for="action in controlCenter.autonomyActions" :key="action.id"><strong>{{ action.action_type }}</strong><small>{{ action.status }}</small><div class="inline-controls"><button v-if="action.status === 'frozen' || action.status === 'deferred'" class="text-button" type="button" :disabled="controlCenter.saving || !controlCenter.governanceReason.trim()" @click="controlCenter.governAutonomyAction(action.id, 'paused', store.fluctlightId)">暂停</button><button v-if="action.status === 'frozen' || action.status === 'deferred' || action.status === 'paused'" class="text-button" type="button" :disabled="controlCenter.saving || !controlCenter.governanceReason.trim()" @click="controlCenter.governAutonomyAction(action.id, 'cancelled', store.fluctlightId)">取消</button></div></li></ul>
        <h3>身份与人格修订记录</h3>
        <p v-if="!(controlCenter.fluctlightDetail.foundation_revisions as unknown[])?.length" class="field-note">还没有修订记录。</p>
        <ul v-else class="detail-list"><li v-for="revision in controlCenter.fluctlightDetail.foundation_revisions as Array<Record<string, unknown>>" :key="String(revision.id)"><strong>r{{ String(revision.revision) }} · {{ String(revision.source) }}</strong><small>{{ String(revision.status) }}<template v-if="revision.reason"> · {{ String(revision.reason) }}</template></small><div v-if="revision.status === 'proposed'" class="inline-controls"><button class="text-button" type="button" :disabled="controlCenter.saving || !controlCenter.revisionReason.trim()" @click="controlCenter.acceptFoundationRevision(store.fluctlightId, String(revision.id))">接受</button><button class="text-button" type="button" :disabled="controlCenter.saving || !controlCenter.revisionReason.trim()" @click="controlCenter.rejectFoundationRevision(store.fluctlightId, String(revision.id))">拒绝</button></div></li></ul>
        <form class="governance-form" @submit.prevent="controlCenter.submitFoundationRevision(store.fluctlightId)"><label for="revision-json">基础修订 JSON<textarea id="revision-json" v-model="controlCenter.revisionChangesJson" rows="4" placeholder='{"name":"新的名称"}' /></label><label for="revision-reason">修订原因<input id="revision-reason" v-model="controlCenter.revisionReason" maxlength="1024" /></label><button class="secondary-button" type="submit" :disabled="controlCenter.saving || !controlCenter.revisionChangesJson.trim() || !controlCenter.revisionReason.trim()">提出修订</button></form>
        <form class="governance-form" @submit.prevent="controlCenter.rollbackFoundationRevision(store.fluctlightId)"><label for="rollback-revision">回滚目标 revision<input id="rollback-revision" v-model="controlCenter.rollbackTargetRevision" type="number" min="0" step="1" /></label><button class="secondary-button" type="submit" :disabled="controlCenter.saving || !controlCenter.rollbackTargetRevision || !controlCenter.revisionReason.trim()">回滚到该 revision</button></form>
      </details>

      <section class="danger-zone" aria-labelledby="retirement-title">
        <p class="eyebrow">DANGEROUS ACTION</p><h2 id="retirement-title">删除摇光</h2>
        <p>删除会从实例目录中移除并停止活动；历史会话与治理记录会保留用于审计，不能恢复到活动状态。</p>
        <form class="governance-form" @submit.prevent="retire"><label for="retirement-reason">删除原因<input id="retirement-reason" v-model="retirementReason" maxlength="1024" required /></label><label for="retirement-confirmation">确认名称<input id="retirement-confirmation" v-model="retirementConfirmation" :aria-label="'输入 ' + store.selectedFluctlightName + ' 确认删除'" maxlength="256" :placeholder="'输入 ' + store.selectedFluctlightName + ' 确认'" required /></label><p class="field-note">需要完整输入“{{ store.selectedFluctlightName }}”后才能启用删除。</p><button class="danger-button" type="submit" :disabled="controlCenter.saving || !retirementReason.trim() || retirementConfirmation.trim() !== store.selectedFluctlightName">{{ controlCenter.saving ? "正在删除..." : "删除摇光" }}</button></form>
      </section>
    </template>
  </section>
</template>
