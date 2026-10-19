<script setup lang="ts">
import { computed, onMounted, reactive, ref, toRef, watch } from 'vue';
import type { ApplyPreviewResponse, NPCWriteBody, RuntimePlace } from '@fluctlight/browser-client';
import { useWorldWorkspace } from '../../composables/use-world-workspace';
import NPCEditor from '../../components/world/NPCEditor.vue';
import FeedbackNotice from '../../components/product/FeedbackNotice.vue';
import JsonViewer from '../../components/product/JsonViewer.vue';
import { zonedLocalInputToISO } from '../../lib/zoned-local-input';

const props = defineProps<{ fluctlightId: string; view?: string; npcId?: string; placeId?: string }>();
const emit = defineEmits<{ navigate: [path: string] }>();
const { client, snapshot, definitions, selectedNPC, loading, busy, error, definitionError, notice, load, selectNPC, mutate } = useWorldWorkspace(toRef(props, 'fluctlightId'));
const q = ref(''), status = ref('active'), definitionId = ref(''), applyHome = ref('');
const worldDirty = ref(false), worldDraft = reactive({ name: '', background: '', rules: '', reason: '', revision: 0 });
const placeDraft = reactive({ id: '', name: '', description: '', parentPlaceId: '', status: 'active' as 'active' | 'archived', revision: 0, reason: '' });
const definitionDraft = reactive({ name: '', role: '', background: '', reason: '' });
const preview = ref<ApplyPreviewResponse | null>(null), choices = reactive<Record<string, 'keep' | 'override' | ''>>({});
const encounter = reactive({ summary: '', start: '', end: '', place: '', evidence: '', evidenceMode: 'records', reason: '', confirmed: false });
const detail = ref<Record<string, unknown> | null>(null);
const npcDefinitions = computed(() => definitions.value.filter(item => item.kind === 'npc'));
const selectedDefinition = computed(() => npcDefinitions.value.find(item => item.id === definitionId.value));
const people = computed(() => (snapshot.value?.npcs || []).filter(n => (status.value === 'all' || n.status === status.value) && (!q.value || `${n.name} ${n.role}`.includes(q.value))));
const context = computed(() => snapshot.value?.current_context);
const timezone = computed(() => typeof context.value?.timezone === 'string' ? context.value.timezone : '');
const places = computed(() => snapshot.value?.places || []);
const placeName = (id: string | null | undefined) => places.value.find(p => p.id === id)?.name || id || '未设定';
const base = computed(() => `#/c/${props.fluctlightId}`);
function resetWorld() { const value = snapshot.value?.world; Object.assign(worldDraft, { name: value?.name || '', background: value?.background || '', rules: value?.rules || '', revision: value?.revision ?? 0, reason: '' }); worldDirty.value = false; }
watch(snapshot, () => { if (!worldDirty.value) resetWorld(); });
watch(() => props.npcId, id => { void selectNPC(id || ''); }, { immediate: true });
watch(definitionId, () => { preview.value = null; const item = selectedDefinition.value; Object.assign(definitionDraft, { name: item?.name || '', role: String(item?.body?.role || ''), background: String(item?.body?.background || ''), reason: '' }); });
async function refresh() { await load(); if (props.npcId) await selectNPC(props.npcId); }
async function saveWorld() { const body = { name: worldDraft.name, background: worldDraft.background, rules: worldDraft.rules, expectedRevision: worldDraft.revision, reason: worldDraft.reason }; if (await mutate('world', body, key => client.updateRuntimeWorld(props.fluctlightId, { ...body, idempotencyKey: key }))) resetWorld(); }
function editPlace(place?: RuntimePlace) { Object.assign(placeDraft, { id: place?.id || '', name: place?.name || '', description: place?.desc || '', parentPlaceId: place?.parent_place_id || '', status: place?.status || 'active', revision: place?.revision ?? 0, reason: '' }); }
async function savePlace() { const body = { name: placeDraft.name, description: placeDraft.description, parentPlaceId: placeDraft.parentPlaceId || null, status: placeDraft.status, expectedRevision: placeDraft.revision, reason: placeDraft.reason }; const result = await mutate(`place:${placeDraft.id}`, body, key => placeDraft.id ? client.updateWorldPlace(props.fluctlightId, placeDraft.id, { ...body, idempotencyKey: key }) : client.createWorldPlace(props.fluctlightId, { ...body, idempotencyKey: key })); if (result) editPlace(places.value.find(p => p.id === result.id)); }
async function saveDefinition() {
  const current = selectedDefinition.value, body = { kind: 'npc' as const, name: definitionDraft.name, body: { role: definitionDraft.role, background: definitionDraft.background }, reason: definitionDraft.reason, ...(current ? { expectedVersion: current.draft_version || current.published_version || undefined } : {}) };
  const saved = await mutate(`definition:${current?.id || 'new'}`, body, async key => { const result = current ? await client.updateStudioResource(current.id, { ...body, idempotencyKey: key }) : await client.createStudioResource({ ...body, idempotencyKey: key }); return client.studioResource(result.id); });
  if (saved) { definitionId.value = saved.id; preview.value = null; notice.value = '草稿已保存并重新读取。尚未发布或应用到任何世界。'; }
}
async function publishDefinition() { const item = selectedDefinition.value; if (!item) return; const body = { expectedRevision: item.revision, reason: definitionDraft.reason }; const result = await mutate(`publish:${item.id}`, body, key => client.publishStudioResource(item.id, { ...body, idempotencyKey: key })); if (result) notice.value = `定义版本 ${result.published_version} 已发布；运行人物尚未改变。`; }
async function previewApply() { const item = selectedDefinition.value; if (!item?.published_version) return; try { preview.value = await client.previewStudioApply(item.id, { fluctlightId: props.fluctlightId, resourceVersion: item.published_version }); for (const row of preview.value.diff) choices[row.field] = ''; } catch (cause) { error.value = cause instanceof Error ? cause.message : '无法预览应用'; } }
async function applyDefinition() {
  const item = selectedDefinition.value, observed = preview.value; if (!item?.published_version || !observed || typeof observed.runtime_revision !== 'number') return;
  if (observed.diff.some(row => !choices[row.field])) { error.value = '请为每个差异选择保留运行值或采用定义。'; return; }
  const body = { fluctlightId: props.fluctlightId, resourceVersion: item.published_version, expectedInstanceRevision: observed.instance_revision, expectedRuntimeRevision: observed.runtime_revision, homePlaceId: applyHome.value || null, choices: observed.diff.map(row => ({ field: row.field, decision: choices[row.field] as 'keep' | 'override' })), reason: definitionDraft.reason };
  const result = await mutate(`apply:${item.id}`, body, key => client.applyStudioResource(item.id, { ...body, idempotencyKey: key }));
  if (result?.applied && result.id) { preview.value = null; notice.value = '已应用并读回运行人物。创建不代表已经结识。'; emit('navigate', `${base.value}/people/npc/${result.id}`); }
}
async function saveNPC(body: Omit<NPCWriteBody, 'idempotencyKey'>) { const id = selectedNPC.value?.id; if (!id) return null; const result = await mutate(`npc:${id}`, body, key => client.updateRuntimeNPC(props.fluctlightId, id, { ...body, idempotencyKey: key })); if (!result) return null; await selectNPC(id); return selectedNPC.value; }
async function recordEncounter() {
  if (!selectedNPC.value || !encounter.confirmed || !timezone.value) return;
  try {
    const current = await client.detail(props.fluctlightId), life = current.context as Record<string, unknown> | undefined;
    if (typeof life?.context_revision !== 'string') throw new Error('生活上下文版本未就绪，请重新读取。');
    const body = { confirmed: true as const, summary: encounter.summary, startAt: zonedLocalInputToISO(encounter.start, timezone.value), endAt: zonedLocalInputToISO(encounter.end, timezone.value), placeId: encounter.place, evidenceRefs: encounter.evidenceMode === 'owner_confirmation' ? ['owner_confirmation'] : encounter.evidence.split(',').map(value => value.trim()).filter(Boolean), expectedLifeContextRevision: life.context_revision, reason: encounter.reason };
    const id = selectedNPC.value.id, result = await mutate(`encounter:${id}`, body, key => client.recordNPCEncounter(props.fluctlightId, id, { ...body, idempotencyKey: key }));
    if (result) { await selectNPC(id); notice.value = `已记录接触 ${result.id}，事件、关系与记忆均已读回。`; encounter.confirmed = false; }
  } catch (cause) { error.value = cause instanceof Error ? cause.message : '接触记录未保存'; }
}
onMounted(async () => { try { detail.value = await client.detail(props.fluctlightId); } catch (cause) { error.value = cause instanceof Error ? cause.message : '事件来源读取失败'; } });
</script>

<template>
  <section class="stack">
    <div class="banner info">浏览世界不会移动角色，也不表示你在场。定义、计划位置、上次确认地点和实际关系分别记录。</div>
    <nav class="tabs" aria-label="世界与人物"><a class="tab" :href="`${base}/world`">世界与地点</a><a class="tab" :href="`${base}/world/scene`">当前场景</a><a class="tab" :href="`${base}/world/events`">事件</a><a class="tab" :href="`${base}/people/list`">人物 / NPC</a><a class="tab" :href="`${base}/people/owner`">关于你</a></nav>
    <FeedbackNotice :message="error" /><FeedbackNotice :message="definitionError" /><FeedbackNotice :message="notice" tone="success" />
    <div class="row"><span class="small muted grow">{{ snapshot?.world ? `${snapshot.world.name} · 运行世界` : '尚未关联运行世界' }}</span><button class="btn" :disabled="loading || busy" @click="refresh">{{ loading ? '正在读取…' : '重新读取（保留草稿）' }}</button></div>

    <template v-if="view === 'scene'">
      <div class="card"><h2>当前有效生活上下文</h2><p>{{ context?.activity || '活动尚未确认' }}</p><p>地点：{{ context?.location || '尚未确认' }}</p><p class="small muted">来源 {{ context?.source }} · {{ context?.timezone }}。自由文本地点不会按同名自动关联目录。</p><a class="btn" :href="`${base}/life/now`">完整生活状态</a></div>
    </template>
    <template v-else-if="view === 'events'">
      <div class="card"><h2>已记录事件</h2><p v-if="!Array.isArray(detail?.events)" class="small muted">事件尚在读取，失败时请重试。</p><ul v-else class="stack"><li v-for="event in (detail.events as Record<string, unknown>[])" :key="String(event.id)"><b>{{ event.activity || event.kind }}</b><p class="small muted">{{ event.start_at }} — {{ event.end_at }} · {{ event.status }}</p><JsonViewer :value="event" label="来源、版本与结果" /></li></ul><p v-if="Array.isArray(detail?.events) && !detail.events.length" class="small muted">尚无事件。可从日程记录确认事件，或在 NPC 详情中记录有依据的接触。</p><a class="btn" :href="`${base}/life/schedule`">打开日程与事件</a></div>
    </template>
    <template v-else-if="view === 'people' || view === 'npc'">
      <div class="row"><label class="field grow">搜索当前世界人物<input v-model="q" class="input" placeholder="姓名或身份"></label><label class="field">状态<select v-model="status" class="select"><option value="active">启用</option><option value="archived">已归档</option><option value="all">全部</option></select></label></div>
      <div class="grid g2"><div class="card"><h3>运行世界人物</h3><a v-for="npc in people" :key="npc.id" class="lrow" :href="`${base}/people/npc/${npc.id}`"><b>{{ npc.name }}</b><span class="grow small muted">{{ npc.role }} · {{ npc.status === 'archived' ? '已归档' : '启用' }}</span></a><p v-if="!people.length" class="small muted">当前范围无人物。先保存并发布下方 NPC 定义，再应用到此运行世界。</p></div>
        <div v-if="selectedNPC" class="card"><h2>{{ selectedNPC.name }}</h2><p>{{ selectedNPC.role }}</p><p>{{ selectedNPC.bio }}</p><p class="small muted">初始地点：{{ placeName(selectedNPC.home_place_id) }}</p><p v-if="selectedNPC.planned_position" class="small muted">{{ selectedNPC.planned_position.source === 'schedule_prediction' ? '日程推算（非在场事实）' : '初始位置（非在场事实）' }}：{{ placeName(selectedNPC.planned_position.place_id) }} {{ selectedNPC.planned_position.activity }}</p><p v-if="selectedNPC.confirmed_at">上次确认地点：{{ placeName(selectedNPC.confirmed_place_id) }} · {{ selectedNPC.confirmed_at }}（不保证此刻仍在）</p><p class="small muted">人物 {{ selectedNPC.id }} · Actor {{ selectedNPC.actor_id }} · 定义 {{ selectedNPC.source_resource_id }} / {{ selectedNPC.source_version }}</p><JsonViewer :value="selectedNPC.relationship_context" label="关系、印象与修订来源（独立于人物设定）" /></div></div>
      <template v-if="selectedNPC"><NPCEditor :npc="selectedNPC" :places="places" :busy="busy" :save="saveNPC" />
        <div class="card"><h3>实际接触记录</h3><p v-if="!selectedNPC.encounters?.length" class="small muted">尚无接触记录，创建或放置 NPC 不代表已经认识。</p><article v-for="entry in selectedNPC.encounters" :key="entry.id" class="lrow"><div class="grow"><b>{{ entry.summary }}</b><p class="small muted">{{ entry.occurred_at }} · {{ placeName(entry.place_id) }}</p><JsonViewer :value="entry" label="事件、关系、记忆与证据引用" /></div></article></div>
        <form v-if="selectedNPC.status === 'active'" class="card stack" @submit.prevent="recordEncounter"><h3>记录已发生的接触</h3><p class="small muted">所有者确认的历史记录，不是要求角色立刻前往或模型演示。只接受已结束、具备来源的接触。时间按 {{ timezone || '未读取的时区' }} 输入。</p><label class="field">经过<input v-model="encounter.summary" class="input" required maxlength="2000"></label><div class="form-grid"><label class="field">开始<input v-model="encounter.start" class="input" type="datetime-local" required></label><label class="field">结束<input v-model="encounter.end" class="input" type="datetime-local" required></label></div><label class="field">地点<select v-model="encounter.place" class="select" required><option value="" disabled>选择地点</option><option v-for="place in places.filter(p => p.status === 'active')" :key="place.id" :value="place.id">{{ place.name }}</option></select></label><label class="field">来源方式<select v-model="encounter.evidenceMode" class="select"><option value="records">引用已有消息或事件</option><option value="owner_confirmation">我的直接确认（保留所有者审计）</option></select></label><label v-if="encounter.evidenceMode === 'records'" class="field">证据引用<input v-model="encounter.evidence" class="input" required placeholder="实际消息、事件或经确认的来源 ID，逗号分隔"></label><label class="field">记录原因<input v-model="encounter.reason" class="input" required maxlength="1000"></label><label class="toggle"><input v-model="encounter.confirmed" type="checkbox" required> 我确认这次接触已经发生，允许形成有来源的关系和记忆</label><button class="btn primary" :disabled="busy || !timezone">保存接触并查询结果</button></form>
      </template>
      <section class="card stack"><h2>NPC 定义与应用</h2><p class="small muted">保存草稿、发布定义、应用到运行世界是三个独立动作。定义可复用，各运行世界中的人物身份互不共享。</p>
        <label class="field">选择定义<select v-model="definitionId" class="select"><option value="">新建 NPC 定义</option><option v-for="item in npcDefinitions" :key="item.id" :value="item.id">{{ item.name }} · {{ item.published_version ? `已发布 ${item.published_version}` : '草稿' }}</option></select></label>
        <form class="stack" @submit.prevent="saveDefinition"><label class="field">姓名<input v-model="definitionDraft.name" class="input" required maxlength="200"></label><label class="field">身份 / 简介<input v-model="definitionDraft.role" class="input" maxlength="512"></label><label class="field">背景<textarea v-model="definitionDraft.background" class="input" rows="3" maxlength="16000" /></label><label class="field">修改 / 发布 / 应用原因<input v-model="definitionDraft.reason" class="input" required maxlength="1000"></label><div class="row"><button class="btn" :disabled="busy">保存定义草稿</button><button v-if="selectedDefinition" type="button" class="btn primary" :disabled="busy || !definitionDraft.reason.trim()" @click="publishDefinition">发布已保存版本</button><button v-if="selectedDefinition?.published_version" type="button" class="btn" :disabled="busy || !snapshot?.world" @click="previewApply">预览应用到当前世界</button></div></form>
        <div v-if="preview" class="stack"><h3>应用预览</h3><p class="small muted">运行对象版本 {{ preview.runtime_revision }} · 新建不产生关系或生活事件。</p><div v-for="row in preview.diff" :key="row.field" class="card"><b>{{ row.field }}</b><p>当前：{{ row.from }}</p><p>定义：{{ row.to }}</p><select v-model="choices[row.field]" class="select" required><option value="" disabled>选择处理方式</option><option value="keep">保留运行值</option><option value="override">采用定义值</option></select></div><label class="field">新建人物的初始地点<select v-model="applyHome" class="select"><option value="">未设定</option><option v-for="place in places.filter(p => p.status === 'active')" :key="place.id" :value="place.id">{{ place.name }}</option></select></label><button class="btn primary" :disabled="busy || !definitionDraft.reason.trim()" @click="applyDefinition">确认应用并查询运行人物</button></div>
      </section>
    </template>
    <template v-else>
      <section class="grid g2"><form class="card stack" @submit.prevent="saveWorld" @input="worldDirty = true"><h2>{{ snapshot?.world ? '运行世界设定' : '创建运行世界' }}</h2><label class="field">世界名称<input v-model="worldDraft.name" class="input" required maxlength="256"></label><label class="field">背景<textarea v-model="worldDraft.background" class="input" rows="3" maxlength="16000" /></label><label class="field">生活规则<textarea v-model="worldDraft.rules" class="input" rows="3" maxlength="8000" /></label><p class="small muted">永久性伤害事件继续禁用。保存这里不会把初始地点变成当前事实。</p><label class="field">原因<input v-model="worldDraft.reason" class="input" required maxlength="1000"></label><div class="row"><button class="btn primary" :disabled="busy">保存运行世界</button><button class="btn" type="button" :disabled="busy" @click="resetWorld">放弃草稿并采用已读取版本</button></div></form>
        <div class="card"><h2>地点目录</h2><button class="btn" :disabled="!snapshot?.world" @click="editPlace()">新增地点</button><article v-for="place in places" :key="place.id" class="lrow"><div class="grow"><b>{{ place.name }}</b><p class="small muted">{{ place.desc }} · {{ place.status === 'archived' ? '已归档' : '启用' }}</p><p class="small muted">{{ place.id }}</p></div><button class="btn sm" @click="editPlace(place)">查看 / 编辑</button></article><p v-if="snapshot?.world && !places.length" class="small muted">尚无地点。可新增地点，然后设置人物的初始地点与生活规律。</p></div></section>
      <form v-if="snapshot?.world" class="card stack" @submit.prevent="savePlace"><h3>{{ placeDraft.id ? '编辑地点' : '新增地点' }}</h3><div class="form-grid"><label class="field">名称<input v-model="placeDraft.name" class="input" required maxlength="256"></label><label class="field">上级地点<select v-model="placeDraft.parentPlaceId" class="select"><option value="">无</option><option v-for="p in places.filter(p => p.id !== placeDraft.id && p.status === 'active')" :key="p.id" :value="p.id">{{ p.name }}</option></select></label></div><label class="field">描述<textarea v-model="placeDraft.description" class="input" rows="3" maxlength="8000" /></label><label v-if="placeDraft.id" class="field">状态<select v-model="placeDraft.status" class="select"><option value="active">启用</option><option value="archived">归档（引用中的地点会被保护）</option></select></label><label class="field">原因<input v-model="placeDraft.reason" class="input" required maxlength="1000"></label><button class="btn primary" :disabled="busy">保存地点并重新查询</button></form>
    </template>
  </section>
</template>
