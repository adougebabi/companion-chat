<script setup>
import { computed, ref, watch } from "vue";
const props = defineProps({
  name: String,
  initial: String,
  color: String,
  src: [String, Object],
});
const failed = ref(false),
  url = computed(() =>
    typeof props.src === "string" ? props.src : props.src?.url || "",
  );
watch(url, () => (failed.value = false));
const initial = computed(
  () => props.initial || [...(props.name || "光")].at(-1),
);
</script>
<template>
  <span class="avatar" :style="{ background: color }"
    ><img
      v-if="url && !failed"
      :src="url"
      :alt="name + '的头像'"
      @error="failed = true"
    /><template v-else>{{ initial }}</template></span
  >
</template>
