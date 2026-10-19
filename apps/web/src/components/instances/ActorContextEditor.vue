<script setup lang="ts">
import { ref, watch, computed, onBeforeUnmount } from 'vue';
import { BrowserClient, BrowserApiError, type BrowserActorContext } from '@fluctlight/browser-client';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ChevronDown, RefreshCw } from '@lucide/vue';
import FormField from '../product/FormField.vue';
import FeedbackNotice from '../product/FeedbackNotice.vue';
import AsyncState from '../product/AsyncState.vue';
import JsonViewer from '../product/JsonViewer.vue';
import { actorUserBackgroundFields, backgroundFromDraft, backgroundDraft } from '../../lib/actor-user-background';
import { validateFields } from '../../lib/form-validation';
import { focusField } from "../../composables/use-form-errors";
import { userError, shortId } from '../../lib/user-feedback';
import { randomId } from '../../random-id';
import { apiOrigin } from '../../runtime-config';

const props = defineProps<{ fluctlightId: string; actors: { id: string; label: string }[]; ownerActorId?: string | null; preferredActorId?: string; nativeControls?: boolean; humanOnly?: boolean }>();
const emit = defineEmits<{ saved: [fluctlightId: string, actorId: string] }>();
const client = new BrowserClient(apiOrigin);
const target = ref(''), context = ref<BrowserActorContext | null>(null), busy = ref(false), error = ref(''), notice = ref(''), reason = ref('');
const operation = ref<'correct' | 'change'>('correct');
const draft = ref<Record<string, string>>({}), fieldErrors = ref<Record<string, string>>({});
const ownerRevision = ref(''), dirty = ref(false);
const isOwner = computed(() => Boolean(props.ownerActorId) && target.value === props.ownerActorId);
const fields = actorUserBackgroundFields;
let epoch = 0, restoring = false;
let pending: { fingerprint: string; key: string } | null = null;

async function load(keepDraft = false) {
  const e = ++epoch, instanceId = props.fluctlightId, actorId = target.value, owner = isOwner.value;
  if (!actorId || !instanceId) { context.value = null; busy.value = false; return; }
  busy.value = true; error.value = '';
  try {
    const [value, detail] = await Promise.all([client.actorContext(instanceId, actorId), owner ? client.detail(instanceId) : Promise.resolve(null)]);
    if (e !== epoch || actorId !== target.value || instanceId !== props.fluctlightId) return;
    if (props.humanOnly && value.actor_type !== 'human') { context.value = null; throw new Error('当前交流身份不是人类参与者；“关于你”不会修改角色或 NPC 设定。'); }
    context.value = value;
    ownerRevision.value = owner ? String(detail?.current_facts_revision ?? '') : '';
    if (owner && detail?.actor_user) context.value = { ...value, background: detail.actor_user.background };
    if (!keepDraft) { restoring = true; draft.value = backgroundDraft(context.value.background); restoring = false; dirty.value = false; }
    pending = null;
  } catch (cause) { if (e === epoch) error.value = userError(cause, '无法读取背景资料，请重试。'); }
  finally { if (e === epoch) busy.value = false; }
}
async function save() {
  if (!context.value || busy.value) return;
  fieldErrors.value = validateFields([
    { id: 'actor-change-reason', label: '修改说明', value: reason.value, required: true, maxLength: isOwner.value ? 500 : undefined, maxBytes: isOwner.value ? undefined : 1000 },
    ...fields.filter(field => field.key !== 'meeting_confirmed').map(field => ({ id: `actor-field-${field.key}`, label: field.label, value: draft.value[field.key], kind: field.key === 'timezone' ? 'timezone' as const : undefined, maxLength: 1024 })),
  ]);
  if (Object.keys(fieldErrors.value).length) { await focusField(Object.keys(fieldErrors.value)[0]); return; }
  const e = epoch, instanceId = props.fluctlightId, actorId = target.value, owner = isOwner.value;
  const background = backgroundFromDraft(draft.value);
  const body = { background, operation: operation.value, reason: reason.value.trim() };
  const version = owner ? ownerRevision.value : context.value.context_version;
  if (!version) { error.value = '当前版本尚未读取，请重新读取后保存。'; return; }
  const fingerprint = JSON.stringify({ instanceId, actorId, owner, version, ...body });
  if (pending?.fingerprint !== fingerprint) pending = { fingerprint, key: randomId() };
  busy.value = true; error.value = ''; notice.value = '';
  try {
    if (owner) await client.updateActorUserBackground(instanceId, { ...body, expectedCurrentFactsRevision: version, idempotencyKey: pending!.key });
    else await client.updateActorContext(instanceId, actorId, { ...body, expectedContextVersion: version, idempotencyKey: pending!.key });
    if (e !== epoch || instanceId !== props.fluctlightId || actorId !== target.value) return;
    pending = null;
    await load();
    if (instanceId === props.fluctlightId && actorId === target.value && !error.value) {
      notice.value = '背景已保存并重新读取。相关目标与尚未启动的行动需要复核；资料修改不会被当作对方回应。';
      emit('saved', instanceId, actorId);
    }
  } catch (cause) {
    if (e === epoch) error.value = cause instanceof BrowserApiError && cause.status === 409 ? '资料或当前状态已变化，草稿已保留。请读取新版本后核对再保存。' : userError(cause, '无法保存背景资料，草稿已保留。');
  } finally { if (e === epoch) busy.value = false; }
}
function resetTarget() {
  epoch++; context.value = null; draft.value = {}; reason.value = ''; fieldErrors.value = {}; pending = null; notice.value = ''; error.value = ''; dirty.value = false;
}
watch(() => [props.fluctlightId, props.preferredActorId], () => { resetTarget(); target.value = props.actors.find(actor => actor.id === props.preferredActorId)?.id ?? props.actors.find(actor => actor.id === props.ownerActorId)?.id ?? props.actors[0]?.id ?? ''; }, { immediate: true });
watch(() => [props.fluctlightId, target.value], () => { resetTarget(); if (target.value) void load(); else busy.value = false; }, { immediate: true });
watch(() => props.actors, () => { if (!target.value && props.actors.length) target.value = props.actors.find(actor => actor.id === props.ownerActorId)?.id ?? props.actors[0]!.id; });
watch(draft, () => { if (!restoring) dirty.value = true; }, { deep: true, flush: 'sync' });
onBeforeUnmount(() => { epoch++; });
</script>
<template>
  <section class="product-section actor-context-editor" aria-label="背景资料与修订">
    <div class="planning-actions"><h3>背景资料</h3><span v-if="dirty" class="draft-indicator">有未保存修改</span></div>
    <p class="section-intro">这里编辑交流对象的资料。留空表示未知，时区不从设备或人格推断。</p>
    <FormField id="actor-target" label="交流对象" v-slot="{controlAttrs}"><select v-if="nativeControls" v-bind="controlAttrs" v-model="target" class="select" :disabled="busy"><option v-for="actor in actors" :key="actor.id" :value="actor.id">{{actor.label}}</option></select><Select v-else v-model="target"><SelectTrigger v-bind="controlAttrs" :disabled="busy"><SelectValue placeholder="选择对象" /></SelectTrigger><SelectContent><SelectItem v-for="actor in actors" :key="actor.id" :value="actor.id">{{actor.label}}</SelectItem></SelectContent></Select></FormField>
    <FeedbackNotice :message="context ? error : ''" /><FeedbackNotice :message="notice" tone="success" />
    <AsyncState :loading="busy && !context" :error="!context ? error : ''" retryable @retry="load()" :empty="!target" loading-label="正在读取背景资料…" empty-label="暂无可编辑对象">
      <form v-if="context" class="stack-form actor-background-form" novalidate @submit.prevent="save">
        <fieldset class="form-grid" :disabled="busy">
          <FormField v-for="field in fields" :key="field.key" :id="`actor-field-${field.key}`" :label="field.label" v-model:error="fieldErrors[`actor-field-${field.key}`]" :class="{ 'full-width': ['background','interests','preferences'].includes(field.key) }" v-slot="{controlAttrs}">
            <select v-if="field.key === 'meeting_confirmed' && nativeControls" v-bind="controlAttrs" v-model="draft[field.key]" class="select"><option value="unknown">未确认</option><option value="true">已约定</option><option value="false">未约定</option></select>
            <Select v-else-if="field.key === 'meeting_confirmed'" v-model="draft[field.key]"><SelectTrigger v-bind="controlAttrs"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="unknown">未确认</SelectItem><SelectItem value="true">已约定</SelectItem><SelectItem value="false">未约定</SelectItem></SelectContent></Select>
            <Textarea v-else-if="['background','interests','preferences'].includes(field.key)" v-bind="controlAttrs" v-model="draft[field.key]" maxlength="1024" rows="3" placeholder="未填写 / 未确认" />
            <Input v-else v-bind="controlAttrs" v-model="draft[field.key]" maxlength="1024" :placeholder="field.key === 'timezone' ? 'Asia/Shanghai，可留空' : '未填写 / 未确认'" />
          </FormField>
          <FormField id="actor-operation" label="这次修改的含义" class="full-width" v-slot="{controlAttrs}"><select v-if="nativeControls" v-bind="controlAttrs" v-model="operation" class="select"><option value="correct">纠正原信息：之前的内容不正确</option><option value="change">情况发生变化：之前正确，现在改变了</option></select><Select v-else v-model="operation"><SelectTrigger v-bind="controlAttrs"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="correct">纠正原信息：之前的内容不正确</SelectItem><SelectItem value="change">情况发生变化：之前正确，现在改变了</SelectItem></SelectContent></Select></FormField>
          <FormField id="actor-change-reason" label="修改说明" required v-model:error="fieldErrors['actor-change-reason']" class="full-width" v-slot="{controlAttrs}"><Input v-bind="controlAttrs" v-model="reason" :maxlength="isOwner ? 500 : 1000" placeholder="例如：补充资料，或生活安排发生变化" /></FormField>
        </fieldset>
        <div class="form-actions"><Button type="submit" :disabled="busy">{{busy ? '正在保存…' : '保存背景资料'}}</Button><Button type="button" variant="outline" :disabled="busy" @click="load(true)"><RefreshCw :size="16" aria-hidden="true" />读取新版本并保留草稿</Button><Button type="button" variant="ghost" :disabled="busy" @click="load(false)">放弃草稿并重新读取</Button></div>
      </form>
      <details v-if="context" class="product-disclosure actor-provenance"><summary><span>来源、版本与修订历史</span><ChevronDown :size="18" aria-hidden="true" /></summary><p class="field-hint">对象 {{shortId(context.actor_id)}} · {{isOwner ? '当前事实版本' : '背景版本'}} {{shortId(isOwner ? ownerRevision : context.context_version)}}</p><JsonViewer :value="{facts:context.facts,history:context.history,relationships:context.relationships,relationship_history:context.relationship_history}" label="查看完整来源与历史" /></details>
    </AsyncState>
  </section>
</template>
