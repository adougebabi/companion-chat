<script setup lang="ts">
import { nextTick, ref, watch } from 'vue';

const props = withDefaults(defineProps<{ open?: boolean; labelledBy?: string; size?: 'small' | 'medium' | 'large' }>(), { open: false, labelledBy: '', size: 'medium' });
const emit = defineEmits<{ (event: 'update:open', value: boolean): void; (event: 'close'): void }>();
const dialog = ref<HTMLDialogElement | null>(null);
const lastTrigger = ref<HTMLElement | null>(null);

function openDialog() {
  if (!dialog.value || dialog.value.open) return;
  lastTrigger.value = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  dialog.value.showModal();
  void nextTick(() => (dialog.value?.querySelector<HTMLElement>('[data-initial-focus], input, textarea, select, button') || dialog.value)?.focus());
}

function closeDialog() {
  if (dialog.value?.open) dialog.value.close('cancel');
  emit('update:open', false);
}

function onNativeClose() {
  emit('update:open', false);
  emit('close');
  if (lastTrigger.value?.isConnected) lastTrigger.value.focus({ preventScroll: true });
  lastTrigger.value = null;
}

watch(() => props.open, value => value ? openDialog() : closeDialog());
defineExpose({ dialog, close: closeDialog, open: openDialog });
</script>

<template>
  <dialog ref="dialog" class="app-dialog" :class="`app-dialog--${size}`" :aria-labelledby="labelledBy || undefined" @close="onNativeClose" @cancel.prevent="closeDialog">
    <slot />
  </dialog>
</template>

