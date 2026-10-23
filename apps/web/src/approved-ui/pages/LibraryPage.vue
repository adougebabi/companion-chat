<script setup>
import { ref, computed, onMounted, onUnmounted } from "vue";
import { useWorkspace } from "../workspace.mjs";
import { ActionButton, Surface } from "../shared/primitives.mjs";
import { FilterChips } from "../shared/components.mjs";
import CharacterAvatar from "../shared/CharacterAvatar.vue";
import { Gu } from "../shared/icons.mjs";
const w = useWorkspace(),
  search = ref(""),
  filters = ref({ tag: "" }),
  configured = ref(null),
  saved = ref(null),
  error = ref("");
let active = true;
const characters = computed(() => w.state.characters),
  groups = computed(() => [
    {
      key: "tag",
      label: "标签",
      options: [...new Set(characters.value.flatMap((c) => c.tags || []))],
    },
  ]),
  rows = computed(() =>
    characters.value.filter(
      (c) =>
        [c.name, c.job, c.bio]
          .filter(Boolean)
          .join(" ")
          .toLowerCase()
          .includes(search.value.trim().toLowerCase()) &&
        (!filters.value.tag || c.tags?.includes(filters.value.tag)),
    ),
  );
function details(id) {
  w.select(id);
  w.navigate("角色详情");
}
function choose(id) {
  w.select(id);
  w.navigate("聊天");
}
const subtitle = (c) =>
  [c.job, c.detail?.identity?.residence].filter(Boolean).join(" · ");
onMounted(() => {
  try {
    const raw = sessionStorage.getItem(
      `fluctlight-creation:${w.state.session.actorId}:draft`,
    );
    if (raw) saved.value = JSON.parse(raw);
  } catch {
    error.value = "上次的创建资料需要核对，可以从继续创建入口查看。";
  }
  void w.services.configurationClient
    .read()
    .then((value) => {
      if (active)
        configured.value = value.bindings.some((b) => b.role === "generic_llm");
    })
    .catch(() => {
      if (active) configured.value = null;
    });
});
onUnmounted(() => (active = false));
</script>
<template>
  <div class="library">
    <div class="row library-toolbar">
      <p class="muted">
        {{
          characters.length
            ? "在这里，遇见和你一起生活的人。"
            : "相识，可以从一句简单的介绍开始。"
        }}
      </p>
      <span class="spacer" /><a
        v-if="characters.length"
        class="button primary"
        href="#/library/create"
        >{{ saved && !saved.activatedId ? "继续创建" : "创建角色" }}</a
      >
    </div>
    <Surface v-if="saved" as="aside" class="card approved-guidance"
      ><b>{{ saved.activatedId ? "继续查看准备进度" : "继续上次的相识" }}</b>
      <p>上次的资料和准备进度已经为你保留。</p>
      <a class="text-button" href="#/library/create"
        >继续创建与查看进度 ↗</a
      ></Surface
    >
    <p v-if="error" class="inline-error" role="alert">{{ error }}</p>
    <template v-if="characters.length"
      ><FilterChips
        v-model:search="search"
        v-model:filters="filters"
        :groups="groups"
        test-id="library-filters"
        placeholder="搜索角色 / 职业"
      /><Surface
        v-for="character in rows"
        :key="character.id"
        as="article"
        class="card compact library-character"
        ><div class="row">
          <CharacterAvatar
            :name="character.name"
            :initial="character.initial"
            :color="character.color"
            :src="character._directory?.avatar"
          />
          <div class="library-character-copy">
            <b>{{ character.name }}</b>
            <p v-if="subtitle(character)">{{ subtitle(character) }}</p>
            <span v-for="tag in character.tags || []" :key="tag" class="pill">{{
              tag
            }}</span>
          </div>
          <span class="spacer" /><span
            v-if="w.state.selectedId === character.id"
            class="pill current-badge"
            >当前</span
          >
          <details class="actions-menu">
            <summary :aria-label="character.name + '的操作'">⋯</summary>
            <div>
              <ActionButton @click="details(character.id)"
                >角色详情</ActionButton
              ><ActionButton
                v-if="w.state.selectedId !== character.id"
                @click="choose(character.id)"
                >切换角色</ActionButton
              >
            </div>
          </details>
        </div></Surface
      >
      <Surface v-if="!rows.length" as="div" class="card empty-state"
        ><p>暂无匹配角色。</p>
        <ActionButton
          class="button"
          @click="
            search = '';
            filters = { tag: '' };
          "
          >清除筛选</ActionButton
        ></Surface
      ></template
    >
    <Surface v-else as="section" class="card library-first-use"
      ><Gu :size="30" aria-hidden="true" />
      <h2>还没有一起生活的人</h2>
      <p>
        从一个名字或一段介绍开始，为新的相识准备一个生活世界。<br />我们会一步步陪你整理资料，再由你确认开启生活。
      </p>
      <p v-if="configured === false" class="approved-guidance">
        还没有连接通用模型。描述分析需要先配置模型，也可以选择不调用模型的空白创建。
      </p>
      <div class="row first-use-actions">
        <a
          v-if="configured !== true"
          class="button"
          href="#/settings?section=%E6%A8%A1%E5%9E%8B%E7%94%A8%E9%80%94"
          >先连接模型</a
        ><a
          class="button primary"
          data-testid="library-create-first"
          href="#/library/create"
          >{{ saved ? "继续创建第一个角色" : "创建第一个角色" }}</a
        >
      </div></Surface
    >
  </div>
</template>
