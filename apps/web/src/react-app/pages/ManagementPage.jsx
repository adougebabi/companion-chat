import {configurationTestSummary} from '@/v3/configuration-feedback.mjs';

import { useController, afterPaint } from "@/react-app/page";
import { selector } from "@/state/view-model";
import { ActionButton,Badge } from "@/react-app/controls";
import {ArrowLeft} from "lucide-react";
import { Fragment } from "@/react-app/render-helpers";
import { ManagementPageView } from "@/react-app/views/ManagementPage";
import { FilterChips, DetailDrawer } from "@/react-app/components";
import { diagnosticProjection } from "@/approved-ui/presentation.mjs";
import { catalogLabel } from "@/v3/runtime-catalog.mjs";
import ModelDirectory from "@/react-app/shared/ModelDirectory";
import SettingsField from "@/react-app/shared/SettingsField";
import { configurationFields, conversationReplyDisplayValue } from "@/v3/proto/components/configuration-fields.js";
import {
  acknowledgeConfiguration,
  clearAcknowledgedCredentials,
} from "@/approved-ui/configuration-ack.mjs";
import { persistentCommand } from "@/v3/providers/persistent-command.mjs";
import { configurationPlan } from "@/approved-ui/configuration-plan.mjs";
import {
  connectionDraftKey,
  connectionDraft,
  connectionDraftDirty,
  restoreConnectionDraft,
  connectionIsSaved,
} from "@/approved-ui/configuration-draft.mjs";
import { errorMessage, domainName, stateLabel, feedbackDiagnostic } from "@/v3/domain-presentation.mjs";
import { randomId } from "@/random-id.mjs";
import { userInstant } from "@/v3/presentation-time.mjs";
import {diagnosticInstant} from "@/approved-ui/task-presentation.mjs";
import { installFieldValidation } from "@/v3/proto/components/forms.js";
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
  if (settingKey === "conversation.reply") values = conversationReplyDisplayValue(values);
  const raw = values?.[field.key],
    choices = (field.options || []).map((o) =>
      typeof o === "string"
        ? {
            value: o,
            label: o,
          }
        : o,
    );
  let type =
    field.type === "boolean"
      ? "checkbox"
      : field.type === "number"
        ? "number"
        : field.type === "select"
          ? "select"
          : ["workflow", "json", "quiet-hours"].includes(
                field.type,
              )
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
function editableConfiguration(settings,endpoints,bindings,quiet){const value=field=>{if(field.type==='json'){try{return JSON.parse(field.value);}catch{return field.value;}}if(field.type==='number'&&field.value!=='')return Number(field.value);if(field.type==='select')return field._choices?.find(choice=>choice.label===field.value)?.value??field.value;return field.value;};return JSON.stringify({settings:settings.map(group=>group.fields.map(value)),endpoints:endpoints.map(e=>[e.id,e.name,e.protocol,e.url.replace(/\/+$/,''),e.secret,e.clearSecret]).sort((a,b)=>a[0].localeCompare(b[0])),bindings:bindings.map(b=>[b.role,b.endpoint,b.model,b.timeout]).sort((a,b)=>a[0].localeCompare(b[0])),quiet:[quiet.enabled,quiet.start,quiet.end,quiet.days,quiet.character,quiet.behavior]});}

function createManagementPageController(props, context, lifecycle) {
  const w = lifecycle.workspace,
    config = w.services.configurationClient,
    diagnostics = w.services.diagnosticsClient,
    scope = w.state.session?.actorId;
  let active = true,
    catalog = null,
    versions = {},
    loaded = false;
  const live = () => active && w.state.session?.actorId === scope;
  const tab = selector({
      get: () => w.state.managementTab,
      set: (v) => (w.state.managementTab = v),
    }),
    kind = lifecycle.cell("模型调用"),
    search = lifecycle.cell(""),
    filters = lifecycle.cell({
      type: "",
      who: "",
      status: "",
    }),
    selectedId = lifecycle.cell(null),
    highlight = lifecycle.cell(null),
    saving = lifecycle.cell(false),
    saveStatus = lifecycle.cell(""),
    saveKind=lifecycle.cell('information'),recoveringConfiguration=lifecycle.cell(false),
    needsBaselineReview=lifecycle.cell(false),
    configurationStorageProblem=lifecycle.cell(null),
    validationPending=lifecycle.cell(false),
    error = lifecycle.cell(""),
    reference = lifecycle.cell(""),
    testResult = lifecycle.cell(""),testRaw=lifecycle.cell(null),testKind=lifecycle.cell('information'),
    currentSection=lifecycle.cell("服务地址");
  const catalogLoading=lifecycle.cell(true),catalogReady=lifecycle.cell(false);
  const settings = lifecycle.cell([]),
    endpoints = lifecycle.cell([]),
    bindings = lifecycle.cell([]),
    quiet = lifecycle.cell({
      enabled: false,
      start: "",
      end: "",
      days: [],
      behavior: "暂停主动联系",
      character: "全部角色",
    }),
    records = lifecycle.cell([]),
    diagnosticPages=lifecycle.cell({"模型调用":{cursor:"",loading:true,error:"",ready:false},"媒体调用":{cursor:"",loading:true,error:"",ready:false}}),
    referenceRecord=lifecycle.cell(null),
    pending = new Map(),
    recovery = lifecycle.cell(null);
  const configForm = lifecycle.cell(null);
  let sectionObserver=null;
  function observeSections(){sectionObserver?.disconnect();}
  lifecycle.onDispose(()=>sectionObserver?.disconnect());
  lifecycle.observe(
    configForm,
    (node, _old, cleanup) => {
      if (node){cleanup(installFieldValidation(node));void afterPaint(()=>{if(live()&&configForm.value===node)observeSections();});}else sectionObserver?.disconnect();
    },
    {
      flush: "post",
    },
  );
  const policyChannels = new Map(),
    policyRecovery = lifecycle.cell(null);
  function policyChannel(fid) {
    let channel = policyChannels.get(fid);
    if (!channel) {
      channel = persistentCommand({
        key:
          "fluctlight:approved-policy:" +
          JSON.stringify([scope, fid]),
        storage: () => sessionStorage,
        execute: async (value, key) => {
          if (!live()) throw Error("所有者已变化");
          if(value.fid!==fid)throw Error('原角色设置与恢复入口不匹配，当前未发起新提交。');
          return w.client.saveAutonomyPolicy(value.fid, {
            ...value.body,
            idempotencyKey: key,
          });
        },
      });
      policyChannels.set(fid, channel);
    }
    return channel;
  }
  async function writePolicy(operation) {
    const channel=policyChannel(operation.fid);
    try {
      return await channel.submit({
        fid: operation.fid,
        body: operation.body,
      });
    } catch (cause) {
      try{const record=channel.pending();policyRecovery.value=record?{channel,fid:operation.fid,record}:null;}
      catch(storageCause){recordConfigurationStorageProblem(storageCause);}
      throw cause;
    }
  }
  async function retryPolicy() {
    const original = policyRecovery.value;
    if (!original||recoveringConfiguration.value) return;
    recoveringConfiguration.value=true;error.value='';saveKind.value='progress';saveStatus.value='正在核对原角色设置，当前输入保留…';
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
      error.value = '';
      saveKind.value='success';saveStatus.value="原角色设置已确认，当前输入保留。";
    } catch (cause) {
      if (live()){error.value = errorMessage(cause);saveStatus.value='';}
    } finally {if(live())recoveringConfiguration.value=false;}
  }
  const characters = selector(() => w.state.characters);
  const referenceSections = lifecycle.cell(
    [
      ["逻辑运行", "agent-runs"],
      ["后台任务", "workflow-intents"],
      ["生命周期", "lifecycle"],
      ["回合诊断", "turns"],
      ["Kev 决策", "kev"],
      ["系统事件与主动触达", "events"],
      ["重试耗尽的后台任务", "workflow-intents"],
      ["技术详情", "links"],
      ["安全拦截", "harm"],
    ].map(([title, kind]) => ({
      title,
      kind,
      rows: [],
    })),
  );
  const selected = selector(() =>
    records.value.find((r) => r.id === selectedId.value&&r.kind===kind.value),
  );
  const statusLabel = (status) =>
    ({
      error: "失败",
      pending: "处理中",
      success: "完成",
    })[status] || "状态待确认";
  const rows = selector(() =>
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
      ),
  );
  const filterGroups = selector(() => [
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
    {
      key: "status",
      label: "状态",
      options: ["完成", "失败", "处理中"],
    },
  ]);
  const clear = () => {
    search.value = "";
    filters.value = {
      type: "",
      who: "",
      status: "",
    };
  };
  const sections = selector(() => [
    "服务地址",
    "模型用途",
    "安静时段",
    ...settings.value.map((g) => g.title),
  ]);
  const sectionId = (value) => "config-" + value;
  const scrollTo = async (value, focus = true) => {
    currentSection.value=value;
    const node = document.getElementById(sectionId(value));
    const trigger = node?.querySelector("[data-slot=collapsible-trigger]");
    if (trigger?.getAttribute("aria-expanded") === "false") trigger.click();
    await afterPaint();
    if(focus)node?.scrollIntoView({
      behavior: "smooth",
      block: "start",
    });
    const heading = node?.querySelector("[data-slot=collapsible-trigger],h2");
    if (focus && heading) {
      if (heading.tagName === "H2") heading.tabIndex = -1;
      heading.focus({
        preventScroll: true,
      });
    }
  };
  lifecycle.observe(()=>w.state.managementSection,section=>{if(section&&tab.value==='配置中心')void afterPaint(()=>{if(live())scrollTo(section);});});
  const sharedScenarios = selector(() =>
    (w.state.runtimeCatalog?.agents || [])
      .filter((a) =>
        a.binding_role
          ? a.binding_role === "generic_llm"
          : [
              "cognitive_assessment",
              "generic_llm",
              "media_prompt",
              "initialization",
              "reflection",
              "interaction",
              "action_realization",
              "visual_identity_patch",
            ].includes(a.role),
      )
      .map((a) => catalogLabel("agent", a.name, a.description)),
  );
  function confirmedEditable(){if(!catalog)return '';const hours=quiet.value._source||catalog.effective_values['product.autonomy']?.quiet_hours||{};return editableConfiguration(settings.value.map(group=>({...group,fields:group.fields.map(field=>uiField(field._settingKey,field._definition,catalog.effective_values[field._settingKey]))})),catalog.endpoints.map(endpoint=>({id:endpoint.id,name:endpoint.id,protocol:endpoint.kind,url:endpoint.base_url,secret:'',clearSecret:false})),Object.entries(roleNames).map(([role])=>{const saved=catalog.bindings.find(binding=>binding.role===role);return {role,endpoint:saved?.endpoint_id||'',model:saved?.model_id||'',timeout:saved?.timeout_seconds??''};}),{...quiet.value,enabled:!!hours.start,start:hours.start||'',end:hours.end||'',days:(hours.days||[0,1,2,3,4,5,6]).map(day=>weekdays[day])});}
  async function readCatalog() {
    catalogLoading.value=true;
    const snapshot = await config.read();
    if (!live()) return;
    error.value='';needsBaselineReview.value=false;
    snapshot._loadedAt = Date.now();
    catalog = snapshot;
    versions = {
      ...snapshot.versions,
    };
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
    const restored = restoreConnectionDraft(
      sessionStorage.getItem(connectionDraftKey(scope)),
      snapshot,
    );
    if (restored?.dirty) {
      endpoints.value = restored.endpoints;
      for (const binding of bindings.value) {
        const saved = restored.bindings.find((b) => b.role === binding.role);
        if (saved) Object.assign(binding, saved);
      }
      versions = {
        ...restored.versions,
      };
      if (restored.stale){
        needsBaselineReview.value=true;
        error.value =
          "配置已经更新。未保存的连接草稿已恢复，请读取最新基线并核对后保存。";
      }else if (restored.credentialPending){
        saveKind.value='information';saveStatus.value=
          "连接草稿已恢复。访问密钥不会保存在浏览器草稿中，请重新填写。";
      }
    }else if(restored)sessionStorage.removeItem(connectionDraftKey(scope));
    inspectConfigurationRecords();
    loaded = true;catalogReady.value=true;catalogLoading.value=false;
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
      e.modelsStatus="loading";
      const token=e.modelsEpoch=(e.modelsEpoch||0)+1,source=e._source;
      void config
        .models(e.id)
        .then((result) => {
          if (live()&&endpoints.value.includes(e)&&token===e.modelsEpoch&&e._source===source&&connectionIsSaved(e)){e.models = result.models || [];e.modelsStatus="ready";e.modelsError="";e.modelsReadAt=new Date().toISOString();}
        })
        .catch(cause => {if(live()&&endpoints.value.includes(e)&&token===e.modelsEpoch&&e._source===source){e.modelsStatus=connectionIsSaved(e)?"error":"stale";e.modelsError=connectionIsSaved(e)?errorMessage(cause):"";}});
    }
    void config
      .catalog()
      .then((value) => {
        if (live()) w.state.runtimeCatalog = value;
      })
      .catch(() => {});
    await afterPaint();
    if (live()){observeSections();scrollTo(w.state.managementSection || "服务地址", false);}
  }
  const diagnosticCorrelation=lifecycle.cell(new URLSearchParams((globalThis.location?.hash||" ").split("?")[1]||"").get("correlationId")||"");
  let diagnosticEpoch=0;
  async function loadDiagnostics() {
    const token=++diagnosticEpoch,correlationId=diagnosticCorrelation.value,filter=correlationId?{correlationId}:{};
    const [models, media] = await Promise.allSettled([
      diagnostics.page("model-runs", {
        ...filter,limit: 20,
      }),
      diagnostics.page("media-prompts", {
        ...filter,limit: 20,
      }),
    ]);
    if (!live()||token!==diagnosticEpoch||correlationId!==diagnosticCorrelation.value) return;
    const projected = [];
    for (const [index, result] of [models, media].entries()) {
      if (result.status === "rejected") {
        const pageState=diagnosticPages.value[index?"媒体调用":"模型调用"];pageState.loading=false;pageState.error=errorMessage(result.reason);
        projected.push(...records.value.filter(record=>record.kind===(index?"媒体调用":"模型调用")));
        continue;
      }
      Object.assign(diagnosticPages.value[index?"媒体调用":"模型调用"],{cursor:result.value.next_cursor||"",loading:false,error:"",ready:true});
      for (const record of result.value.items) {
        projected.push(diagnosticProjection(record, index, characters.value));
      }
    }
    records.value = projected;
    w.state.diagnostics = projected;
  }
  async function loadMoreDiagnostics(){
    const selectedKind=kind.value,state=diagnosticPages.value[selectedKind],cursor=state.cursor,correlationId=diagnosticCorrelation.value,token=diagnosticEpoch;
    if(!state.ready&&state.error){await loadDiagnostics();return;}if(state.loading||!cursor)return;state.loading=true;state.error='';
    const current=()=>live()&&token===diagnosticEpoch&&correlationId===diagnosticCorrelation.value;
    try{const page=await diagnostics.page(selectedKind==='模型调用'?'model-runs':'media-prompts',{limit:20,cursor,...(correlationId?{correlationId}:{})});if(!current())return;const seen=new Set(records.value.filter(record=>record.kind===selectedKind).map(record=>record.id));records.value=[...records.value,...page.items.filter(record=>!seen.has(record.ref)).map(record=>diagnosticProjection(record,selectedKind==='媒体调用'?1:0,characters.value))];state.cursor=page.next_cursor||'';}
    catch(cause){if(current())state.error=errorMessage(cause);}finally{if(current())state.loading=false;}
  }
  function diagnosticRouteChanged(){const value=new URLSearchParams((globalThis.location?.hash||'').split('?')[1]||'').get('correlationId')||'';if(value===diagnosticCorrelation.value)return;diagnosticCorrelation.value=value;selectedId.value=null;reference.value='';referenceEpoch++;referenceDetailEpoch++;referenceRecord.value=null;for(const section of referenceSections.value)Object.assign(section,{rows:[],nextCursor:'',loading:false,error:'',failedMore:false});records.value=[];clear();for(const state of Object.values(diagnosticPages.value))Object.assign(state,{loading:true,ready:false,error:'',cursor:''});void loadDiagnostics();}
  lifecycle.onStart(()=>window.addEventListener('hashchange',diagnosticRouteChanged));
  lifecycle.onDispose(()=>window.removeEventListener('hashchange',diagnosticRouteChanged));
  let referenceDetailEpoch=0;
  async function openReference(record,section){const token=++referenceDetailEpoch;referenceRecord.value={record,section,loading:true,error:'',detail:null};try{const detail=await diagnostics.record(section.kind,record.ref);if(live()&&token===referenceDetailEpoch&&referenceRecord.value)referenceRecord.value={record,section,loading:false,error:'',detail};}catch(cause){if(live()&&token===referenceDetailEpoch&&referenceRecord.value)referenceRecord.value={record,section,loading:false,error:errorMessage(cause),detail:null};}}
  function findConfigurationRecovery(){
    const prefixes=['fluctlight:approved-config-command:','fluctlight:approved-policy:'];
    for(let i=0;i<sessionStorage.length;i++){
      const keyName=sessionStorage.key(i),prefix=prefixes.find(prefix=>keyName?.startsWith(prefix));if(!prefix)continue;
      let identity;try{identity=JSON.parse(keyName.slice(prefix.length));}catch{continue;}
      if(!Array.isArray(identity)||identity.length!==2||identity[0]!==scope||typeof identity[1]!=='string')continue;
      const key=sessionStorage.getItem(keyName);if(key===null)continue;
      if(prefix===prefixes[0]){if(!key)throw Error('原设置提交的身份暂时无法读取，原记录已保留。');if(!recovery.value)recovery.value={key,keyName,kind:identity[1]};}
      else if(!policyRecovery.value){
        const fid=identity[1],channel=policyChannel(fid),record=channel.pending();
        if(!record||record.value.fid!==fid||!record.value.body||!Number.isSafeInteger(record.value.body.expectedRevision))throw Error('原角色设置记录无法安全恢复，原记录已保留。');
        policyRecovery.value={channel,fid,record};
      }
    }
  }
  function recordConfigurationStorageProblem(cause){configurationStorageProblem.value=feedbackDiagnostic(cause);error.value='暂时无法读取本机的原设置提交记录，当前输入已保留。恢复浏览器存储访问后，请重新读取原提交记录。';saveStatus.value='';}
  function inspectConfigurationRecords(){try{findConfigurationRecovery();configurationStorageProblem.value=null;return true;}catch(cause){recordConfigurationStorageProblem(cause);return false;}}
  function rereadConfigurationRecords(){if(!live()||recoveringConfiguration.value)return;error.value='';if(inspectConfigurationRecords()&&!recovery.value&&!policyRecovery.value){saveKind.value='information';saveStatus.value='本机提交记录已重新读取，当前输入保留。';}}
  async function write(kind, body, run) {
    const keyName =
      "fluctlight:approved-config-command:" + JSON.stringify([scope, kind]);
    let value = pending.get(kind);
    if (!value) {
      const restored = sessionStorage.getItem(keyName);
      if (restored) {
        recovery.value = {
          key: restored,
          keyName,
          kind,
        };
        throw Error(
          "有原配置提交等待核对，请先恢复原回执或停止尚未受理的提交；当前草稿已保留。",
        );
      }
      value = {
        body: clone(body),
        key: randomId(),
      };
      pending.set(kind, value);
      sessionStorage.setItem(keyName, value.key);
    }
    if (JSON.stringify(value.body) !== JSON.stringify(body)) {
      recovery.value = {
        key: value.key,
        keyName,
        kind,
      };
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
      recovery.value = {
        key: value.key,
        keyName,
        kind,
        status:cause.status,
        failure:feedbackDiagnostic(cause),
      };
      throw cause;
    }
  }
  async function recoverOriginal(stop = false) {
    const original = recovery.value;
    if (!original||recoveringConfiguration.value) return;
    recoveringConfiguration.value=true;error.value='';saveKind.value='progress';saveStatus.value='正在核对原设置提交，原输入和身份保持…';
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
      if(!inspectConfigurationRecords())return;
      needsBaselineReview.value=true;
      validationPending.value=false;
      error.value='';
      saveStatus.value="原提交已核对。请读取最新基线，保留当前输入并核对后再次保存。";
      saveKind.value='information';
    } catch (cause) {
      if (live()) error.value = errorMessage(cause);
    }finally{if(live())recoveringConfiguration.value=false;}
  }
  async function rebase() {
    if(recoveringConfiguration.value||configurationStorageProblem.value||!inspectConfigurationRecords())return;
    if (recovery.value) {
      error.value = "请先核对原提交，再读取最新基线";
      return;
    }
    if (policyRecovery.value?.record&&!policyRecovery.value.record.rejection) {
      error.value = "原角色设置结果尚未确认，请先重试原角色设置";
      return;
    }
    recoveringConfiguration.value=true;error.value='';saveKind.value='progress';saveStatus.value='正在读取最新基线，当前输入保留…';
    try {
      const next = await config.read();
      if (!live()) return;
      catalog = next;
      versions = {
        ...next.versions,
      };
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
      needsBaselineReview.value=false;validationPending.value=false;error.value='';
      saveStatus.value = "已读取最新基线，当前输入保持不变；请核对后保存。";
      saveKind.value='information';
    } catch (cause) {
      if (live()) error.value = errorMessage(cause);
    }finally{if(live())recoveringConfiguration.value=false;}
  }
  async function save() {
    if (saving.value || !catalog || recovery.value||policyRecovery.value||needsBaselineReview.value||recoveringConfiguration.value||configurationStorageProblem.value) return;
    if(!inspectConfigurationRecords()||recovery.value||policyRecovery.value)return;
    saving.value = true;
    saveStatus.value = "正在保存设置，等待服务端确认…";
    saveKind.value='progress';
    error.value = "";
    validationPending.value=false;
    for(const group of settings.value)for(const field of group.fields)field.validationError='';
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
            : {
                expectedVersions: {
                  ...versions,
                },
              }),
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
              ? config.endpoint({
                  ...value,
                  idempotencyKey: key,
                })
              : operation.operation === "binding"
                ? config.binding({
                    ...value,
                    idempotencyKey: key,
                  })
                : config.save({
                    ...value,
                    idempotencyKey: key,
                  }),
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
        testReadEpoch++;testResult.value='';testRaw.value=null;testKind.value='information';
        clearAcknowledgedCredentials(
          endpoints.value,
          submitted.endpoints,
          operation.body,
        );
        completed++;
      }
      if (live()) {
        saveKind.value=plan.length?'success':'information';
        saveStatus.value = plan.length
          ? "设置已保存；新任务使用当前有效配置。"
          : "当前设置已保存，没有新的修改。";
      }
    } catch (cause) {
      if (live()) {
        saveStatus.value = "";
        const group=cause.settingField&&settings.value.find(group=>group.fields.some(field=>field._settingKey===cause.settingField.settingKey&&field._key===cause.settingField.key));
        if(group){
          const field=group.fields.find(field=>field._settingKey===cause.settingField.settingKey&&field._key===cause.settingField.key);
          // settingField is attached only by our local form validator. Its
          // authored copy can name required JSON keys without diagnostic masking.
          field.validationError=cause.message;error.value='';validationPending.value=true;saveKind.value='information';saveStatus.value='请修正标记的设置项后保存。';
          await scrollTo(group.title,false);if(!live())return;
          const identity=JSON.stringify([field._settingKey,field._key]),host=[...document.querySelectorAll('[data-setting-field]')].find(node=>node.getAttribute('data-setting-field')===identity);
          host?.scrollIntoView({block:'center',behavior:'instant'});
          const input=host?.querySelector('input:not([type=hidden]),textarea,[role=combobox]')||host?.querySelector('button');input?.focus({preventScroll:true});
        }else error.value = errorMessage(
          cause,
          "设置未完全保存，已保存部分和当前草稿请分别核对。",
        );
      }
    } finally {
      if (live()) saving.value = false;
    }
  }
  let testReadEpoch=0;
  async function test(target) {
    const endpoint = typeof target === "object" ? target : null;
    const name = endpoint ? endpoint.name || "新服务" : target,token=endpoint?(endpoint.modelsEpoch=(endpoint.modelsEpoch||0)+1):++testReadEpoch;
    if(!endpoint)testRaw.value=null;
    testResult.value = endpoint ? "" : name + "：测试中…";
    if(!endpoint)testKind.value='progress';
    if (endpoint){endpoint.feedback = "";endpoint.modelsStatus="loading";endpoint.modelsError="";}
    try {
      if (endpoint && !connectionIsSaved(endpoint)) {
        endpoint.modelsStatus="stale";
        endpoint.feedback =
          "请先保存服务地址和访问密钥，再读取已保存连接的模型目录。";
        return;
      }
      const source = endpoint?._source;
      const result = endpoint
        ? await config.models(endpoint._source.id)
        : await config.test(
            name === "高级" ? "kev" : name === "看图理解" ? "vision" : "media",
          );
      if (live()) {
        if(endpoint&&token!==endpoint.modelsEpoch)return;
        if(!endpoint&&token!==testReadEpoch)return;
        if (
          endpoint &&
          (!connectionIsSaved(endpoint) || endpoint._source !== source)
        ) {
          endpoint.modelsStatus="stale";
          endpoint.feedback = "连接输入已修改，请保存后重新读取目录。";
          return;
        }
        if (endpoint) {
          endpoint.models = result.models || [];
          endpoint.modelsStatus="ready";endpoint.modelsReadAt=new Date().toISOString();
          endpoint.feedback =
            "已读取模型目录 " + endpoint.models.length + " 项";
          return;
        }
        testRaw.value=result;
        const summary=configurationTestSummary(result);
        testKind.value=summary.state==='success'?'success':summary.state==='failed'?'error':'warning';
        testResult.value =
          name +
          "：" +
          (endpoint
            ? "已读取模型目录 " + (result.models || []).length + " 项"
            : summary.text);
      }
    } catch (cause) {
      if (live()) {
        if(endpoint&&token!==endpoint.modelsEpoch)return;
        if (endpoint){endpoint.feedback = "";endpoint.modelsStatus="error";endpoint.modelsError=errorMessage(cause);}
        else if(token===testReadEpoch){testRaw.value={status:"failed",reason:errorMessage(cause)};testKind.value='error';testResult.value = name + "：" + errorMessage(cause);}
      }
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
  lifecycle.observe(
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
          ...(policy
            ? {
                _policy: policy,
              }
            : {}),
        };
      } catch (cause) {
        if (live() && token === quietRead) error.value = errorMessage(cause);
      }
    },
  );
  let referenceEpoch=0;
  async function readReference(title,more=false){
    const section=referenceSections.value.find(s=>s.title===title);if(!section){referenceEpoch++;return;}if(more&&(section.loading||!section.nextCursor))return;const token=++referenceEpoch,correlationId=diagnosticCorrelation.value;
    const current=()=>live()&&reference.value===title&&token===referenceEpoch&&correlationId===diagnosticCorrelation.value;
    section.loading=true;section.error='';
    try{const page=await diagnostics.page(section.kind,{limit:20,...(correlationId?{correlationId}:{}),...(more?{cursor:section.nextCursor}:{}),...(title==='重试耗尽的后台任务'?{status:'dead_letter'}:{})});if(!current())return;section.rows=[...new Map([...(more?section.rows:[]),...page.items].map(record=>[record.ref,record])).values()];section.nextCursor=page.next_cursor||'';}
    catch(cause){if(current()){section.error=errorMessage(cause);section.failedMore=more;}}
    finally{if(current())section.loading=false;}
  }
  lifecycle.observe(reference,title=>void readReference(title));
  lifecycle.observe(
    [endpoints, bindings],
    () => {
      for (const endpoint of endpoints.value)
        if (!connectionIsSaved(endpoint)){
          if(endpoint.models?.length)endpoint.models=[];
          if(endpoint._source)endpoint.modelsStatus="stale";
        }
      if (live() && loaded){
        const draft=connectionDraft({endpoints:endpoints.value,bindings:bindings.value,versions});
        try{if(connectionDraftDirty(draft,catalog))sessionStorage.setItem(connectionDraftKey(scope),JSON.stringify(draft));else sessionStorage.removeItem(connectionDraftKey(scope));}
        catch(cause){recordConfigurationStorageProblem(cause);}
      }
    },
    {
      deep: true,
    },
  );
  lifecycle.onStart(() => {
    void readCatalog().catch((e) => {if(live()){error.value=errorMessage(e);catalogLoading.value=false;}});
    void loadDiagnostics().catch((e) => (error.value = errorMessage(e)));
  });
  lifecycle.observe(()=>settings.value.some(group=>group.fields.some(field=>!!field.validationError)),invalid=>{if(!invalid&&validationPending.value){validationPending.value=false;saveStatus.value='';}});
  lifecycle.observe([saveStatus,saveKind],([text,kind],_old,cleanup)=>{
    if(!text||!['success','information'].includes(kind)||validationPending.value||needsBaselineReview.value||recovery.value||policyRecovery.value)return;
    const timer=setTimeout(()=>{if(live()&&saveStatus.value===text&&saveKind.value===kind&&!needsBaselineReview.value&&!recovery.value&&!policyRecovery.value)saveStatus.value='';},6000);
    cleanup(()=>clearTimeout(timer));
  });
  lifecycle.onDispose(() => {
    active = false;
    for (const endpoint of endpoints.value) endpoint.secret = "";
  });
  const returnTo = new URLSearchParams((globalThis.location?.hash || "").split("?")[1] || "").get("returnTo") || "";
  const render = () => (
    <ManagementPageView
      view={{
        accountActions:()=>(<div className="flex flex-wrap items-center justify-end gap-3">{ /^#?\/(?!\/)/.test(returnTo) && <ActionButton variant="outline" type="button" onClick={()=>w.navigatePath(returnTo.replace(/^#/,''))}><ArrowLeft size={14}/>{returnTo.replace(/^#/,'').startsWith('/library/create')?'返回初始化流程':/^#?\/c\/[^/]+\/chat/.test(returnTo)?'返回聊天':'返回来源页'}</ActionButton>}<Badge variant="outline">我的工作区</Badge></div>),
        catalogLoading,catalogReady,retryCatalog:()=>{void readCatalog().catch(cause=>{if(live()){catalogLoading.value=false;error.value=errorMessage(cause);}});},
        openData:()=>w.navigatePath("/data"),
        sectionLabel:value=>({"聊天回复":"回复行为","看图理解":"图片理解设置"})[value]||value,
        userInstant,diagnosticPages,loadMoreDiagnostics,referenceRecord,openReference,
        selectReference:value=>{reference.value=value;selectedId.value=null;referenceRecord.value=null;},
        loadMoreReference:()=>readReference(reference.value,true),
        retryReference:()=>readReference(reference.value,referenceSections.value.find(section=>section.title===reference.value)?.failedMore||false),
        referenceTitle:record=>domainName(record.data?.event_type||record.data?.intent_type||record.data?.agent_id||record.data?.boundary,({"agent-runs":"逻辑执行记录",events:"系统事件",links:"关联信息",kev:"辅助决策",harm:"安全拦截"})[record.kind]||"后台运行记录"),
        referenceStatus:record=>record.data?.status||record.data?.payload?.status?stateLabel(record.data?.status||record.data?.payload?.status):({events:"系统事件",links:"关联记录",harm:"拦截记录"})[record.kind]||"未记录状态",
        currentSection,
        dirty:selector(()=>loaded&&confirmedEditable()!==editableConfiguration(settings.value,endpoints.value,bindings.value,quiet.value)),
        canDiscard:selector(()=>!saving.value&&!recovery.value&&!policyRecovery.value&&!pending.size&&!configurationStorageProblem.value),
        discard:async()=>{if(saving.value||recovery.value||policyRecovery.value||pending.size||configurationStorageProblem.value||!inspectConfigurationRecords()||recovery.value||policyRecovery.value)return;validationPending.value=false;loaded=false;try{sessionStorage.removeItem(connectionDraftKey(scope));await readCatalog();saveKind.value='information';saveStatus.value="已放弃未保存修改";}catch(cause){if(live()){loaded=true;error.value=errorMessage(cause);}}},
        configForm,
        tab,
        sections,
        scrollTo,
        save,
        sectionId,
        addEndpoint: () =>
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
        endpoints,
        test,
        bindings,
        quiet,
        characters,
        settings,
        testRaw,testResult,testKind,
        error,
        recovery,policyRecovery,needsBaselineReview,configurationStorageProblem,RecoveryActions,recoveringConfiguration,saveKind,
        hasRecovery:selector(()=>!!recovery.value||!!policyRecovery.value||needsBaselineReview.value||!!configurationStorageProblem.value),
        saving,
        saveStatus,
        kind,
        selectKind: (value) => {
          kind.value = value;
          reference.value="";
          clear();
          selectedId.value = null;
          highlight.value = null;
        },
        FilterChips,
        search,
        filters,
        filterGroups,
        rows,
        highlight,
        selectRecord: (record) => {
          selectedId.value = record.id;
          highlight.value = record.id;
        },
        openMediaAsset: id=>w.navigatePath('/media/'+encodeURIComponent(id)),
        recordTime: (record) =>
          record.start
            ? diagnosticInstant(new Date(record.start).toISOString())
            : "时间未记录",
        statusLabel,
        retry,
        clear,
        selected,
        DetailDrawer,
        selectedId,
        reference,
        referenceSection: selector(() =>
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
      }}
    />
  );
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
  lifecycle.onDispose(() => clearInterval(clockTimer));
  lifecycle.onStart(
    () =>
      void afterPaint(() => {
        if (w.state.managementSection) scrollTo(w.state.managementSection, false);
      }),
  );
  function RecoveryActions(){
    if(configurationStorageProblem.value)return <ActionButton type="button" variant="outline" onClick={rereadConfigurationRecords}>重新读取本机提交记录</ActionButton>;
    const policy=policyRecovery.value?.record;
    return <div className="flex flex-wrap gap-2">
      {policy&&!policy.rejection&&<ActionButton type="button" variant="outline" disabled={recoveringConfiguration.value} onClick={retryPolicy}>重试原角色设置</ActionButton>}
      {recovery.value&&<><ActionButton type="button" variant="outline" disabled={recoveringConfiguration.value} onClick={()=>recoverOriginal()}>核对原提交</ActionButton><ActionButton type="button" variant="outline" disabled={recoveringConfiguration.value} onClick={()=>recoverOriginal(true)}>停止未受理提交</ActionButton></>}
      {!recovery.value&&(!policy||policy.rejection)&&<ActionButton type="button" variant="outline" disabled={recoveringConfiguration.value} onClick={rebase}>读取最新基线（保留输入）</ActionButton>}
    </div>;
  }
  return (context, cache) => (
    <Fragment>
      {render(context, cache)}
    </Fragment>
  );
}
export default function ManagementPage(input = {}) {
  return useController(createManagementPageController, {
    ...input,
  });
}
