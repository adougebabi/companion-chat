<script setup>
import { computed } from "vue";
import {
  semanticLabel,
  hasSemanticValue,
} from "../../v3/proto/components/semantic-fields.js";
import { enumLabel } from "../../lib/fluctlight-display.ts";
import { wardrobeCategoryLabel, wardrobeSlotLabel, wardrobeOwnershipLabel, wardrobeAvailabilityLabel } from "../../lib/wardrobe-form.ts";
const props = defineProps({
  value: null,
  field: { type: String, default: "" },
  depth: { type: Number, default: 0 },
});
const technical = (k) =>
  /^(?:id|schema_version|provenance|extensions|evidence_refs|source|source_kind|source_ref|source_verified|revision|instant|update_policy|owner_switch_targets)$/.test(
    k,
  ) || /_(?:id|ids|revision|version|at)$/.test(k);
function categoryLabel(value) {
  const category = wardrobeCategoryLabel(value);
  if (category !== value || !/^[a-z\d_.-]+$/i.test(category)) return category;
  const slot = wardrobeSlotLabel(value);
  return slot !== value ? slot : "自定义分类";
}
const entries = computed(() =>
  props.value && typeof props.value === "object" && !Array.isArray(props.value)
    ? Object.entries(props.value).filter(
        ([key, value]) => !technical(key) && hasSemanticValue(value),
      )
    : [],
);
const dimensions = {
  openness: ["偏稳妥熟悉", "开放程度适中", "愿意探索新事物"],
  conscientiousness: ["偏随性", "条理程度适中", "重视计划与责任"],
  extraversion: ["偏内敛", "内外向适中", "偏外向"],
  agreeableness: ["更重视独立立场", "亲和程度适中", "偏温和合作"],
  neuroticism: ["情绪较稳定", "敏感程度适中", "感受较敏锐"],
  curiosity: ["偏熟悉事物", "好奇程度适中", "好奇心较强"],
  independence: ["重视共同决定", "独立程度适中", "偏独立自主"],
  patience: ["偏及时行动", "耐心程度适中", "比较耐心"],
  empathy: ["更重视事实判断", "共情程度适中", "比较细腻体贴"],
  assertiveness: ["偏含蓄表达", "主张程度适中", "倾向明确表达立场"],
  humor: ["表达较认真", "幽默程度适中", "表达较幽默"],
  sociability: ["偏少而深入的交往", "社交倾向适中", "比较乐于交往"],
  risk_tolerance: ["偏谨慎", "冒险倾向适中", "比较愿意尝试"],
};
const scalar = computed(() => {
  const value = props.value;
  if (typeof value === "boolean") return value ? "是" : "否";
  if (typeof value === "number") {
    if (dimensions[props.field] && value >= 0 && value <= 1)
      return dimensions[props.field][value < 0.35 ? 0 : value > 0.65 ? 2 : 1];
    return String(value);
  }
  const aliases = { known: "已记录", unknown: "尚未确定", male: "男性", female: "女性", nonbinary: "非二元性别" };
  const translated = aliases[value] || enumLabel(value);
  return translated === "未分类" ? String(value ?? "") : translated;
});
</script>
<template>
  <span v-if="value == null || value === ''" class="muted">尚未记录</span>
  <span v-else-if="typeof value !== 'object'">{{ scalar }}</span>
  <p v-else-if="depth >= 7" class="muted">更多层级可在技术详情中查看。</p>
  <ul v-else-if="Array.isArray(value) && ['wardrobe_items','worn_items','used_items'].includes(field)" class="semantic-list">
    <li v-for="(item, index) in value" :key="index">
      <template v-if="item && typeof item === 'object'">
        <strong>{{ item.description || item.name || '物品描述尚未记录' }}</strong>
        <p class="muted">{{ [categoryLabel(item.category), item.slot && wardrobeSlotLabel(item.slot), wardrobeOwnershipLabel(item.ownership), typeof item.available === 'boolean' ? (item.available ? '可用' : '不可用') : wardrobeAvailabilityLabel(item.availability)].filter(Boolean).join(' · ') }}</p>
      </template>
      <template v-else>{{ item }}</template>
    </li>
  </ul>
  <ul v-else-if="Array.isArray(value)" class="semantic-list">
    <li v-for="(item, index) in value" :key="index">
      <SemanticTree :value="item" :field="field" :depth="depth + 1" />
    </li>
    <li v-if="!value.length" class="muted">尚未记录</li>
  </ul>
  <dl v-else class="approved-facts">
    <div v-for="[key, item] in entries" :key="key">
      <dt>
        {{
          semanticLabel(key) === "自定义字段" ? "补充设定" : semanticLabel(key)
        }}
      </dt>
      <dd><SemanticTree :value="item" :field="key" :depth="depth + 1" /></dd>
    </div>
    <div v-if="!entries.length"><dd class="muted">尚未记录</dd></div>
  </dl>
</template>
