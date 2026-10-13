<script setup lang="ts" generic="T extends string">
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

defineOptions({ inheritAttrs: false });
const props = defineProps<{ modelValue: T; options: readonly { value: T; label: string }[]; disabled?: boolean }>();
const emit = defineEmits<{ "update:modelValue": [value: T] }>();
// Prefix every option so an empty domain value remains selectable without changing its payload.
function choiceValue(value: string) { return `choice:${value}`; }
function choose(value: unknown) {
  const option = props.options.find(option => choiceValue(option.value) === String(value));
  if (option) emit("update:modelValue", option.value);
}
</script>

<template>
  <Select :model-value="choiceValue(modelValue)" :disabled="disabled" @update:model-value="choose">
    <SelectTrigger v-bind="$attrs" class="w-full"><SelectValue /></SelectTrigger>
    <SelectContent><SelectItem v-for="option in options" :key="choiceValue(option.value)" :value="choiceValue(option.value)">{{ option.label }}</SelectItem></SelectContent>
  </Select>
</template>
