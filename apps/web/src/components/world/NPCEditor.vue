<script setup lang="ts">
import { reactive, ref, watch } from 'vue';
import type { NPCWriteBody, NPCRoutineSlot, RuntimeNPC, RuntimePlace } from '@fluctlight/browser-client';

const props = defineProps<{ npc: RuntimeNPC; places: RuntimePlace[]; busy: boolean; save: (body: Omit<NPCWriteBody, 'idempotencyKey'>) => Promise<RuntimeNPC | null> }>();
const draft = reactive({ name: '', role: '', background: '', homePlaceId: '', status: 'active' as 'active' | 'archived', routine: [] as NPCRoutineSlot[], reason: '' });
const revision = ref(0), dirty = ref(false);
function reset() { Object.assign(draft, { name: props.npc.name, role: props.npc.role, background: props.npc.bio, homePlaceId: props.npc.home_place_id || '', status: props.npc.status, routine: props.npc.routine.map(slot => ({ ...slot })), reason: '' }); revision.value = props.npc.revision; dirty.value = false; }
watch(() => props.npc.id, reset, { immediate: true });
async function save() {
  const result = await props.save({ ...draft, homePlaceId: draft.homePlaceId || null, expectedRevision: revision.value });
  if (result) { Object.assign(draft, { name: result.name, role: result.role, background: result.bio, homePlaceId: result.home_place_id || '', routine: result.routine.map(slot => ({ ...slot })), status: result.status, reason: '' }); revision.value = result.revision; dirty.value = false; }
}
</script>
<template>
  <form class="card stack" @submit.prevent="save" @input="dirty = true" @change="dirty = true">
    <div class="row"><h3 class="grow">编辑运行人物</h3><span v-if="dirty" class="small muted">有未保存修改</span><span class="badge">版本 {{ revision }}</span></div>
    <p class="small muted">这里是当前运行世界中的局部修正，不会修改原始定义，也不代表已经结识。</p>
    <div class="form-grid">
      <label class="field">姓名<input v-model="draft.name" class="input" required maxlength="256"></label>
      <label class="field">身份 / 简介<input v-model="draft.role" class="input" maxlength="512"></label>
      <label class="field full-width">背景<textarea v-model="draft.background" class="input" rows="3" maxlength="16000" /></label>
      <label class="field">初始 / 默认地点<select v-model="draft.homePlaceId" class="select"><option value="">未设定</option><option v-for="p in places.filter(p => p.status === 'active')" :key="p.id" :value="p.id">{{ p.name }}</option></select></label>
      <label class="field">档案状态<select v-model="draft.status" class="select"><option value="active">启用</option><option value="archived">归档（保留全部历史引用）</option></select></label>
    </div>
    <h3>生活规律</h3><p class="small muted">规律只用于推算计划位置；不会成为已确认在场。结束早于开始表示跨午夜。</p>
    <div v-for="(slot, index) in draft.routine" :key="index" class="card stack">
      <div class="form-grid"><label class="field">适用日<select v-model="slot.day" class="select"><option value="daily">每天</option><option value="weekday">工作日</option><option value="weekend">周末</option></select></label><label class="field">地点<select v-model="slot.place_id" class="select" required><option value="" disabled>选择地点</option><option v-for="p in places.filter(p => p.status === 'active')" :key="p.id" :value="p.id">{{ p.name }}</option></select></label><label class="field">开始<input v-model="slot.start" class="input" type="time" required></label><label class="field">结束<input v-model="slot.end" class="input" type="time" required></label></div>
      <label class="field">活动<input v-model="slot.activity" class="input" required maxlength="512"></label><button type="button" class="btn ghost" @click="draft.routine.splice(index, 1); dirty = true">移除此时段</button>
    </div>
    <button type="button" class="btn" :disabled="draft.routine.length >= 42" @click="draft.routine.push({ day: 'daily', start: '09:00', end: '17:00', place_id: '', activity: '' }); dirty = true">添加规律时段</button>
    <label class="field">修改原因<input v-model="draft.reason" class="input" required maxlength="1000"></label>
    <div class="row"><button class="btn primary" :disabled="busy" type="submit">{{ busy ? '正在保存…' : '保存运行人物' }}</button><button class="btn" type="button" :disabled="busy" @click="reset">放弃草稿</button><button class="btn ghost" type="button" :disabled="busy" @click="revision = npc.revision">使用已读取的新版本（保留草稿）</button></div>
  </form>
</template>
