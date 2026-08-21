<script setup lang="ts">
import { nextTick, ref, watch } from 'vue';

const props = withDefaults(defineProps<{
  modelValue?: string;
  personaName: string;
  disabled?: boolean;
  isComposing?: boolean;
}>(), { modelValue: '', disabled: false, isComposing: false });
const emit = defineEmits<{
  (event: 'update:modelValue', value: string): void;
  (event: 'submit'): void;
  (event: 'composition-start'): void;
  (event: 'composition-end'): void;
  (event: 'selection-change', start: number, end: number): void;
}>();

const textarea = ref<HTMLTextAreaElement | null>(null);
const localValue = ref(props.modelValue);
watch(() => props.modelValue, value => { if (value !== localValue.value) localValue.value = value; });

function onInput(event: Event) {
  const target = event.currentTarget as HTMLTextAreaElement;
  localValue.value = target.value;
  emit('update:modelValue', target.value);
  emit('selection-change', target.selectionStart || 0, target.selectionEnd || 0);
}

function onKeydown(event: KeyboardEvent) {
  if (event.key !== 'Enter' || event.shiftKey || props.isComposing) return;
  event.preventDefault();
  emit('submit');
}

function onSelection(event: Event) {
  const target = event.currentTarget as HTMLTextAreaElement;
  emit('selection-change', target.selectionStart || 0, target.selectionEnd || 0);
}

async function focus() {
  await nextTick();
  textarea.value?.focus({ preventScroll: true });
}

defineExpose({ textarea, focus });
</script>

<template>
  <form class="composer" aria-label="发送消息" @submit.prevent="emit('submit')">
    <textarea
      ref="textarea"
      :value="localValue"
      rows="1"
      :placeholder="`给 ${personaName} 发消息`"
      aria-label="消息内容"
      :disabled="disabled"
      @input="onInput"
      @keydown="onKeydown"
      @select="onSelection"
      @keyup="onSelection"
      @compositionstart="emit('composition-start')"
      @compositionend="emit('composition-end')"
    />
    <button class="send-button" type="submit" aria-label="发送" title="发送" :disabled="disabled || !localValue.trim()">↑</button>
  </form>
</template>

