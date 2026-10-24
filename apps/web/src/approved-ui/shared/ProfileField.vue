<script setup>
defineProps({ field: { type: Object, required: true } });
</script>
<template>
  <label
    :class="{
      'full-field': field.kind === 'lines' || String(field.value).length > 80,
    }"
    >{{ field.label
    }}<select v-if="field.kind === 'boolean'" v-model="field.value">
      <option :value="true">是</option>
      <option :value="false">否</option></select
    ><select v-else-if="field.kind === 'select'" v-model="field.value" :aria-label="field.label"><option v-for="option in field.options" :key="option.value" :value="option.value">{{option.label}}</option></select><textarea
      v-else-if="
        field.kind === 'lines' ||
        String(field.value).length > 80 ||
        /background|biography|notes|description/.test(field.path.at(-1))
      "
      v-model="field.value"
      rows="3"
      :aria-label="field.label"
      :placeholder="field.kind === 'lines' ? '每行一项，可留空' : '尚未填写'"
    /><input
      v-else
      v-model="field.value"
      :type="field.kind === 'number' ? 'number' : 'text'"
      :min="field.min"
      :max="field.max"
      :step="field.kind === 'number' ? 'any' : undefined"
      :aria-label="field.label"
      placeholder="尚未填写"
    /><small v-if="field.max === 1" class="muted"
      >0–1，数值越大表示这项倾向越明显。</small
    ></label
  >
</template>
