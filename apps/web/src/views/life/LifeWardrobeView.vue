<script setup lang="ts">
import { computed, ref, watch } from "vue";

import Button from "@/components/ui/button/Button.vue";
import Textarea from "@/components/ui/textarea/Textarea.vue";
import AsyncState from "../../components/product/AsyncState.vue";
import ChoiceSelect from "../../components/product/ChoiceSelect.vue";
import FeedbackNotice from "../../components/product/FeedbackNotice.vue";
import FormField from "../../components/product/FormField.vue";
import { useFormErrors } from "../../composables/use-form-errors";
import { idSet, usedItems, wardrobeFilterAvailability, wardrobeFilters, wardrobeItemVisible, wornItems, sourceLabel, type WardrobeFilter } from "../../lib/life-surface";
import { newWardrobeItemDraft, wardrobeAvailabilityLabel, wardrobeAvailabilityOptions, wardrobeCategoryLabel, wardrobeCategoryOptions, wardrobeItemPayload, wardrobeOwnershipLabel, wardrobeOwnershipOptions, wardrobeSlotLabel, wardrobeSlotOptions } from "../../lib/wardrobe-form";
import { userError } from "../../lib/user-feedback";
import { useCharacterSurface } from "./use-character-surface";

const { store, control, detail, preview, readOnlyPreview } = useCharacterSurface();
const { errors, validate } = useFormErrors();
const filter = ref<WardrobeFilter>("all");
const draft = ref(newWardrobeItemDraft());
const categoryChoices = computed(() => wardrobeCategoryOptions(draft.value.itemKind));
const slotChoices = computed(() => wardrobeSlotOptions(draft.value.itemKind, draft.value.category));
watch(() => draft.value.itemKind, () => {
  draft.value.category = categoryChoices.value[0].value;
  draft.value.slot = wardrobeSlotOptions(draft.value.itemKind, draft.value.category)[0].value;
});
watch(() => draft.value.category, () => {
  if (!slotChoices.value.some((item) => item.value === draft.value.slot)) draft.value.slot = slotChoices.value[0]?.value ?? "";
});
watch(() => store.fluctlightId, (id) => {
  draft.value = newWardrobeItemDraft();
  if (!preview && id) void control.loadWardrobeItems(id);
}, { immediate: true });

const items = computed(() => preview ? preview.wardrobe : control.wardrobeItems);
const worn = computed(() => wornItems(detail.value));
const used = computed(() => usedItems(detail.value));
const wornIds = computed(() => idSet(worn.value));
const usedIds = computed(() => idSet(used.value));
const visible = computed(() => items.value.filter((item) => wardrobeItemVisible(filter.value, item, detail.value)));
const filterState = computed(() => wardrobeFilterAvailability(filter.value, detail.value));

async function addItem() {
  if (readOnlyPreview.value || control.saving || !await validate([{ id: "life-wardrobe-description", label: "物品描述", value: draft.value.description, required: true, maxLength: 512 }])) return;
  const selectedId = store.fluctlightId;
  try {
    const payload = wardrobeItemPayload(draft.value);
    if (await control.addWardrobeItems(selectedId, payload) && store.fluctlightId === selectedId) draft.value.description = "";
  } catch (error) {
    control.error = error instanceof Error ? error.message : "请检查物品选项。";
  }
}
</script>

<template>
  <section class="page life-page" aria-labelledby="life-wardrobe-title">
    <header class="pagehead">
      <h1 id="life-wardrobe-title">衣柜与物品</h1>
      <span class="badge b-plain">BP-08</span>
      <span v-if="readOnlyPreview" class="badge b-preview">不是业务数据</span>
    </header>
    <p v-if="!store.fluctlightId && !readOnlyPreview" class="statebox">请先选择角色。这里不会显示其他角色的数据。</p>
    <template v-else>
      <FeedbackNotice :message="control.error ? userError(control.error) : ''" />
      <FeedbackNotice :message="control.governanceNotice" tone="success" />
      <p class="small muted">拥有、穿着、使用和来源分开显示。添加物品不会换装，也不会开始使用。换装请求和拍照尚未接入。</p>
      <div class="life-grid">
        <article class="card">
          <h2>当前穿着</h2>
          <p v-if="worn === null" data-test="wearing-unwired">穿着尚未接入。</p>
          <p v-else-if="!worn.length" class="small muted">当前没有记录的穿着。</p>
          <ul v-else class="outfit"><li v-for="item in worn" :key="String(item.id)"><span class="badge">穿着</span> {{ item.description || item.id }} <small>{{ wardrobeOwnershipLabel(item.ownership) }}</small></li></ul>
        </article>
        <article class="card">
          <h2>正在使用</h2>
          <p v-if="used === null">使用尚未接入。</p>
          <p v-else-if="!used.length" class="small muted">当前没有记录的使用。</p>
          <ul v-else><li v-for="item in used" :key="String(item.id)"><span class="badge">使用</span> {{ item.description || item.id }}</li></ul>
        </article>
      </div>
      <div class="chips" role="tablist" aria-label="物品筛选">
        <button v-for="item in wardrobeFilters" :key="item.id" class="chip" :class="{ on: filter === item.id }" type="button" @click="filter = item.id">{{ item.label }}</button>
      </div>
      <AsyncState :loading="!readOnlyPreview && control.wardrobeLoading" loading-label="正在加载衣柜与物品…" />
      <p v-if="control.wardrobeError" class="banner err" role="alert">{{ control.wardrobeError }} <button class="btn sm" type="button" @click="control.loadWardrobeItems(store.fluctlightId)">重试</button></p>
      <p v-else-if="filterState === 'unwired'" class="statebox" data-test="filter-unwired">{{ filter === "inbound" ? "入库中尚未接入。没有单独的入库状态字段。" : filter === "wearing" ? "穿着筛选尚未接入。不会把部位当成穿着。" : "使用筛选尚未接入。" }}</p>
      <p v-else-if="!control.wardrobeLoading && !visible.length" class="statebox">当前筛选没有物品。</p>
      <div v-else class="wgrid" data-test="wgrid">
        <article v-for="item in visible" :key="item.id" class="icard" :class="{ wearing: wornIds?.has(item.id) }">
          <div class="item-fallback">物品图尚未接入</div>
          <div class="cap">
            <b>{{ item.description || item.id }}</b>
            <div class="row">
              <span class="badge">{{ wardrobeOwnershipLabel(item.ownership) }}</span>
              <span v-if="wornIds?.has(item.id)" class="badge">穿着</span>
              <span v-if="usedIds?.has(item.id)" class="badge">使用</span>
              <span class="badge b-plain">{{ wardrobeAvailabilityLabel(item.availability) }}</span>
            </div>
            <p class="small muted">分类 {{ wardrobeCategoryLabel(item.category) }} · 部位 {{ wardrobeSlotLabel(item.slot) }}</p>
            <p class="small muted">来源 {{ sourceLabel(item.source_kind) }} · revision {{ item.revision }}</p>
            <div v-if="!readOnlyPreview" class="row">
              <Button variant="ghost" type="button" :disabled="control.saving" @click="control.updateWardrobeItem(store.fluctlightId, item.id, { availability: item.availability === 'available' ? 'unavailable' : 'available' })">{{ item.availability === "available" ? "设为不可用" : "设为可用" }}</Button>
              <Button variant="ghost" type="button" :disabled="control.saving" @click="control.deleteWardrobeItem(store.fluctlightId, item.id)">删除</Button>
            </div>
          </div>
        </article>
      </div>
      <form v-if="!readOnlyPreview" class="card stack" novalidate @submit.prevent="addItem">
        <h2>登记物品</h2>
        <div class="form-grid">
          <label for="life-wardrobe-kind">物品类型<ChoiceSelect id="life-wardrobe-kind" v-model="draft.itemKind" :options="[{ value: 'wearable', label: '服装与配饰' }, { value: 'object', label: '普通物品' }]" /></label>
          <label for="life-wardrobe-category">分类<ChoiceSelect id="life-wardrobe-category" v-model="draft.category" :options="categoryChoices" /></label>
          <label for="life-wardrobe-slot">穿着部位<ChoiceSelect id="life-wardrobe-slot" v-model="draft.slot" :disabled="draft.itemKind === 'object'" :options="slotChoices" /></label>
          <label for="life-wardrobe-ownership">所有权<ChoiceSelect id="life-wardrobe-ownership" v-model="draft.ownership" :options="wardrobeOwnershipOptions" /></label>
          <label for="life-wardrobe-availability">可用状态<ChoiceSelect id="life-wardrobe-availability" v-model="draft.availability" :options="wardrobeAvailabilityOptions" /></label>
        </div>
        <FormField id="life-wardrobe-description" label="物品描述" required v-model:error="errors['life-wardrobe-description']" v-slot="{ controlAttrs }"><Textarea v-bind="controlAttrs" v-model="draft.description" rows="3" maxlength="512" /></FormField>
        <Button type="submit" :disabled="control.saving || !draft.description.trim()">添加物品</Button>
      </form>
    </template>
  </section>
</template>
