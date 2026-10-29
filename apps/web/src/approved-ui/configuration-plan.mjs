import {configurationPoints} from '../v3/proto/components/configuration-fields.js';
import { providerBindingBody, defaultModelTimeoutSeconds } from "../v3/provider-budget.mjs";
const clone = (value) => JSON.parse(JSON.stringify(value));
export const weekdays = [
  "周日",
  "周一",
  "周二",
  "周三",
  "周四",
  "周五",
  "周六",
];
const equal = (a, b) => JSON.stringify(a) === JSON.stringify(b);

const workflowGraph=g=>g&&typeof g==='object'&&!Array.isArray(g)&&Object.keys(g).length>0&&Object.values(g).every(node=>node&&typeof node==='object'&&!Array.isArray(node)&&typeof node.class_type==='string'&&node.class_type.trim()&&node.inputs&&typeof node.inputs==='object'&&!Array.isArray(node.inputs));
function validateComfySetting(value){
 const invalid=(key,message)=>Object.assign(Error(message),{settingField:{settingKey:'media.comfyui',key}});
 let url;try{url=new URL(value.baseUrl||value.base_url);}catch{throw invalid('baseUrl','ComfyUI 地址：请填写完整的 HTTP(S) 服务地址');}
 if(!['http:','https:'].includes(url.protocol)||url.username||url.password||url.search||url.hash)throw invalid('baseUrl','ComfyUI 地址不能包含嵌入凭据、查询或片段');
 for(const [key,label] of [['workflow','默认 API 工作流'],['visual_identity_workflow','视觉身份工作流覆盖']])if((key==='workflow'||value[key]!==undefined)&&!workflowGraph(value[key]))throw invalid(key,label+'：工作流须为非空节点对象，每个节点包含 class_type 和 inputs');
 for(const [key,label] of [['visual_identity_workflows','视觉身份分阶段覆盖'],['media_workflows','媒体分支覆盖']])if(value[key]!==undefined&&(!value[key]||typeof value[key]!=='object'||Array.isArray(value[key])||!Object.values(value[key]).every(workflowGraph)))throw invalid(key,label+'：每个分支须包含有效的 class_type 和 inputs 节点');
}

/** Validate the entire immutable form snapshot before any configuration write. */
export function configurationPlan({
  catalog,
  settings,
  endpoints,
  bindings,
  quiet,
  characters,
}) {
  const writes = [],
    changedEndpoints = new Set(),
    endpointIds = new Map(),
    seen = new Set();
  for (const endpoint of endpoints) {
    const id = endpoint._source?.id || endpoint.name.trim();
    if (!id || !endpoint.url.trim()) throw Error("请填写完整的服务名称和地址");
    if (seen.has(id)) throw Error("服务名称不能重复");
    seen.add(id);
    endpointIds.set(endpoint.id, id);
    const url = new URL(endpoint.url);
    if (
      !["http:", "https:"].includes(url.protocol) ||
      url.username ||
      url.password ||
      url.search ||
      url.hash
    )
      throw Error("服务地址须为不带嵌入凭据、查询或片段的 http(s) URL");
    if (endpoint._source && endpoint.name !== endpoint._source.id)
      throw Error("已保存的服务标识不可改名，请新增连接后重新绑定用途");
    if (endpoint.secret && endpoint.clearSecret)
      throw Error("不能同时填写新密钥并清除同一密钥");
    const source = endpoint._source;
    if (
      !source ||
      source.base_url !== endpoint.url.replace(/\/+$/, "") ||
      source.kind !== endpoint.protocol
    ) {
      changedEndpoints.add(id);
      writes.push({
        kind: "endpoint:" + id,
        operation: "endpoint",
        formId: endpoint.id,
        body: {
          endpointId: id,
          kind: endpoint.protocol || "openai-compatible",
          baseUrl: endpoint.url,
          secretPurpose: source?.secret_purpose || "provider:" + id,
        },
      });
    }
    if (endpoint.secret || endpoint.clearSecret) {
      const purpose = source?.secret_purpose || "provider:" + id;
      writes.push({
        kind: "secret:" + purpose,
        operation: "settings",
        body: {
          ...(endpoint.secret
            ? { secrets: { [purpose]: endpoint.secret } }
            : {}),
          ...(endpoint.clearSecret ? { clearSecrets: [purpose] } : {}),
        },
      });
    }
  }
  for (const binding of bindings) {
    if (!binding.endpoint && !binding.model && !binding._source && (binding.timeout === "" || binding.timeout == null)) continue;
    const endpointId = endpointIds.get(binding.endpoint);
    if (!endpointId) throw Error("请为用途选择有效的服务地址");
    const source = binding._source,
      normalized = {
        role: binding.role,
        endpointId,
        modelId: binding.model.trim(),
        timeoutSeconds: binding.timeout === "" || binding.timeout == null ? defaultModelTimeoutSeconds : Number(binding.timeout),
      };
    if (
      !normalized.modelId ||
      !Number.isSafeInteger(normalized.timeoutSeconds) ||
      normalized.timeoutSeconds < 1
    )
      throw Error(binding.name + "：请核对模型和超时");
    const body = providerBindingBody(normalized, catalog.versions);
    delete body.expectedVersions;
    if (
      source &&
      !changedEndpoints.has(endpointId) &&
      source.endpoint_id === endpointId &&
      source.model_id === normalized.modelId &&
      source.timeout_seconds === normalized.timeoutSeconds
    )
      continue;
    writes.push({
      kind: "binding:" + binding.role,
      operation: "binding",
      body,
    });
  }
  const values = {};
  for (const group of settings)
    for (const field of group.fields) {
      try{
      let value = field.value;
      if (field._initial == null && (value === "" || value === false)) continue;
      if (field.type === "json") {
        if (!String(value).trim() && field._definition.optional)
          value = undefined;
        else {
          try {
            value = JSON.parse(value);
          } catch {
            throw Error(field.label + "：请填写有效 JSON");
          }
        }
      } else if(field.type==='boolean-map'){
        if(value===""&&field._initial===undefined)continue;
        if(!value||typeof value!=='object'||Array.isArray(value)||Object.entries(value).some(([key,state])=>!configurationPoints.includes(key)||typeof state!=='boolean')||configurationPoints.filter(key=>!key.startsWith('media.')).some(key=>!Object.hasOwn(value,key)))throw Error(field.label+'：请选择有效的决策场景');
        value=clone(value);
      } else if(field.type==='action-types'){
        if(!Array.isArray(value)||value.some(item=>typeof item!=='string'))throw Error(field.label+'：请选择有效的行动类型');
        value=[...value];
      } else if (field.type === "select")
        value = field._choices.find((c) => c.label === value)?.value ?? value;
      else if (field.type === "number") {
        if (value === "" && field._initial === undefined) continue;
        value = Number(value);
        if (
          !Number.isFinite(value) ||
          (field._definition?.step!=='any'&&!Number.isSafeInteger(value)) ||
          (field.min != null && value < field.min) ||
          (field.max != null && value > field.max)
        )
          throw Error(field.label + "：数值超出范围");
      }
      if (
        equal(value, field._initial) ||
        (field._initial === undefined && field.value === false)
      )
        continue;
      values[field._settingKey] ??= clone(
        catalog.effective_values[field._settingKey] || {},
      );
      if (value === undefined) delete values[field._settingKey][field._key];
      else values[field._settingKey][field._key] = value;
      }catch(cause){cause.settingField={settingKey:field._settingKey,key:field._key};throw cause;}
    }
  if(values['media.comfyui'])validateComfySetting(values['media.comfyui']);
  const kev=values.kev&&{...catalog.effective_values.kev,...values.kev};
  if(kev&&kev.budget_ms<kev.timeout_ms)throw Error('辅助决策阶段总等待时间不能短于单次请求超时');
  const original =
    quiet._policy?.quiet_hours ||
    catalog.effective_values["product.autonomy"]?.quiet_hours ||
    {};
  const normalizedQuiet = (q) => ({
    enabled: !!q.start,
    start: q.start || "",
    end: q.end || "",
    days: q.days || [0, 1, 2, 3, 4, 5, 6],
  });
  const nextQuiet = {
    enabled: quiet.enabled,
    start: quiet.start,
    end: quiet.end,
    days: quiet.days.map((d) => weekdays.indexOf(d)),
  };
  if (!equal(normalizedQuiet(original), nextQuiet)) {
    if (
      quiet.enabled &&
      (!/^([01]\d|2[0-3]):[0-5]\d$/.test(quiet.start) ||
        !/^([01]\d|2[0-3]):[0-5]\d$/.test(quiet.end) ||
        quiet.start === quiet.end ||
        !nextQuiet.days.length ||
        nextQuiet.days.some((d) => d < 0))
    )
      throw Error("请选择安静日期及不同的开始、结束时间");
    if (quiet.behavior !== "暂停主动联系")
      throw Error("当前策略暂停新的主动行动，已受理任务继续按原期限处理");
    const hours = quiet.enabled
      ? {
          start: quiet.start,
          end: quiet.end,
          timezone: original.timezone || "Asia/Shanghai",
          days: nextQuiet.days,
        }
      : {};
    if (quiet.character === "全部角色") {
      values["product.autonomy"] ??= clone(
        catalog.effective_values["product.autonomy"] || {},
      );
      values["product.autonomy"].quiet_hours = hours;
    } else {
      const targets = characters.filter(
        (c) => c.id === quiet.character || c.name === quiet.character,
      );
      if (
        targets.length !== 1 ||
        !quiet._policy ||
        quiet._policy.fluctlight_id !== targets[0].id
      )
        throw Error("请先读取并核对所选角色的安静策略");
      writes.push({
        kind: "autonomy:" + targets[0].id,
        operation: "autonomy",
        fid: targets[0].id,
        body: {
          policySource: "override",
          quietHours: hours,
          expectedRevision: quiet._policy.revision,
          reason: "设置该角色的安静时段",
        },
      });
    }
  }
  if (Object.keys(values).length)
    writes.push({ kind: "settings", operation: "settings", body: { values } });
  return writes;
}
