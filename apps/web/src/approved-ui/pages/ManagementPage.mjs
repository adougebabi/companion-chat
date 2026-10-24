import {
  defineComponent,
  ref,
  computed,
  onMounted,
  onUnmounted,
  watch,
  h,
  Fragment,
  onMounted as mount,
  onUnmounted as unmount,
  nextTick,
} from "vue";
import { useWorkspace } from "../workspace.mjs";
import { renderManagementPage } from "../renderers/ManagementPage.mjs";
import { FilterChips, DetailDrawer } from "../shared/components.mjs";
import { diagnosticProjection } from "../presentation.mjs";
import { catalogLabel } from "../../v3/runtime-catalog.mjs";
import ModelDirectory from "../shared/ModelDirectory.vue";
import SettingsField from "../shared/SettingsField.vue";
import { configurationFields } from "../../v3/proto/components/configuration-fields.js";
import {
  acknowledgeConfiguration,
  clearAcknowledgedCredentials,
} from "../configuration-ack.mjs";
import { persistentCommand } from "../../v3/providers/persistent-command.mjs";
import { configurationPlan } from "../configuration-plan.mjs";
import { connectionDraftKey, connectionDraft, restoreConnectionDraft,connectionIsSaved } from "../configuration-draft.mjs";
import { errorMessage, domainName } from "../../v3/domain-presentation.mjs";
import { randomId } from "../../random-id.mjs";
import { userInstant } from "../../v3/presentation-time.mjs";
import { installFieldValidation } from "../../v3/proto/components/forms.js";
const clone = (v) => JSON.parse(JSON.stringify(v));
const groupSpecs = [
  ["图片（ComfyUI）", ["media.comfyui"]],
  ["聊天回复", ["conversation.reply"]],
  ["看图理解", ["vision.understanding"]],
  ["角色自主与主动联系", ["product.autonomy"]],
  [
    "作息与后台节奏",
    ["product.wakeup", "product.summary", "product.goal_retry"],
  ],
  ["资源与预算", ["llm.queue"]],
  ["通知", ["product.notifications"]],
  ["高级", ["kev"]],
];
const roleNames = {
  generic_llm: "思考与通用分析",
  conversation_reply: "聊天回复",
  vision_understanding: "看图理解",
  embedding: "记忆检索向量",
};
const weekdays = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];
function uiField(settingKey, field, values) {
  const raw = values?.[field.key],
    choices = (field.options || []).map((o) =>
      typeof o === "string" ? { value: o, label: o } : o,
    );
  let type =
    field.type === "boolean"
      ? "checkbox"
      : field.type === "number"
        ? "number"
        : field.type === "select"
          ? "select"
          : [
                "workflow",
                "json",
                "boolean-map",
                "quiet-hours",
              ].includes(field.type)
            ? "json"
            : field.type;
  return {
    label: field.label,
    value:
      type === "json"
        ? raw === undefined
          ? ""
          : JSON.stringify(raw, null, 2)
        : type === "select"
          ? choices.find((o) => o.value === raw)?.label || raw || ""
          : (raw ?? (type === "checkbox" ? false : "")),
    type,
    options: choices.map((o) => o.label),
    min: field.min,
    max: field.max,
    _key: field.key,
    _settingKey: settingKey,
    _choices: choices,
    _definition: field,
    _initial: raw,
  };
}
export default defineComponent({
  name: "ManagementPage",
  __scopeId: "data-v-5bb367ad",
  setup() {
    const w = useWorkspace(),
      config = w.services.configurationClient,
      diagnostics = w.services.diagnosticsClient,
      scope = w.state.session?.actorId;
    let active = true,
      catalog = null,
      versions = {},
      loaded = false;
    const live = () => active && w.state.session?.actorId === scope;
    const tab = computed({
        get: () => w.state.managementTab,
        set: (v) => (w.state.managementTab = v),
      }),
      kind = ref("模型调用"),
      search = ref(""),
      filters = ref({ type: "", who: "", status: "" }),
      selectedId = ref(null),
      highlight = ref(null),
      saving = ref(false),
      saveStatus = ref(""),
      error = ref(""),
      reference = ref(""),
      testResult = ref("");
    const settings = ref([]),
      endpoints = ref([]),
      bindings = ref([]),
      quiet = ref({
        enabled: false,
        start: "",
        end: "",
        days: [],
        behavior: "暂停主动联系",
        character: "全部角色",
      }),
      records = ref([]),
      pending = new Map(),
      recovery = ref(null);
    const configForm=ref(null);
    watch(configForm,(node,_old,cleanup)=>{if(node)cleanup(installFieldValidation(node));},{flush:'post'});
    const policyChannels = new Map(),
      policyRecovery = ref(null);
    async function writePolicy(operation) {
      let channel = policyChannels.get(operation.fid);
      if (!channel) {
        channel = persistentCommand({
          key:
            "fluctlight:approved-policy:" +
            JSON.stringify([scope, operation.fid]),
          storage: () => sessionStorage,
          execute: async (value, key) => {
            if (!live()) throw Error("所有者已变化");
            return w.client.saveAutonomyPolicy(value.fid, {
              ...value.body,
              idempotencyKey: key,
            });
          },
        });
        policyChannels.set(operation.fid, channel);
      }
      try {
        return await channel.submit({
          fid: operation.fid,
          body: operation.body,
        });
      } catch (error) {
        policyRecovery.value = { channel, fid: operation.fid };
        throw error;
      }
    }
    async function retryPolicy() {
      const original = policyRecovery.value;
      if (!original) return;
      try {
        const pending = original.channel.pending(),
          receipt = await original.channel.retry();
        if (!live()) return;
        acknowledgeConfiguration(
          {
            catalog,
            versions,
            settings: settings.value,
            endpoints: endpoints.value,
            bindings: bindings.value,
            quiet: quiet.value,
          },
          {
            operation: "autonomy",
            fid: pending.value.fid,
            body: pending.value.body,
          },
          receipt,
        );
        policyRecovery.value = null;
        error.value = "原角色设置已确认，当前输入保留。";
      } catch (error) {
        if (live()) error.value = errorMessage(error);
      }
    }
    const characters = computed(() => w.state.characters);
    const referenceSections = ref(
      [
        ["逻辑运行", "agent-runs"],
        ["后台任务", "workflow-intents"],
        ["生命周期", "lifecycle"],
        ["回合诊断", "turns"],
        ["Kev 决策", "kev"],
        ["主动触达投递", "events"],
        ["待处理事项 / 重试耗尽", "workflow-intents"],
        ["技术详情", "links"],
        ["安全拦截", "harm"],
      ].map(([title, kind]) => ({ title, kind, rows: [] })),
    );
    const selected = computed(() =>
      records.value.find((r) => r.id === selectedId.value),
    );
    const statusLabel = (status) =>
      ({ error: "失败", pending: "处理中", success: "完成" })[status] ||
      "状态待确认";
    const rows = computed(() =>
      records.value
        .filter(
          (r) =>
            r.kind === kind.value &&
            (!filters.value.type || filters.value.type === r.label) &&
            (!filters.value.who || filters.value.who === r.character) &&
            (!filters.value.status ||
              filters.value.status === statusLabel(r.status)) &&
            [r.label, r.input, r.character, r.model, r.reason, r.response]
              .join(" ")
              .toLowerCase()
              .includes(search.value.trim().toLowerCase()),
        )
        .sort((a, b) => b.start - a.start),
    );
    const filterGroups = computed(() => [
      {
        key: "type",
        label: "类型",
        options: [
          ...new Set(
            records.value
              .filter((r) => r.kind === kind.value)
              .map((r) => r.label),
          ),
        ],
      },
      {
        key: "who",
        label: "角色",
        options: characters.value.map((c) => c.name),
      },
      { key: "status", label: "状态", options: ["完成", "失败", "处理中"] },
    ]);
    const clear = () => {
      search.value = "";
      filters.value = { type: "", who: "", status: "" };
    };
    const sections = computed(() => [
      "服务地址",
      "模型用途",
      "安静时段",
      ...settings.value.map((g) => g.title),
    ]);
    const sectionId = (value) => "config-" + value;
    const scrollTo = (value) => {
      const node = document.getElementById(sectionId(value));
      if (node?.tagName === "DETAILS") node.open = true;
      node?.scrollIntoView({ behavior: "smooth", block: "start" });
      const heading=node?.querySelector('summary,h2');
      if(heading){if(heading.tagName==='H2')heading.tabIndex=-1;heading.focus({preventScroll:true});}
    };
    const sharedScenarios = computed(() =>
      (w.state.runtimeCatalog?.agents || [])
        .filter(
          (a) =>
            a.binding_role ? a.binding_role === "generic_llm" : ["cognitive_assessment","generic_llm","media_prompt","initialization","reflection","interaction","action_realization","visual_identity_patch"].includes(a.role),
        )
        .map((a) => catalogLabel("agent", a.name, a.description)),
    );
    async function readCatalog() {
      const snapshot = await config.read();
      if (!live()) return;
      snapshot._loadedAt = Date.now();
      catalog = snapshot;
      versions = { ...snapshot.versions };
      w.state.configuration = snapshot;
      settings.value = groupSpecs.map(([title, keys]) => ({
        title,
        fields: keys.flatMap((key) =>
          (configurationFields[key] || [])
            .filter((f) => !["model_ref", "quiet_hours"].includes(f.key))
            .map((f) => uiField(key, f, snapshot.effective_values[key])),
        ),
      }));
      endpoints.value = snapshot.endpoints.map((e) => ({
        id: e.id,
        name: e.id,
        protocol: e.kind,
        url: e.base_url,
        secret: "",
        clearSecret: false,
        models: [],
        status: e.secret_configured ? "已保存凭据" : "已保存",
        _source: e,
      }));
      bindings.value = Object.entries(roleNames).map(([role, name]) => {
        const b = snapshot.bindings.find((v) => v.role === role);
        return {
          name,
          role,
          endpoint: b?.endpoint_id || "",
          model: b?.model_id || "",
          timeout: b?.timeout_seconds ?? "",
          _source: b,
        };
      });
      const restored = restoreConnectionDraft(sessionStorage.getItem(connectionDraftKey(scope)), snapshot);
      if (restored) {
        endpoints.value = restored.endpoints;
        for (const binding of bindings.value) {
          const saved=restored.bindings.find(b=>b.role===binding.role);
          if(saved)Object.assign(binding,saved);
        }
        versions={...restored.versions};
        if(restored.stale)error.value='配置已经更新。未保存的连接草稿已恢复，请读取最新基线并核对后保存。';
        else if(restored.credentialPending)error.value='连接草稿已恢复。访问密钥不会保存在浏览器草稿中，请重新填写。';
      }
      loaded=true;
      const hours =
        snapshot.effective_values["product.autonomy"]?.quiet_hours || {};
      quiet.value = {
        enabled: !!hours.start,
        start: hours.start || "",
        end: hours.end || "",
        days: (hours.days || [0, 1, 2, 3, 4, 5, 6]).map((d) => weekdays[d]),
        behavior: "暂停主动联系",
        character: "全部角色",
        _source: hours,
      };
      for (const e of endpoints.value.filter(connectionIsSaved)) {
        void config
          .models(e.id)
          .then((result) => {
            if (live()) e.models = result.models || [];
          })
          .catch(() => {});
      }
      void config
        .catalog()
        .then((value) => {
          if (live()) w.state.runtimeCatalog = value;
        })
        .catch(() => {});
      await nextTick();
      if (live()) scrollTo(w.state.managementSection||'服务地址');
    }
    async function loadDiagnostics() {
      const [models, media] = await Promise.allSettled([
        diagnostics.page("model-runs", { limit: 100 }),
        diagnostics.page("media-prompts", { limit: 100 }),
      ]);
      if (!live()) return;
      const projected = [];
      for (const [index, result] of [models, media].entries()) {
        if (result.status === "rejected") {
          error.value = errorMessage(result.reason);
          continue;
        }
        for (const record of result.value.items) {
          projected.push(diagnosticProjection(record, index, characters.value));
        }
      }
      records.value = projected;
      w.state.diagnostics = projected;
    }
    async function write(kind, body, run) {
      const keyName =
        "fluctlight:approved-config-command:" + JSON.stringify([scope, kind]);
      let value = pending.get(kind);
      if (!value) {
        const restored = sessionStorage.getItem(keyName);
        if (restored) {
          recovery.value = { key: restored, keyName, kind };
          throw Error(
            "有原配置提交等待核对，请先恢复原回执或停止尚未受理的提交；当前草稿已保留。",
          );
        }
        value = { body: clone(body), key: randomId() };
        pending.set(kind, value);
        sessionStorage.setItem(keyName, value.key);
      }
      if (JSON.stringify(value.body) !== JSON.stringify(body)) {
        recovery.value = { key: value.key, keyName, kind };
        throw Error("原配置提交结果待核对，当前新草稿已保留。");
      }
      try {
        await run(value.body, value.key);
        if (!live()) throw Error("设置范围已经变化");
        let receipt = await config.command(value.key);
        if (receipt.wake_rearm_required)
          receipt = await config.recover(value.key);
        if (!live()) throw Error("设置范围已经变化");
        pending.delete(kind);
        sessionStorage.removeItem(keyName);
        return receipt;
      } catch (cause) {
        recovery.value = { key: value.key, keyName, kind };
        throw cause;
      }
    }
    async function recoverOriginal(stop = false) {
      const original = recovery.value;
      if (!original) return;
      try {
        const receipt = stop
          ? await config.cancelUnaccepted(original.key)
          : await config.command(original.key);
        if (!live()) return;
        if (receipt.status !== "cancelled") await config.recover(original.key);
        if (!live()) return;
        pending.delete(original.kind);
        sessionStorage.removeItem(original.keyName);
        recovery.value = null;
        error.value =
          "原提交已核对。请点击“读取最新基线”，保留当前输入并核对后再次保存。";
      } catch (cause) {
        if (live()) error.value = errorMessage(cause);
      }
    }
    async function rebase() {
      if (recovery.value) {
        error.value = "请先核对原提交，再读取最新基线";
        return;
      }
      if (
        [...policyChannels.values()].some(
          (c) => c.pending() && !c.pending().rejection,
        )
      ) {
        error.value = "原角色设置结果尚未确认，请先重试原角色设置";
        return;
      }
      try {
        const next = await config.read();
        if (!live()) return;
        catalog = next;
        versions = { ...next.versions };
        for (const group of settings.value)
          for (const field of group.fields)
            field._initial =
              next.effective_values[field._settingKey]?.[field._key];
        for (const endpoint of endpoints.value)
          endpoint._source = next.endpoints.find(
            (e) => e.id === (endpoint._source?.id || endpoint.name),
          );
        for (const binding of bindings.value)
          binding._source = next.bindings.find((b) => b.role === binding.role);
        if (quiet.value.character !== "全部角色" && quiet.value._policy) {
          const fid = quiet.value._policy.fluctlight_id,
            policy = await w.client.autonomyPolicy(fid);
          if (!live() || quiet.value._policy?.fluctlight_id !== fid) return;
          if (policy.fluctlight_id !== fid) throw Error("角色策略范围不匹配");
          quiet.value._policy = policy;
          quiet.value._source = policy.quiet_hours;
        }
        for (const channel of policyChannels.values())
          if (channel.pending()?.rejection) channel.releaseRejected();
        policyRecovery.value = null;
        error.value = "已读取最新基线，当前输入保持不变；请核对后保存。";
      } catch (cause) {
        if (live()) error.value = errorMessage(cause);
      }
    }
    async function save() {
      if (saving.value || !catalog) return;
      saving.value = true;
      saveStatus.value = "正在保存设置，等待服务端确认…";
      error.value = "";
      const submitted = {
        settings: clone(settings.value),
        endpoints: clone(endpoints.value),
        bindings: clone(bindings.value),
        quiet: clone(quiet.value),
      };
      try {
        const plan = configurationPlan({
          ...submitted,
          catalog,
          characters: characters.value,
        });
        let completed = 0;
        for (const operation of plan) {
          if (!live()) return;
          saveStatus.value = `正在保存设置（${completed + 1} / ${plan.length}），等待服务端确认…`;
          const body = {
            ...operation.body,
            ...(operation.operation === "autonomy"
              ? {}
              : { expectedVersions: { ...versions } }),
          };
          if (operation.operation === "endpoint")
            body.expectedVersions["endpoint:" + body.endpointId] ??= 0;
          if (operation.operation === "binding") {
            body.expectedVersions["binding:" + body.role] ??= 0;
            body.expectedVersions["endpoint:" + body.endpointId] ??= 0;
          }
          let result;
          if (operation.operation === "autonomy") {
            result = await writePolicy(operation);
          } else
            result = await write(operation.kind, body, (value, key) =>
              operation.operation === "endpoint"
                ? config.endpoint({ ...value, idempotencyKey: key })
                : operation.operation === "binding"
                  ? config.binding({ ...value, idempotencyKey: key })
                  : config.save({ ...value, idempotencyKey: key }),
            );
          if (!live()) return;
          acknowledgeConfiguration(
            {
              catalog,
              versions,
              settings: settings.value,
              endpoints: endpoints.value,
              bindings: bindings.value,
              quiet: quiet.value,
            },
            operation,
            result,
          );
          clearAcknowledgedCredentials(
            endpoints.value,
            submitted.endpoints,
            operation.body,
          );
          completed++;
        }

        if (live()) {
          saveStatus.value = plan.length ? "设置已保存；新任务使用当前有效配置。" : "当前设置已保存，没有新的修改。";
          w.notify(saveStatus.value);
        }
      } catch (cause) {
        if (live()) {
          saveStatus.value = "";
          error.value = errorMessage(
            cause,
            "设置未完全保存，已保存部分和当前草稿请分别核对。",
          );
        }
      } finally {
        if (live()) saving.value = false;
      }
    }
    async function test(target) {
      const endpoint=typeof target==='object'?target:null;
      const name=endpoint?(endpoint.name||'新服务'):target;
      testResult.value = endpoint ? '' : name + "：测试中…";
      if(endpoint)endpoint.feedback='正在读取已保存连接的模型目录…';
      try {
        if(endpoint&&!connectionIsSaved(endpoint)){
          endpoint.feedback='请先保存服务地址和访问密钥，再读取已保存连接的模型目录。';
          return;
        }
        const source=endpoint?._source;
        const result = endpoint
          ? await config.models(endpoint._source.id)
          : await config.test(
              name === "高级"
                ? "kev"
                : name === "看图理解"
                  ? "vision"
                  : "media",
            );
        if (live()) {
          if(endpoint&&(!connectionIsSaved(endpoint)||endpoint._source!==source)){
            endpoint.feedback='连接输入已修改，请保存后重新读取目录。';return;
          }
          if(endpoint){endpoint.models=result.models||[];endpoint.feedback='已读取模型目录 '+endpoint.models.length+' 项';return;}
          testResult.value =
            name +
            "：" +
            (endpoint
              ? "已读取模型目录 " + (result.models || []).length + " 项"
              : JSON.stringify(result));
        }
      } catch (cause) {
        if (live()) {if(endpoint)endpoint.feedback=errorMessage(cause);else testResult.value = name + "：" + errorMessage(cause);}
      }
    }
    function retry(record) {
      if (record._record?.kind === "media-prompts") {
        w.navigatePath(
          "/diag/media-prompts?q=" + encodeURIComponent(record._record.data.id),
        );
        return;
      }
      w.navigatePath(
        "/diag/record/" +
          encodeURIComponent(record._record.kind) +
          "/" +
          encodeURIComponent(record._record.ref),
      );
    }
    let quietRead = 0;
    watch(
      () => quiet.value.character,
      async (target) => {
        const token = ++quietRead;
        if (!catalog) return;
        try {
          let hours, policy;
          if (target === "全部角色")
            hours =
              catalog.effective_values["product.autonomy"]?.quiet_hours || {};
          else {
            const matches = characters.value.filter(
              (c) => c.id === target || c.name === target,
            );
            if (matches.length !== 1) throw Error("无法唯一确定所选角色");
            policy = await w.client.autonomyPolicy(matches[0].id);
            if (policy.fluctlight_id !== matches[0].id)
              throw Error("角色安静策略范围不匹配");
            hours = policy.quiet_hours;
          }
          if (!live() || token !== quietRead) return;
          quiet.value = {
            enabled: !!hours.start,
            start: hours.start || "",
            end: hours.end || "",
            days: (hours.days || [0, 1, 2, 3, 4, 5, 6]).map((d) => weekdays[d]),
            behavior: "暂停主动联系",
            character: target,
            _source: hours,
            ...(policy ? { _policy: policy } : {}),
          };
        } catch (cause) {
          if (live() && token === quietRead) error.value = errorMessage(cause);
        }
      },
    );
    watch(reference, async (title) => {
      const section = referenceSections.value.find((s) => s.title === title);
      if (!section) return;
      section.rows = ["正在读取实际记录…"];
      try {
        const page = await diagnostics.page(section.kind, {
          limit: 20,
          ...(title === "待处理事项 / 重试耗尽"
            ? { status: "dead_letter" }
            : {}),
        });
        if (!live() || reference.value !== title) return;
        section.rows = page.items.map((record) => {
          const data = record.data;
          return [
            domainName(
              data.event_type ||
                data.intent_type ||
                data.agent_id ||
                data.boundary,
              "运行记录",
            ),
            data.status || data.payload?.status || "",
            record.occurred_at ? userInstant(record.occurred_at) : "时间未记录",
          ]
            .filter(Boolean)
            .join(" · ");
        });
        if (!section.rows.length) section.rows = ["当前没有对应记录。"];
      } catch (cause) {
        if (live() && reference.value === title)
          section.rows = [errorMessage(cause)];
      }
    });
    watch([endpoints,bindings],()=>{
      for(const endpoint of endpoints.value)if(!connectionIsSaved(endpoint)&&endpoint.models?.length)endpoint.models=[];
      if(live()&&loaded)sessionStorage.setItem(connectionDraftKey(scope),JSON.stringify(connectionDraft({endpoints:endpoints.value,bindings:bindings.value,versions})));
    },{deep:true});
    onMounted(() => {
      void readCatalog().catch((e) => (error.value = errorMessage(e)));
      void loadDiagnostics().catch((e) => (error.value = errorMessage(e)));
    });
    onUnmounted(() => {
      active = false;
      for (const endpoint of endpoints.value) endpoint.secret = "";
    });
    const render = renderManagementPage({
      configForm,
      a: tab,
      U: sections,
      ee: scrollTo,
      R: save,
      _: sectionId,
      ye: () =>
        endpoints.value.push({
          id: "new-" + randomId(),
          name: "",
          protocol: "openai-compatible",
          url: "",
          secret: "",
          clearSecret: false,
          models: [],
          status: "未配置",
        }),
      l: endpoints,
      x: test,
      i: bindings,
      r: quiet,
      characters,
      n: settings,
      te: testResult,
      A: error,
      w: saving,
      saveStatus,
      u: kind,
      q: (value) => {
        kind.value = value;
        clear();
        selectedId.value = null;
        highlight.value = null;
      },
      FilterChips,
      f: search,
      c: filters,
      le: filterGroups,
      K: rows,
      C: highlight,
      V: (record) => {
        selectedId.value = record.id;
        highlight.value = record.id;
      },
      he: (record) =>
        record.start
          ? userInstant(new Date(record.start).toISOString())
          : "时间未记录",
      I: statusLabel,
      T: retry,
      H: clear,
      S: selected,
      DetailDrawer,
      d: selectedId,
      B: reference,
      j: computed(() =>
        referenceSections.value.find(
          (section) => section.title === reference.value,
        ),
      ),
      referenceSections,
      removeEndpoint: (index) => {
        if (endpoints.value[index]?._source) {
          error.value =
            "已保存连接保留在服务端，可修改地址或重新绑定用途；没有删除连接。";
          return;
        }
        endpoints.value.splice(index, 1);
      },
      sharedScenarios,
      ModelDirectory,
      SettingsField,
      openTaskDiagnostics: () => w.navigatePath("/diag/pending"),
    });
    const clockTimer = setInterval(() => {
      if (
        live() &&
        !document.hidden &&
        records.value.some((r) => r.status === "pending")
      )
        records.value = records.value.map((r) =>
          r.status === "pending"
            ? diagnosticProjection(
                r._record,
                r.kind === "媒体调用" ? 1 : 0,
                characters.value,
              )
            : r,
        );
    }, 1000);
    onUnmounted(() => clearInterval(clockTimer));
    onMounted(
      () =>
        void nextTick(() => {
          if (w.state.managementSection) scrollTo(w.state.managementSection);
        }),
    );
    return (context, cache) =>
      h(Fragment, [
        render(context, cache),
        error.value
          ? h("aside", { class: "approved-recovery", role: "status" }, [
              policyRecovery.value &&
              policyRecovery.value.channel.pending() &&
              !policyRecovery.value.channel.pending().rejection
                ? h(
                    "button",
                    { class: "button", onClick: retryPolicy },
                    "重试原角色设置",
                  )
                : null,
              recovery.value
                ? h(
                    "button",
                    { class: "button", onClick: () => recoverOriginal() },
                    "核对原提交",
                  )
                : null,
              recovery.value
                ? h(
                    "button",
                    { class: "button", onClick: () => recoverOriginal(true) },
                    "停止未受理提交",
                  )
                : null,
              h(
                "button",
                { class: "button", onClick: rebase },
                "读取最新基线（保留输入）",
              ),
            ])
          : null,
      ]);
  },
});
