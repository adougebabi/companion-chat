<script setup lang="ts">
import { computed } from 'vue';
import Avatar from '../Avatar.vue';
import type { ContactGroup, PersonaDetailData } from '../types';

const props = withDefaults(defineProps<{ detail: PersonaDetailData; groups?: ContactGroup[]; loading?: boolean }>(), { groups: () => [], loading: false });
const emit = defineEmits<{
  (event: 'close'): void;
  (event: 'chat', id: string): void;
  (event: 'activity', id: string): void;
  (event: 'delete', id: string): void;
  (event: 'screen', id: string): void;
  (event: 'save-group', id: string, groupId: string | null): void;
  (event: 'save-policy', id: string, policy: string): void;
  (event: 'delete-memory', personaId: string, memoryId: string): void;
  (event: 'rollback', personaId: string, evolutionId: string): void;
  (event: 'restore-foundation', personaId: string, revisionId: string): void;
  (event: 'edit-foundation', id: string): void;
  (event: 'reschedule', id: string, scheduleId: string): void;
  (event: 'cancel-schedule', id: string, scheduleId: string): void;
  (event: 'hidden-activities', id: string): void;
}>();

const state = computed(() => props.detail.state || {});
const sourceText = computed(() => typeof state.value.source === 'string' ? state.value.source : state.value.source?.label || '当前生活状态');
const foundationLines = computed(() => (props.detail.foundation || '').split(/\r?\n/).filter(Boolean));
</script>

<template>
  <section class="detail-sheet persona-detail">
    <header><div class="detail-heading"><Avatar :persona="detail" /><span><small>FLUCTLIGHT INSTANCE</small><h2 id="persona-detail-title">{{ detail.name }}</h2><p>{{ detail.role }}</p></span></div><button class="close-dialog" type="button" aria-label="关闭详情" @click="emit('close')">×</button></header>
    <div v-if="loading" class="detail-scroll"><div class="loading-state" role="status">正在加载详情…</div></div>
    <div v-else class="detail-scroll">
      <section><h3>现在</h3><p class="state-line">{{ state.situation || detail.currentSituation || '正在过自己的日常' }}<small>{{ state.mood || detail.mood || '平静' }}</small></p><p class="state-source">{{ state.scene || state.room || '日常场景' }}<template v-if="state.location"> · {{ state.location }}</template></p><p class="state-source">{{ sourceText }}</p><p v-if="state.appearance && Object.keys(state.appearance).length" class="state-source">当前外观：{{ Object.values(state.appearance).join(' · ') }}</p></section>
      <section><h3>近期安排</h3><ul v-if="detail.schedule?.length" class="schedule-list"><li v-for="item in detail.schedule" :key="item.id"><span><b>{{ item.title }}</b><small>{{ item.startsAt ? new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(new Date(item.startsAt)) : '' }}</small></span><button class="schedule-reschedule" type="button" aria-label="改期" @click="emit('reschedule', detail.id, item.id)">↻</button><button class="schedule-cancel" type="button" aria-label="取消这项安排" @click="emit('cancel-schedule', detail.id, item.id)">×</button></li></ul><p v-else class="muted">暂无公开的近期安排</p></section>
      <section><h3>生活设定</h3><p class="detail-text">{{ foundationLines.join('\n') || '还没有记录身份核心。' }}</p><p class="state-source">{{ detail.inferredFields?.length ? `AI 推断：${detail.inferredFields.join('、')}` : '所有初始设定均由你提供' }}</p><button class="quiet" type="button" @click="emit('edit-foundation', detail.id)">修订身份核心</button><ul v-if="detail.foundationRevisions?.length && detail.foundationRevisions.length > 1" class="foundation-list"><li v-for="(revision, index) in detail.foundationRevisions" :key="revision.id"><span><b>版本 {{ revision.version }}</b><small>{{ revision.reason }} · {{ revision.createdAt ? new Date(revision.createdAt).toLocaleString('zh-CN') : '' }}</small></span><button v-if="index" class="quiet" type="button" @click="emit('restore-foundation', detail.id, revision.id)">恢复此版本</button><small v-else>当前版本</small></li></ul></section>
      <section><h3>她认识的你</h3><ul v-if="detail.memories?.length" class="memory-list"><li v-for="memory in detail.memories" :key="memory.id"><span>{{ memory.key }}</span><p>{{ memory.value }}</p><button type="button" :aria-label="`删除记忆 ${memory.key}`" @click="emit('delete-memory', detail.id, memory.id)">×</button></li></ul><p v-else class="muted">她还在慢慢了解你。</p></section>
      <section><h3>关系变化</h3><ul v-if="detail.evolutions?.length" class="evolution-list"><li v-for="(item, index) in detail.evolutions" :key="item.id"><b>{{ item.reason }}</b><small>{{ item.createdAt ? new Date(item.createdAt).toLocaleString('zh-CN') : '' }}</small><span>{{ item.evidenceSummary }}</span><p v-for="change in item.changes || []" :key="change.field" class="evolution-diff">{{ change.field }}：{{ change.before }} → {{ change.after }}</p><button v-if="item.status === 'applied' && index === 0" class="quiet" type="button" @click="emit('rollback', detail.id, item.id)">撤销这次变化</button></li></ul><p v-else class="muted">还没有需要保留的关系变化。</p></section>
      <section><h3>身边的人</h3><p class="supporting-names">{{ detail.supportingCharacters?.map(item => item.name).join(' · ') || '会在生活里慢慢认识新朋友' }}</p></section>
      <section><h3>生图频率</h3><label class="detail-label" :for="`image-policy-${detail.id}`">选择她在视觉时刻的默认行为<select :id="`image-policy-${detail.id}`" :value="detail.imageGenerationPolicy || 'ask'" @change="emit('save-policy', detail.id, ($event.target as HTMLSelectElement).value)"><option value="ask">始终询问</option><option value="always">始终生成</option><option value="important">重要时刻自动生成</option><option value="user_only">只有我要求才生成</option><option value="autonomous">摇光实例自行决定</option></select></label></section>
      <section><h3>管理</h3><div class="detail-actions"><button class="quiet" type="button" @click="emit('chat', detail.id)">开始聊天</button><button class="quiet" type="button" @click="emit('activity', detail.id)">查看她的动态</button><button class="quiet" type="button" @click="emit('screen', detail.id)">{{ detail.screened ? '取消屏蔽' : '屏蔽动态与主动私聊' }}</button><button class="quiet" type="button" @click="emit('hidden-activities', detail.id)">管理已隐藏动态</button><button class="quiet danger" type="button" @click="emit('delete', detail.id)">删除此摇光实例</button></div></section>
      <section v-if="groups.length" class="persona-group-setting"><h3>所属分组</h3><p class="state-source">在这里设置 {{ detail.name }} 出现在哪个联系人分组中。</p><div class="persona-group-controls"><select :value="detail.groupId || ''" :aria-label="`选择 ${detail.name} 所属分组`" @change="emit('save-group', detail.id, ($event.target as HTMLSelectElement).value || null)"><option value="">所有联系人</option><option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option></select></div></section>
    </div>
  </section>
</template>

