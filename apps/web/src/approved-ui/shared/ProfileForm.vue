<script setup>
import { computed } from "vue";
import ProfileField from "./ProfileField.vue";
const props = defineProps({
  fields: { type: Array, required: true },
  disabled: Boolean,
});
const primary = {
  identity: new Set([
    "name",
    "gender",
    "age",
    "occupation",
    "residence",
    "timezone",
    "background_story",
    "core_values",
  ]),
  behavioral_policy: new Set([
    "response_style",
    "message_length",
    "directness",
    "initiative",
    "conflict_style",
    "refusal_style",
  ]),
  life_profile: new Set([
    "social_background",
    "preferences",
    "life_habits",
    "character_constraints",
  ]),
};
const sections = computed(() =>
  [
    ["identity", "身份与背景", "更多身份资料"],
    ["behavioral_policy", "表达与相处", "更多表达设置"],
    ["life_profile", "生活设定", "更多生活与外貌设定"],
  ]
    .map(([key, title, moreTitle]) => {
      const fields = props.fields.filter((f) => f.path[0] === key);
      return {
        key,
        title,
        moreTitle,
        fields,
        primary: fields.filter((f) => primary[key].has(f.path[1])),
        more: fields.filter((f) => !primary[key].has(f.path[1])),
      };
    })
    .filter((g) => g.fields.length),
);
</script>
<template>
  <fieldset class="character-fieldset character-stack" :disabled="disabled">
    <section v-for="group in sections" :key="group.key" class="character-stack">
      <h3>{{ group.title }}</h3>
      <div v-if="group.primary.length" class="character-field-grid">
        <ProfileField
          v-for="field in group.primary"
          :key="JSON.stringify(field.path)"
          :field="field"
        />
      </div>
      <details v-if="group.more.length">
        <summary>{{ group.moreTitle }}</summary>
        <div class="character-field-grid approved-more-fields">
          <ProfileField
            v-for="field in group.more"
            :key="JSON.stringify(field.path)"
            :field="field"
          />
        </div>
      </details>
    </section>
  </fieldset>
</template>
