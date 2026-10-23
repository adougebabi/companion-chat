<script setup>
import { computed, ref, onMounted, onUnmounted } from "vue";
import { useWorkspace } from "../workspace.mjs";
import { ActionButton, Surface } from "../shared/primitives.mjs";
import { ActionFeedback } from "../shared/components.mjs";
import CharacterAvatar from "../shared/CharacterAvatar.vue";
import SemanticTree from "../shared/SemanticTree.vue";
import ProfileForm from "../shared/ProfileForm.vue";
import { profileSnapshotFromDetail } from "../../v3/proto/data/profile.js";
import {
  semanticLabel,
  hasSemanticValue,
} from "../../v3/proto/components/semantic-fields.js";
import {
  coreFromDetail,
  profileFields,
  profileChanges,
  profileSaveCommand,
  rebaseProfileFields,
} from "../profile-fields.mjs";
import { errorMessage, stateLabel } from "../../v3/domain-presentation.mjs";
const w = useWorkspace(),
  fid = w.state.selectedId,
  scope = w.currentScope(),
  detail = ref(null),
  world = ref(null),
  editing = ref(false),
  fields = ref([]),
  base = ref(null),
  baseline = ref(null),
  saving = ref(false),
  readbackPending = ref(false),
  submittedFields = ref(null),
  error = ref(""),
  conflict = ref(false),
  last = ref(null),
  switching = ref(false),
  target = ref(""),
  reason = ref(""),
  confirm = ref(false);
let active = true,
  epoch = 0;
const live = () => active && w.currentScope() === scope;
const snapshot = computed(() =>
    profileSnapshotFromDetail(detail.value || {}, w.current.value._directory),
  ),
  core = computed(() => coreFromDetail(detail.value || {})),
  personas = computed(() => snapshot.value.profiles),
  currentPersona = computed(() =>
    personas.value.find((p) => p.id === snapshot.value.activeProfileId),
  );
const sections = computed(() =>
  [
    ["identity", "身份与背景", core.value.identity],
    [
      "personality",
      "共同性格",
      detail.value?.personality || detail.value?.core_persona?.personality,
    ],
    ["policy", "表达与相处", core.value.behavioral_policy],
    ["life", "生活设定", core.value.life_profile],
    ["working", "当前表达画像", snapshot.value.groups.find((group) => group.key === 'working')?.value],
    [
      "current",
      "此刻状态",
      detail.value?.current_state || detail.value?.inner_state,
    ],
    ["developing", "发展中的自我", detail.value?.developing_self],
  ].filter(([, , value]) => hasSemanticValue(value)),
);
const title = computed(
    () => detail.value?.identity?.name || w.current.value.name,
  ),
  subtitle = computed(() =>
    [
      detail.value?.identity?.age != null
        ? detail.value.identity.age + " 岁"
        : "",
      detail.value?.identity?.occupation,
      stateLabel(detail.value?.status, ""),
    ]
      .filter(Boolean)
      .join(" · "),
  );
const avatar = computed(
  () => snapshot.value.avatar || w.current.value._directory?.avatar?.url || "",
);
const governance = (kind) =>
  "#/c/" +
  encodeURIComponent(fid) +
  "/governance" +
  (kind ? "?tab=" + encodeURIComponent(kind) : "");
async function read({ preserve = false } = {}) {
  const token = ++epoch;
  try {
    const raw = await w.client.detail(fid);
    if (!live() || token !== epoch) return;
    detail.value = raw;
    w.current.value.detail = raw;
    if (preserve) {
      base.value = coreFromDetail(raw);
      baseline.value = raw.current_revision;
      fields.value = rebaseProfileFields(base.value, fields.value, semanticLabel,
        readbackPending.value ? submittedFields.value : null);
    }
    error.value = "";
    readbackPending.value = false;
    submittedFields.value = null;
    return true;
  } catch (cause) {
    if (live() && token === epoch) error.value = errorMessage(cause);
    return false;
  }
}
function edit() {
  base.value = coreFromDetail(detail.value);
  baseline.value = detail.value.current_revision;
  fields.value = profileFields(base.value, semanticLabel);
  editing.value = true;
  error.value = "";
  conflict.value = false;
}
async function save() {
  if (saving.value || snapshot.value.readOnly) return;
  try {
    const changes = profileChanges(base.value, fields.value);
    if (!Object.keys(changes).length) {
      editing.value = false;
      return;
    }
    if (!Number.isSafeInteger(baseline.value))
      throw Error("资料版本尚未读取，请重新读取后保存");
    const submitted = JSON.parse(JSON.stringify(fields.value));
    saving.value = true;
    error.value = "";
    const record = w.command(
      "角色详情",
      "profile:" + fid,
      {
        changes,
        expectedRevision: baseline.value,
        reason: "所有者编辑角色设定资料",
      },
      profileSaveCommand({ client: w.client, fid, live }),
      {
        preAdmissionConflicts: ["revision_conflict", "foundation_proposal_stale"],
        success: "角色资料已保存，并记录本次修订",
        onSaved: async () => {
          if (!live()) return;
          readbackPending.value = true;
          submittedFields.value = submitted;
          const readback = await read({ preserve: true });
          if (!readback) return;
          if (live()) {
            if (!Object.keys(profileChanges(base.value, fields.value)).length)
              editing.value = false;
            void w.refreshDirectory();
          }
        },
      },
    );
    last.value = record;
    await record.completion;
  } catch (cause) {
    if (live()) {
      error.value = errorMessage(cause);
      conflict.value = cause.status === 409;
    }
  } finally {
    if (live()) saving.value = false;
  }
}
async function rebase() {
  try {
    if (last.value) await last.value.release();
    if (!live()) return;
    await read({ preserve: true });
    conflict.value = false;
  } catch (cause) {
    if (live()) error.value = errorMessage(cause);
  }
}
async function switchPersona() {
  if (
    !target.value ||
    !reason.value.trim() ||
    !confirm.value ||
    switching.value
  )
    return;
  if (
    !personas.value.some((p) => p.id === target.value) ||
    target.value === snapshot.value.activeProfileId
  ) {
    error.value = "请选择当前角色可用的其他人格";
    return;
  }
  if (
    snapshot.value.switchTargetIds &&
    !snapshot.value.switchTargetIds.includes(target.value)
  ) {
    error.value = "该人格当前不能通过所有者指令切换";
    return;
  }
  switching.value = true;
  const submitted = { target: target.value, reason: reason.value };
  try {
    const body = {
      targetPersonaId: target.value,
      expectedRevision: snapshot.value.foundationRevision,
      expectedRuntimeRevision: snapshot.value.runtimeRevision,
      reason: reason.value.trim(),
      confirm: true,
    };
    if (
      !Number.isSafeInteger(body.expectedRevision) ||
      !Number.isSafeInteger(body.expectedRuntimeRevision)
    )
      throw Error("人格或运行版本未读取完整，请重新读取");
    const record = w.command(
      "角色详情",
      "persona-switch",
      body,
      async (value, key) => {
        const result = await w.client.switchPersona(fid, {
          ...value,
          idempotencyKey: key,
        });
        return result.request_id
          ? { request: { id: result.request_id, status: "pending" } }
          : result;
      },
      {
        onSaved: async () => {
          if (live()) await read();
        },
        success: "人格切换已确认",
      },
    );
    await record.completion;
    if (
      live() &&
      target.value === submitted.target &&
      reason.value === submitted.reason
    ) {
      target.value = "";
      reason.value = "";
      confirm.value = false;
    }
  } catch (cause) {
    if (live()) error.value = errorMessage(cause);
  } finally {
    if (live()) switching.value = false;
  }
}
onMounted(() => {
  void read();
  void w.client
    .world(fid)
    .then((value) => {
      if (live()) world.value = value.world;
    })
    .catch(() => {});
});
onUnmounted(() => {
  active = false;
  epoch++;
});
</script>
<template>
  <div class="character-detail character-workflow">
    <Surface as="section" class="card"
      ><header class="row character-profile-heading">
        <CharacterAvatar
          class="character-avatar"
          :name="title"
          :initial="w.current.value.initial"
          :color="w.current.value.color"
          :src="avatar"
        />
        <div>
          <h2>{{ title }}</h2>
          <small v-if="subtitle">{{ subtitle }}</small>
        </div>
        <span class="spacer" /><ActionButton
          v-if="detail && !editing"
          class="button"
          data-testid="detail-edit"
          :disabled="snapshot.readOnly"
          @click="edit"
          >编辑资料</ActionButton
        >
      </header>
      <form
        v-if="editing"
        class="character-form detail-form"
        novalidate
        @submit.prevent="save"
      >
        <p class="muted">
          保存会生成并接受一条资料修订记录。这里编辑的是设定；实际穿着、位置和已发生的经历由生活记录决定。
        </p>
        <ProfileForm :fields="fields" />
        <p v-if="error" class="inline-error" role="alert">{{ error }}</p>
        <ActionFeedback :labels="['角色详情']" /><ActionButton
          v-if="readbackPending"
          type="button"
          class="button"
          @click="read({ preserve: true })"
          >重新读取已保存资料</ActionButton
        ><ActionButton
          v-if="conflict"
          type="button"
          class="button"
          @click="rebase"
          >读取最新资料，保留当前编辑</ActionButton
        >
        <footer class="wizard-footer">
          <ActionButton
            type="button"
            class="button"
            :disabled="saving"
            @click="editing = false"
            >取消</ActionButton
          ><span class="spacer" /><ActionButton
            class="button primary"
            data-testid="detail-save"
            :disabled="saving || readbackPending"
            >{{ saving ? "保存中…" : "保存" }}</ActionButton
          >
        </footer>
      </form>
      <div v-else class="character-detail-sections">
        <p v-if="!detail && !error" role="status">正在读取角色资料…</p>
        <p v-if="error" class="inline-error" role="alert">
          {{ error }}
          <ActionButton class="text-button" @click="read()"
            >重新读取</ActionButton
          >
        </p>
        <section v-for="[key, label, value] in sections" :key="key">
          <h3>{{ label }}</h3>
          <SemanticTree
            :value="value"
            :field="key === 'personality' ? 'personality' : ''"
          />
        </section>
        <section v-if="personas.length">
          <h3>人格档案</h3>
          <p>
            当前人格：{{
              currentPersona?.name || snapshot.activeProfileId || "尚未确认"
            }}
          </p>
          <details v-for="persona in personas" :key="persona.id">
            <summary>
              {{ persona.name || persona.id }}
              <small v-if="persona.id === snapshot.activeProfileId">当前</small>
            </summary>
            <SemanticTree :value="persona" />
          </details>
          <details v-if="personas.length > 1 && !snapshot.readOnly">
            <summary>切换人格</summary>
            <form class="character-stack" @submit.prevent="switchPersona">
              <label
                >目标人格<select v-model="target" aria-label="目标人格">
                  <option value="">请选择</option>
                  <option
                    v-for="persona in personas.filter(
                      (p) =>
                        p.id !== snapshot.activeProfileId &&
                        (!snapshot.switchTargetIds ||
                          snapshot.switchTargetIds.includes(p.id)),
                    )"
                    :key="persona.id"
                    :value="persona.id"
                  >
                    {{ persona.name || persona.id }}
                  </option>
                </select></label
              ><label
                >切换原因<textarea
                  v-model="reason"
                  maxlength="1000"
                  rows="2"
                /></label
              ><label class="check-field"
                ><input
                  v-model="confirm"
                  type="checkbox"
                />我确认向当前角色提出这次人格切换</label
              ><ActionButton
                class="button"
                :disabled="switching || !target || !reason.trim() || !confirm"
                >{{ switching ? "处理中…" : "确认切换" }}</ActionButton
              >
            </form>
          </details>
        </section>
        <section v-if="detail?.core_persona?.personality_system">
          <details>
            <summary>人格之间的关系与协作</summary>
            <SemanticTree
              :value="
                Object.fromEntries(
                  Object.entries(detail.core_persona.personality_system).filter(
                    ([key]) => key !== 'profiles',
                  ),
                )
              "
            />
          </details>
        </section>
        <details v-if="snapshot.history.length">
          <summary>人格切换记录</summary>
          <ul class="semantic-list">
            <li v-for="(entry, index) in snapshot.history" :key="entry.id || index">
              {{ personas.find((persona) => persona.id === entry.from_profile_id)?.name || '原人格' }} →
              {{ personas.find((persona) => persona.id === entry.to_profile_id)?.name || '目标人格' }}
              <p v-if="entry.reason" class="muted">{{ entry.reason }}</p>
            </li>
          </ul>
          <p v-if="snapshot.historyHasMore" class="muted">这里展示最近的切换记录，完整记录可在治理入口查看。</p>
        </details>
        <section>
          <h3>头像与生活世界</h3>
          <p v-if="world?.name">{{ world.name }}</p>
          <p v-if="detail?.identity?.residence">
            居住地 · {{ detail.identity.residence }}
          </p>
          <div class="row">
            <a
              class="text-button"
              :href="'#/c/' + encodeURIComponent(fid) + '/media/avatar'"
              >更换头像 ↗</a
            ><a
              class="text-button"
              :href="'#/c/' + encodeURIComponent(fid) + '/world'"
              >查看生活世界 ↗</a
            ><a
              class="text-button"
              :href="'#/c/' + encodeURIComponent(fid) + '/life/wardrobe'"
              >她的衣柜 ↗</a
            >
          </div>
        </section>
        <details v-if="detail">
          <summary>技术详情与来源</summary>
          <p>
            身份核心版本 {{ snapshot.foundationRevision ?? "未读取" }} ·
            运行版本 {{ snapshot.runtimeRevision ?? "未读取" }}
          </p>
          <pre class="approved-json">{{
            JSON.stringify(
              {
                core_persona: detail.core_persona,
                provenance: detail.provenance,
                initialization_source: detail.initialization_source,
                foundation_revisions: detail.foundation_revisions,
                persona_switch_history: detail.persona_switch_history,
                persona_compilation: detail.persona_compilation,
                working_personas: detail.working_personas,
              },
              null,
              2,
            )
          }}</pre>
        </details>
      </div></Surface
    >
    <p class="character-more">
      <small>更多：</small
      ><a class="text-button" :href="governance('capabilities')">能力申请</a
      ><span>·</span
      ><a class="text-button" :href="governance('developing')">发展中的自我</a
      ><span>·</span
      ><a class="text-button" :href="governance('foundation')"
        >身份核心与提议</a
      >
    </p>
    <ActionFeedback v-if="!editing" :labels="['角色详情']" />
  </div>
</template>
