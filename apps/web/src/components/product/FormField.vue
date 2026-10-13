<script setup lang="ts">
import { computed } from "vue";
const props = defineProps<{ id: string; label: string; hint?: string; error?: string; required?: boolean }>();
const emit = defineEmits<{ "update:error": [value: string] }>();
const controlAttrs = computed(() => ({
  id: props.id,
  "aria-invalid": props.error ? true : undefined,
  "aria-required": props.required || undefined,
  "aria-describedby": props.error ? `${props.id}-error` : props.hint ? `${props.id}-hint` : undefined,
}));
</script>
<template>
  <div class="form-field" :class="{ 'has-error': error }" @input="emit('update:error', '')" @change="emit('update:error', '')">
    <label :for="id" class="form-field-label">{{ label }}<span v-if="required" class="field-required" aria-hidden="true">必填</span></label>
    <slot :control-attrs="controlAttrs" />
    <p v-if="error" :id="`${id}-error`" class="field-error" role="alert">{{ error }}</p>
    <p v-else-if="hint" :id="`${id}-hint`" class="field-hint">{{ hint }}</p>
  </div>
</template>
