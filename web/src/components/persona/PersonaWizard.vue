<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import type { PersonaSummary } from '../types';

interface PreviewData {
  [key: string]: unknown;
  name?: string;
  role?: string;
  foundation?: string;
  interests?: string[];
  visualBaseline?: string;
  supportingCast?: string[];
  inferred?: boolean;
  routine?: string[];
}

const props = withDefaults(defineProps<{
  stage?: 'description' | 'preview';
  description?: string;
  preview?: PreviewData | null;
  analyzing?: boolean;
  creating?: boolean;
  error?: string | null;
}>(), { stage: 'description', description: '', preview: null, analyzing: false, creating: false, error: null });
const emit = defineEmits<{
  (event: 'close'): void;
  (event: 'analyze', description: string): void;
  (event: 'back'): void;
  (event: 'create', values: PreviewData): void;
}>();

const descriptionDraft = ref(props.description);
const previewDraft = ref<PreviewData>({ ...(props.preview || {}) });
watch(() => props.description, value => { descriptionDraft.value = value; });
watch(() => props.preview, value => { previewDraft.value = { ...(value || {}) }; }, { deep: true });
const title = computed(() => props.stage === 'description' ? '描述一下她' : '确认她的设定');

function submitDescription() { emit('analyze', descriptionDraft.value.trim()); }
function submitPreview(event: Event) {
  const form = event.currentTarget as HTMLFormElement;
  const data = new FormData(form);
  emit('create', {
    ...previewDraft.value,
    name: String(data.get('name') || '').trim(), role: String(data.get('role') || '').trim(),
    foundation: String(data.get('foundation') || '').trim(), interests: String(data.get('interests') || '').split(/[、,，]/).map(value => value.trim()).filter(Boolean),
    visualBaseline: String(data.get('visualBaseline') || '').trim(), supportingCast: String(data.get('supportingCast') || '').split(/[、,，]/).map(value => value.trim()).filter(Boolean)
  });
}
</script>

<template>
  <form v-if="stage === 'description'" class="persona-wizard" @submit.prevent="submitDescription">
    <header><div><small>FLUCTLIGHT INSTANCE</small><h2 id="persona-dialog-title">{{ title }}</h2></div><button class="close-dialog" type="button" aria-label="关闭创建流程" @click="emit('close')">×</button></header>
    <div class="wizard-body"><p class="wizard-intro">描述你希望她是谁、什么性格、生活背景以及你们如何相处。自然地写几句就够了。</p><p v-if="error" class="wizard-error" role="alert">{{ error }}</p><label>摇光实例描述<textarea v-model="descriptionDraft" rows="9" maxlength="6000" required data-initial-focus placeholder="例如：她叫林晚，是在读设计专业的学生，性格细腻慢热，喜欢摄影和旧书。">{{ descriptionDraft }}</textarea></label></div>
    <footer class="wizard-footer"><button class="quiet" type="button" @click="emit('close')">取消</button><button class="primary" type="submit" :disabled="analyzing || !descriptionDraft.trim()">{{ analyzing ? '分析中…' : '分析并预览' }}</button></footer>
  </form>
  <form v-else class="persona-wizard" @submit.prevent="submitPreview">
    <header><div><small>FLUCTLIGHT INSTANCE PREVIEW</small><h2 id="persona-dialog-title">{{ title }}</h2></div><button class="close-dialog" type="button" aria-label="关闭创建流程" @click="emit('close')">×</button></header>
    <div class="wizard-body"><p class="wizard-intro">确认前可以修改核心摘要。AI 推断的字段会保留来源标记，原始描述只用于这次分析。</p><p v-if="error" class="wizard-error" role="alert">{{ error }}</p><div class="preview-card"><b>{{ previewDraft.inferred ? 'AI 推断' : '你的设定' }}</b><p><strong>日常作息</strong>{{ previewDraft.routine?.join(' · ') || '将由生活模型生成' }}</p></div><label>名字<input name="name" maxlength="30" required :value="previewDraft.name || ''" /></label><label>身份<input name="role" maxlength="80" required :value="previewDraft.role || ''" /></label><label>身份核心<textarea name="foundation" rows="5" maxlength="3000" required>{{ previewDraft.foundation || '' }}</textarea></label><label>兴趣<input name="interests" maxlength="180" :value="previewDraft.interests?.join('、') || ''" /></label><label>外观和日常穿衣印象<input name="visualBaseline" maxlength="240" :value="previewDraft.visualBaseline || ''" /></label><label>身边最早出现的人<input name="supportingCast" maxlength="180" :value="previewDraft.supportingCast?.join('、') || ''" /></label></div>
    <footer class="wizard-footer"><button class="quiet" type="button" @click="emit('back')">返回修改</button><button class="primary" type="submit" :disabled="creating">{{ creating ? '创建中…' : '确认并创建' }}</button></footer>
  </form>
</template>
