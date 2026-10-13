<script setup lang="ts">
import { computed, ref } from "vue";
import { ChevronDown } from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import FormField from "../product/FormField.vue";
import FeedbackNotice from "../product/FeedbackNotice.vue";
import JsonViewer from "../product/JsonViewer.vue";
import { useControlCenterStore } from "../../stores/control-center";
import { useFormErrors } from "../../composables/use-form-errors";
import { autonomySettingsPatch, configObject } from "../../lib/configuration-forms";
const store = useControlCenterStore();
const base = ref(configObject(store.settings?.values["product.autonomy"]));
const mode = ref(String(base.value.mode ?? "active"));
const actions = ref<string[]>(Array.isArray(base.value.allowed_actions) ? base.value.allowed_actions.map(String) : ["proactive_message", "moment", "capability"]);
const budget = ref(Number(base.value.budget_remaining ?? 100));
const topicSuppression = ref(base.value.topic_suppression_seconds === undefined ? "" : String(base.value.topic_suppression_seconds));
const wake = configObject(store.settings?.values["product.wakeup"]), queue = configObject(store.settings?.values["llm.queue"]);
const wakeEnabled = ref(wake.enabled !== false), generatedConcurrency = ref(Number(queue.generated_concurrency ?? 1)), embeddingConcurrency = ref(Number(queue.embedding_concurrency ?? 1));
const notice = ref(""), error = ref(""), advancedJson = ref(""), advancedError = ref("");
const { errors, validate } = useFormErrors();
const actionLabels: Record<string,string> = { proactive_message: "主动发起消息", moment: "发布动态", capability: "请求新能力", memory_candidate: "记录候选记忆", relationship_candidate: "提出关系修订", schedule_proposal: "提出日程", media_request: "请求媒体生成" };
const actionOptions = computed(() => [...new Set([...Object.keys(actionLabels), ...actions.value])]);
function currentAutonomy() { return autonomySettingsPatch(base.value, { mode: mode.value, actions: actions.value, budget: Number(budget.value), ...(topicSuppression.value.trim() ? {topicSuppression: Number(topicSuppression.value)} : {}) }); }
async function save() {
  if(store.saving)return;
  error.value = ""; notice.value = "";
  if (!await validate([
    {id:"autonomy-budget",label:"剩余预算",value:budget.value,required:true,kind:"number",min:0},
    {id:"autonomy-topic",label:"重复话题冷却",value:topicSuppression.value,kind:"number",integer:true,min:300,max:604800},
    {id:"queue-generated",label:"生成式并发",value:generatedConcurrency.value,required:true,kind:"number",integer:true,min:1,max:8},
    {id:"queue-embedding",label:"Embedding并发",value:embeddingConcurrency.value,required:true,kind:"number",integer:true,min:1,max:8},
  ])) return;
  if (!actions.value.length) { error.value = "请至少保留一种允许行为。需要停止自主行动时，可选择暂停自主性。"; return; }
  await store.saveSettings({
    "product.autonomy": currentAutonomy(),
    "product.wakeup": { enabled: wakeEnabled.value, interval_seconds: 600 },
    "llm.queue": { generated_concurrency: Number(generatedConcurrency.value), embedding_concurrency: Number(embeddingConcurrency.value) },
  });
  error.value = store.settingsError;
  if (!error.value) { base.value = configObject(store.settings?.values["product.autonomy"]); notice.value = "运行策略已保存，后续任务使用新配置。"; }
}
function onAdvancedToggle(event: Event) { if ((event.target as HTMLDetailsElement).open) advancedJson.value = JSON.stringify(currentAutonomy(), null, 2); }
function applyAdvanced() {
  try {
    const parsed: unknown = JSON.parse(advancedJson.value);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) throw new Error();
    const value = configObject(parsed);
    if (value.mode !== undefined && typeof value.mode !== "string") throw new Error();
    if (value.allowed_actions !== undefined && (!Array.isArray(value.allowed_actions) || !value.allowed_actions.every(item=>typeof item==='string'))) throw new Error();
    base.value = value; mode.value = String(value.mode ?? "active"); actions.value = Array.isArray(value.allowed_actions) ? value.allowed_actions.map(String) : actions.value;
    budget.value = Number(value.budget_remaining ?? budget.value); topicSuppression.value = value.topic_suppression_seconds === undefined ? "" : String(value.topic_suppression_seconds);
    advancedError.value = ""; notice.value = "高级配置已应用到草稿，保存后生效。";
  } catch { advancedError.value = "请填写合法配置对象；mode 必须是文本，allowed_actions 必须是文本数组。"; }
}
</script>
<template>
  <form class="stack-form operational-settings" novalidate @submit.prevent="save">
    <FeedbackNotice :message="error" /><FeedbackNotice :message="notice" tone="success" />
    <p class="section-intro">这些运行策略适用于全部实例。修改会影响后续任务，已发生的记录会保留。</p>
    <fieldset class="form-grid" :disabled="store.saving">
      <FormField id="autonomy-mode" label="自主性" v-slot="{controlAttrs}"><Select v-model="mode"><SelectTrigger v-bind="controlAttrs"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="active">允许自主行动</SelectItem><SelectItem value="paused">暂停自主性</SelectItem><SelectItem v-if="!['active','paused'].includes(mode)" :value="mode">当前策略（{{mode}}）</SelectItem></SelectContent></Select></FormField>
      <FormField id="autonomy-budget" label="剩余行动预算" required v-model:error="errors['autonomy-budget']" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model.number="budget" type="number" min="0" /></FormField>
      <fieldset class="checkbox-group full-width"><legend>允许的行为</legend><div class="option-grid"><label v-for="action in actionOptions" :key="action" class="checkbox-row"><input v-model="actions" type="checkbox" :value="action" /><span>{{actionLabels[action] ?? `已有自定义行为：${action}`}}</span></label></div></fieldset>
      <label class="checkbox-row full-width"><input v-model="wakeEnabled" type="checkbox" /><span>定期唤醒<span class="field-hint"> · 聊天结束后，每 10 分钟检查一次</span></span></label>
      <FormField id="autonomy-topic" label="重复话题冷却（秒）" v-model:error="errors['autonomy-topic']" hint="可选；范围为 5 分钟至 7 天。留空保留原配置。" class="full-width" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="topicSuppression" type="number" min="300" max="604800" /></FormField>
      <FormField id="queue-generated" label="生成式模型并发" required v-model:error="errors['queue-generated']" hint="1–8 个请求" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model.number="generatedConcurrency" type="number" min="1" max="8" /></FormField>
      <FormField id="queue-embedding" label="Embedding 并发" required v-model:error="errors['queue-embedding']" hint="1–8 个请求" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model.number="embeddingConcurrency" type="number" min="1" max="8" /></FormField>
    </fieldset>
    <section class="effective-policy"><h3>诊断保留</h3><p>当前清理规则：保留 30 天，每类最多 10,000 条。</p><p class="field-hint">当前版本采用固定规则，此处只读。历史资料、关系修订和业务运行记录不属于诊断清理范围。</p><JsonViewer :value="store.settings?.values['diagnostics.retention'] ?? {}" label="查看已存储的保留配置（当前不影响清理）" /></section>
    <details class="product-disclosure" @toggle="onAdvancedToggle"><summary><span>高级自治配置</span><ChevronDown :size="18" aria-hidden="true" /></summary><p class="field-hint">用于维护常用表单之外的已有配置。应用到草稿后，核对表单再保存。</p><FormField id="autonomy-advanced" label="自治配置 JSON" v-model:error="advancedError" v-slot="{controlAttrs}"><Textarea v-bind="controlAttrs" v-model="advancedJson" rows="10" spellcheck="false" /></FormField><Button type="button" variant="outline" :disabled="store.saving" @click="applyAdvanced">应用到草稿</Button></details>
    <div class="form-actions"><Button type="submit" :disabled="store.saving">{{store.saving ? '正在保存…' : '保存运行策略'}}</Button></div>
  </form>
</template>
