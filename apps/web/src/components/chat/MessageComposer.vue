<script setup lang="ts">
import Textarea from "@/components/ui/textarea/Textarea.vue";
import AsyncButton from "../feedback/AsyncButton.vue";

defineProps<{
  draft: string;
  sending: boolean;
  canCancel: boolean;
  disabled: boolean;
}>();
const emit = defineEmits<{
  "update:draft": [value: string];
  submit: [];
  cancel: [];
  keydown: [event: KeyboardEvent];
}>();

function onKeydown(event: KeyboardEvent) {
  emit("keydown", event);
}
</script>
<template>
  <form class="message-composer composer" @submit.prevent="emit('submit')">
    <label class="sr-only" for="message-composer">消息</label>
    <div class="composer-row">
      <Textarea
        id="message-composer"
        :model-value="draft"
        rows="1"
        maxlength="32000"
        placeholder="写一条消息..."
        :disabled="disabled"
        @update:model-value="emit('update:draft', String($event))"
        @keydown="onKeydown"
      />
      <div class="composer-actions">
        <button v-if="canCancel" class="btn sm" type="button" @click="emit('cancel')">取消</button>
        <AsyncButton class="primary-button send-button" :busy="sending" busy-label="发送中" :disabled="disabled || !draft.trim()" @click="emit('submit')">发送</AsyncButton>
      </div>
    </div>
    <div class="composer-footer"><span class="composer-hint">Enter 发送 · Shift + Enter 换行</span></div>
  </form>
</template>
