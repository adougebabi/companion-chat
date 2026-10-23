import { providerBindingBody } from "../v3/provider-budget.mjs";
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
    if (!binding.endpoint && !binding.model && !binding._source) continue;
    const endpointId = endpointIds.get(binding.endpoint);
    if (!endpointId) throw Error("请为用途选择有效的服务地址");
    const source = binding._source,
      normalized = {
        role: binding.role,
        endpointId,
        modelId: binding.model.trim(),
        tokenBudget: Number(binding.tokens),
        timeoutSeconds: Number(binding.timeout),
        contextWindowTokens: Number(binding.window),
        safetyMarginTokens: binding.safetyMargin,
      };
    if (
      !normalized.modelId ||
      !Number.isSafeInteger(normalized.tokenBudget) ||
      normalized.tokenBudget < 1 ||
      !Number.isSafeInteger(normalized.timeoutSeconds) ||
      normalized.timeoutSeconds < 1
    )
      throw Error(binding.name + "：请核对模型、输出预算和超时");
    const body = providerBindingBody(normalized, catalog.versions);
    delete body.expectedVersions;
    if (
      source &&
      !changedEndpoints.has(endpointId) &&
      source.endpoint_id === endpointId &&
      source.model_id === normalized.modelId &&
      source.token_budget === normalized.tokenBudget &&
      source.timeout_seconds === normalized.timeoutSeconds &&
      source.context_window_tokens === normalized.contextWindowTokens
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
      } else if (field.type === "select")
        value = field._choices.find((c) => c.label === value)?.value ?? value;
      else if (field.type === "number") {
        if (value === "" && field._initial === undefined) continue;
        value = Number(value);
        if (
          !Number.isFinite(value) ||
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
    }
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
