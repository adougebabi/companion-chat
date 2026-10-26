<script setup>
import { computed, ref, onMounted, onUnmounted, watch } from "vue";
import { useWorkspace } from "../workspace.mjs";
import { directory, activationFromAnalysis } from "../../v3/providers/real.ts";
import { get, list } from "../../v3/proto/store.js";
import { semanticLabel } from "../../v3/proto/components/semantic-fields.js";
import { errorMessage } from "../../v3/domain-presentation.mjs";
import { createCreationController } from "../creation-controller.mjs";
import { ActionButton, Surface } from "../shared/primitives.mjs";
import PreparationSteps from "../shared/PreparationSteps.vue";
import ProfileForm from "../shared/ProfileForm.vue";
import SemanticTree from "../shared/SemanticTree.vue";
import { migrateCreationRecovery } from "../creation-recovery.mjs";
import {reusableWorlds} from '../world-sources.mjs';
const w = useWorkspace(),
  owner = w.state.session?.actorId,
  migrated = migrateCreationRecovery(owner,localStorage,sessionStorage),
  c = createCreationController({
    owner,
    client: w.client,
    directory,
    storage: localStorage,
    analysisToActivation: activationFromAnalysis,
    ownerCurrent: () => w.state.session?.actorId === owner,
    refreshDirectory: w.refreshDirectory,
    label: semanticLabel,
    query: new URLSearchParams(location.hash.split("?")[1] || ""),
    today: (zone) => w.day(new Date(), zone),
  });
// `new=1` is an entry action, not a persistent instruction on every reload.
const entryUrl = new URL(location.href);
const [entryPath, entrySearch = ""] = entryUrl.hash.split("?");
const entryQuery = new URLSearchParams(entrySearch);
if (entryQuery.has("new")) {
  entryQuery.delete("new");
  entryUrl.hash = entryPath + (entryQuery.size ? "?" + entryQuery : "");
  history.replaceState(history.state, "", entryUrl);
}
const { draft, step, busy, error, feedback, fieldErrors, progress, previewFields, pending,analysisStartedAt,worldSourceReview } =
    c,
  worlds = ref([]),
  existingWorlds=ref([]),
  worldChoice=ref(!!draft.worldSource||!!draft.worldResourceId||draft.step>0||!!draft.activatedId),
  templates = ref([]),
  worldCursor = ref(""),
  templateCursor = ref(""),
  sourceError = ref(""),
  configured = ref(null);
const wizardRoot=ref(null);
const analysisNow=ref(Date.now());
watch(step,()=>{
  const root=wizardRoot.value,heading=root?.querySelector('h2');
  if(heading){heading.tabIndex=-1;heading.focus({preventScroll:true});root.scrollIntoView({block:'start'});}
},{flush:'post'});
let timer,
  active = true,
  reading = false;
const modes = [
  ["blank", "空白创建", "从名字开始，其他资料可以在相处中逐渐补充。"],
  ["description", "一段描述", "由模型把身份、性格与生活整理成完整设定。"],
  ["json", "JSON 导入", "导入已有的人物资料，核对后再创建。"],
  ["template", "角色模板", "选择一份已发布版本，先解析与审阅。"],
];
const groups = computed(() =>
  list("groups").filter((g) => g.id !== "ungrouped"),
);
const navSteps = ["生活世界", "认识角色", "资料预览", "准备相遇"],
  currentStep = computed(
    () =>
      ({
        world: 0,
        person: 1,
        analysis: 1,
        preview: 2,
        progress: 3,
        complete: 3,
      })[step.value] ?? 0,
  ),
  stateLabel = (s) =>
    ({
      ready: "已完成",
      running: "进行中",
      pending: "等待中",
      failed: "失败",
      unknown: "状态待确认",
    })[s] || "状态待确认";
const configHref = (section) =>
  "#/settings?section=" + encodeURIComponent(section || "模型用途");
async function loadSources(kind, more = false) {
  try {
    const result = await w.client.listStudioResources({
      kind,
      published: true,
      cursor: more
        ? kind === "world"
          ? worldCursor.value
          : templateCursor.value
        : "",
    });
    if (!active || !c.live()) return;
    if (kind === "world") {
      worlds.value = more ? [...worlds.value, ...result.items] : result.items;
      worldCursor.value = result.next_cursor || "";
    } else {
      templates.value = more
        ? [...templates.value, ...result.items]
        : result.items;
      templateCursor.value = result.next_cursor || "";
    }
    sourceError.value = "";
  } catch (cause) {
    if (active) sourceError.value = errorMessage(cause);
  }
}
async function loadExistingWorlds(){
  try{const result=await reusableWorlds(w.client,w.state.characters);if(!active||!c.live())return;existingWorlds.value=result.items;if(result.unavailable)sourceError.value='部分已有世界暂未读到，可以重新读取后选择。';}
  catch(e){if(active)sourceError.value=errorMessage(e);}
}
function chooseExisting(world){c.selectExistingWorld(world);worldChoice.value=true;}
function chooseDefinition(world){c.selectWorld(world);worldChoice.value=true;}
function templateChanged(event) {
  const value = templates.value.find((t) => t.id === event.target.value);
  if (!value) return;
  draft.templateId = value.id;
  draft.templateVersion = value.published_version;
  c.invalidate();
}
function prev() {
  if (pending.value || draft.activatedId) return;
  if (step.value === "person") step.value = "world";
  else if (step.value === "preview") {
    step.value = "person";
  }
  c.persist();
}
async function poll() {
  analysisNow.value=Date.now();
  if (
    !active ||
    document.hidden ||
    reading ||
    busy.value ||
    !draft.activatedId ||
    progress.value?.complete
  )
    return;
  reading = true;
  try {
    await c.refreshProgress();
  } finally {
    reading = false;
  }
}
watch(previewFields, () => c.persistPreview(), { deep: true });
const visibility = () => {
  if (!document.hidden) void poll();
};
onMounted(() => {
  void loadSources("world");
  void loadExistingWorlds();
  void loadSources("character_template");
  void w.services.configurationClient
    .read()
    .then((value) => {
      if (c.live())
        configured.value = value.bindings.some((b) => b.role === "generic_llm");
    })
    .catch(() => {});
  if (draft.activatedId) void c.refreshProgress();
  timer = setInterval(() => void poll(), 5000);
  document.addEventListener("visibilitychange", visibility);
});
onUnmounted(() => {
  active = false;
  clearInterval(timer);
  document.removeEventListener("visibilitychange", visibility);
  c.dispose();
});
</script>
<template>
  <div ref="wizardRoot" class="character-workflow">
    <div class="character-breadcrumb">
      <a class="text-button" href="#/library">角色库</a><span>›</span
      ><small>{{ draft.activatedId ? "为新的生活做准备" : "新的相识" }}</small>
    </div>
    <ol class="wizard-steps">
      <li
        v-for="(label, index) in navSteps"
        :key="label"
        :class="{
          active: index === currentStep,
          complete: index < currentStep,
        }"
        :aria-current="index === currentStep ? 'step' : undefined"
      >
        <span>{{ index + 1 }}</span
        >{{ label }}
      </li>
    </ol>
    <Surface
      v-if="step === 'world'"
      as="form"
      class="card character-form"
      novalidate
      @submit.prevent="worldChoice&&c.next()"
      ><header>
        <small>第 1 / 4 步</small>
        <h2>生活世界</h2>
        <p class="muted">
          先选择已有世界的配置，或创建一份新的设定。新角色的生活记录始终独立。
        </p>
      </header>
      <section class="character-stack" aria-label="复用已有世界配置"><h3>已有生活世界</h3><p class="muted">复用背景、规则和具体地点；人物、事件模板和生活历史不复制。</p><ActionButton v-for="world in existingWorlds" :key="world.fluctlightId+':'+world.id" type="button" class="world-choice button" :aria-pressed="draft.worldSource?.worldId===world.id&&draft.worldSource?.fluctlightId===world.fluctlightId" @click="chooseExisting(world)"><b>{{world.name}}</b><small>沿用 {{world.characterName}} 的世界与地点配置</small></ActionButton><p v-if="!existingWorlds.length" class="muted">还没有可复用的生活世界，可以选择下方已发布设定，或创建新的世界设定。</p><button class="button" type="button" @click="loadExistingWorlds">重新读取已有世界</button></section>
      <h3>也可以从新的设定开始</h3>
      <ActionButton
        type="button"
        :class="['button', { primary: worldChoice&&draft.useDefaultWorld }]"
        :aria-pressed="worldChoice&&draft.useDefaultWorld"
        @click="chooseDefinition(null)"
        >使用日常生活世界</ActionButton
      ><small class="muted"
        >会为新角色建立独立的生活世界，具体地点可在相处中继续补充。</small
      >
      <div class="character-stack">
        <ActionButton
          v-for="world in worlds"
          :key="world.id"
          type="button"
          class="world-choice button"
          :aria-pressed="
            draft.worldResourceId === world.id && !draft.useDefaultWorld
          "
          @click="chooseDefinition(world)"
          ><b>{{ world.name }}</b
          ><small>已发布版本 {{ world.published_version }}</small></ActionButton
        >
        <p v-if="!worlds.length && !sourceError" class="muted">
          还没有已发布的世界底稿，也可以先使用日常生活世界。
        </p>
        <ActionButton
          v-if="worldCursor"
          type="button"
          class="button"
          @click="loadSources('world', true)"
          >读取更多世界</ActionButton
        >
        <p v-if="sourceError" class="inline-error">
          {{ sourceError }}
          <ActionButton
            type="button"
            class="text-button"
            @click="loadSources('world')"
            >重新读取</ActionButton
          >
        </p>
        <ActionButton class="button" type="button" @click="w.navigatePath('/studio/edit/new?kind=world')">创建世界设定</ActionButton>
      </div>
      <ul v-if="Object.keys(fieldErrors).length" class="inline-error" role="alert"><li v-for="(message,key) in fieldErrors" :key="key">{{message}}</li></ul>
    <p v-if="error" class="inline-error" role="alert">{{ error }}</p>
      <footer class="wizard-footer">
        <ActionButton class="button" type="button" @click="w.navigatePath('/library')">保留草稿并返回</ActionButton
        ><span class="spacer" /><ActionButton
          class="button primary"
          :disabled="!worldChoice||busy"
          data-testid="wizard-world-next"
          >下一步</ActionButton
        >
      </footer></Surface
    >
    <Surface
      v-else-if="step === 'person' || step === 'analysis'"
      as="form"
      class="card character-form"
      novalidate
      @submit.prevent="c.analyze()"
      ><header>
        <small>第 2 / 4 步</small>
        <h2>这是一个什么样的人？</h2>
        <p class="muted">
          {{
            draft.useDefaultWorld
              ? "生活世界：日常生活世界"
              : "生活世界：" + (draft.worldName || "所选已发布底稿")
          }}
        </p>
      </header>
      <fieldset
        class="character-fieldset character-stack"
        :disabled="busy || !!pending"
      >
        <div class="creation-mode-choices">
          <ActionButton
            v-for="[value, label, note] in modes"
            :key="value"
            type="button"
            :class="['button creation-mode', { active: draft.mode === value }]"
            :aria-pressed="draft.mode === value"
            @click="c.changeMode(value)"
            ><b>{{ label }}</b
            ><small>{{ note }}</small></ActionButton
          >
        </div>
        <div class="character-field-grid">
          <label
            >名字{{ draft.mode === "blank" ? "（必填）" : "（可选）"
            }}<input
              v-model="draft.name"
              data-testid="wizard-name"
              maxlength="200"
              placeholder="你想怎么称呼对方？"
              @input="c.invalidate()"
            /><small
              v-if="error && draft.mode === 'blank' && !draft.name.trim()"
              class="inline-error"
              >请填写角色名字。</small
            ></label
          ><label
            >生活时区（必填）<input
              v-model="draft.timezone"
              aria-label="生活时区"
              placeholder="例如 Asia/Shanghai"
              @input="c.invalidate()"
            /><small
              v-if="error && (!draft.timezone || error.includes('时区'))"
              class="inline-error"
              >{{ error.includes("时区") ? error : "请填写生活时区。" }}</small
            ></label
          >
        </div>
        <label
          >角色分组<select v-model="draft.groupId" @change="c.persist()">
            <option value="">未分组</option>
            <option v-for="group in groups" :key="group.id" :value="group.id">
              {{ group.name }}
            </option>
          </select></label
        >
        <template v-if="draft.mode === 'description'"
          ><label
            >描述你想认识的人<textarea
              v-model="draft.bio"
              data-testid="wizard-description"
              rows="7"
              placeholder="身份、背景、性格和生活可以写在同一段描述里。"
              @input="c.invalidate()"
            /><small v-if="error && !draft.bio.trim()" class="inline-error"
              >请写一段人物描述。</small
            ></label
          >
          <details>
            <summary>可选：一些相处期望</summary>
            <div class="character-stack">
              <label
                >性格期望<input
                  v-model="draft.traits"
                  @input="c.invalidate()" /></label
              ><label
                >说话方式<textarea
                  v-model="draft.speakingStyle"
                  rows="2"
                  @input="c.invalidate()"
                /></label
              ><label
                >核心价值观<textarea
                  v-model="draft.coreValues"
                  rows="2"
                  @input="c.invalidate()"
                /></label
              ><small class="muted"
                >这些内容会加入描述，由模型整理；不会把标签直接写成人格数值。</small
              >
            </div>
          </details></template
        >
        <template v-else-if="draft.mode === 'json'"
          ><a
            class="text-button"
            href="/templates/yaoguang-character-init-template.v2.json"
            download
            >下载当前 JSON 模板</a
          ><label
            >选择 JSON 文件<input
              type="file"
              accept="application/json,.json"
              @change="c.importFile($event.target.files?.[0])"
          /></label>
          <p class="muted">
            {{ draft.fileName || "尚未选择文件" }} · 上限 60000 UTF-8 字节
          </p>
          <label>核对或修正 JSON
            <textarea v-model="draft.json" data-testid="wizard-json" rows="10" spellcheck="false" @input="c.invalidate()" />
          </label><p class="muted">填写已知的人物资料，保留模板结构；不要导入聊天存档或其他分析记录。</p></template
        >
        <template v-else-if="draft.mode === 'template'"
          ><label
            >已发布角色模板<select
              :value="draft.templateId || ''"
              @change="templateChanged"
            >
              <option value="">请选择一份模板</option>
              <option
                v-for="template in templates"
                :key="template.id"
                :value="template.id"
              >
                {{ template.name }} · {{ template.published_version }}
              </option>
            </select></label
          >
          <p v-if="draft.templateId" class="muted">
            使用精确发布版本 {{ draft.templateVersion }}
          </p>
          <ActionButton
            v-if="templateCursor"
            type="button"
            class="button"
            @click="loadSources('character_template', true)"
            >读取更多模板</ActionButton
          >
          <p v-if="!templates.length" class="muted">
            还没有已发布的角色模板。可以选择其他创建方式，或<button class="button" type="button" @click="w.navigatePath('/studio/edit/new?kind=character_template')">去创作中心准备角色模板</button>。
          </p></template
        >
        <p v-else class="muted">
          空白创建从名字和生活时区开始，其他资料可以稍后在角色详情里补充。
        </p>
      </fieldset>
      <p
        v-if="configured === false && draft.mode === 'description'"
        class="approved-guidance"
      >
        还没有配置通用模型。描述分析需要模型服务；也可以先选择空白创建。<a
          class="text-button"
          :href="configHref('模型用途')"
          >先连接模型 ↗</a
        >
      </p>
      <p v-if="step === 'analysis'" class="approved-guidance" role="status">
        正在整理人物设定，已等待 {{Math.max(0,Math.floor((analysisNow-(analysisStartedAt||analysisNow))/1000))}} 秒。原输入已保留。
        <ActionButton type="button" class="text-button" @click="c.cancelAnalysis()">停止本次等待</ActionButton>
      </p>
      <p v-if="error && !Object.keys(fieldErrors).length" class="inline-error" role="alert">
        {{ error }}
        <a
          v-if="feedback?.type === 'configuration'"
          class="text-button"
          :href="configHref('模型用途')"
          >去配置中心 ↗</a
        >
      </p>
      <footer class="wizard-footer">
        <ActionButton
          type="button"
          class="button"
          :disabled="busy"
          @click="prev"
          >上一步</ActionButton
        ><span class="spacer" /><ActionButton
          class="button primary"
          :disabled="busy || !!pending || (draft.mode==='template'&&(!draft.templateId||!draft.templateVersion))"
          data-testid="wizard-analyze"
          >{{
            busy
              ? "整理中…"
              : draft.mode === "blank"
                ? "预览空白角色"
                : "整理并预览资料"
          }}</ActionButton
        >
      </footer></Surface
    >
    <Surface
      v-else-if="step === 'preview'"
      as="form"
      class="card character-form"
      novalidate
      @submit.prevent="c.activate()"
      ><header>
        <small>第 3 / 4 步</small>
        <h2>在相遇之前，先认识一下</h2>
        <p class="muted">
          先核对身份、表达和生活设定，确认后开始准备相遇。
        </p>
      </header>
      <template v-if="draft.mode === 'blank'"
        ><h3>{{ draft.name }}</h3>
        <p>生活时区：{{ draft.timezone }}</p>
        <p class="muted">
          性格从服务端默认起点开始，经历与认识在后续互动中形成。
        </p></template
      ><template v-else
        ><ProfileForm :fields="previewFields" :disabled="busy || !!pending" />
        <details>
          <summary>共同性格与人格档案（只读）</summary>
          <SemanticTree
            :value="{
              personality: draft.analysis?.core_persona?.personality,
              personality_system:
                draft.analysis?.core_persona?.personality_system,
            }"
          />
        </details>
        <details>
          <summary>初始关系、目标与意图</summary>
          <SemanticTree
            :value="{
              initial_relationships: draft.analysis?.initial_relationships,
              initial_goals: draft.analysis?.initial_goals,
              initial_intentions: draft.analysis?.initial_intentions,
            }"
          />
        </details>
        <label class="check-field"
          ><input
            v-model="draft.includeGoals"
            type="checkbox"
            :disabled="busy || !!pending"
            @change="c.persist()"
          />采用已预览的初始目标和意图</label
        >
        <details>
          <summary>完整设定与分析来源</summary>
          <pre class="approved-json">{{
            JSON.stringify(draft.analysis, null, 2)
          }}</pre>
        </details></template
      >
      <p v-if="error" class="inline-error" role="alert">{{ error }}</p>
      <footer class="wizard-footer">
        <ActionButton
          type="button"
          class="button"
          :disabled="busy || !!pending"
          @click="prev"
          >返回修改输入</ActionButton
        ><span class="spacer" /><ActionButton
          class="button primary"
          data-testid="wizard-activate"
          :disabled="busy || !!pending"
          >{{ busy ? "正在激活…" : "确认并开始准备" }}</ActionButton
        >
      </footer></Surface
    >
    <Surface v-else as="section" class="card character-preparation"
      ><header>
        <small>{{ draft.name || "角色" }}的新生活</small>
        <h2>
          {{
            progress?.complete ? "准备好了，很高兴遇见你" : progress?.blocker ? "准备已暂停，需要恢复" : "一点点，准备好相遇"
          }}
        </h2>
        <p class="muted">
          {{
            progress?.complete
              ? "每个准备环节都已读回确认，接下来一起走进日常。"
              : progress?.blocker ? `「${progress.blocker.title}」尚未完成，后续等待环节不会自行继续。请先恢复此环节，角色与原任务均已保留。` : "角色与原任务会保留。遇到问题时，可单独恢复尚未完成的环节。"
          }}
        </p>
      </header>
      <PreparationSteps
        :stages="progress?.stages || []"
        :busy="busy"
        @recover="c.recoverStage"
      />
      <section v-if="draft.worldSource&&!draft.worldSelectionConfirmed" class="card stack"><h3>已选世界的配置核对</h3><p class="muted">原角色会保留。只有服务端明确拒绝、确认没有创建新世界时，才可核对并采用最新配置。</p><ActionButton type="button" class="button" :disabled="busy" @click="c.reviewWorldSource">核对最新世界配置</ActionButton><template v-if="worldSourceReview"><SemanticTree :value="worldSourceReview.preview"/><ActionButton type="button" class="button primary" :disabled="busy" @click="c.confirmWorldSourceReview">确认按以上配置继续复用</ActionButton></template></section>
      <p v-if="!progress" class="muted" role="status">
        {{ busy ? "正在确认原创建结果…" : "正在读取实际准备进度…" }}
      </p>
      <p v-if="error" class="inline-error" role="alert">{{ error }}</p>
      <div class="preparation-done row">
        <ActionButton type="button"
          v-if="draft.activatedId && draft.worldSelectionConfirmed"
          class="button primary"
          @click="w.navigatePath('/c/' + encodeURIComponent(draft.activatedId) + '/chat')"
          >先开始相处</ActionButton
        ><ActionButton type="button"
          v-if="draft.activatedId"
          class="button"
          @click="w.navigatePath('/c/' + encodeURIComponent(draft.activatedId) + '/profile')"
          >角色详情</ActionButton
        ><ActionButton
          class="button"
          :disabled="busy"
          @click="c.refreshProgress()"
          >重新读取进度</ActionButton
        ><ActionButton
          v-if="progress?.complete"
          class="text-button"
          @click="c.newDraft()"
          >创建新的角色</ActionButton
        >
      </div></Surface
    >
    <Surface v-if="pending" as="section" class="card approved-guidance"
      ><p>
        原激活结果尚未确认，当前资料和请求身份已保留。请先查询或重试原提交，避免创建第二个角色。
      </p>
      <ActionButton
        v-if="!pending.rejection"
        class="button"
        :disabled="busy"
        @click="c.activate(true)"
        >查询或重试原激活</ActionButton
      ><ActionButton v-else class="button" @click="c.reanalyze()"
        >保留输入并重新整理</ActionButton
      ></Surface
    >
    <button v-if="step!=='world'" class="button" type="button" @click="w.navigatePath('/library')">保留草稿并返回角色库</button>
    <ActionButton v-if="draft.activatedId&&!c.archived&&!pending" class="text-button" @click="c.newDraft()">再认识一个人（保留当前进度）</ActionButton>
  </div>
</template>
