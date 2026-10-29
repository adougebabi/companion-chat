const clone = (value) => JSON.parse(JSON.stringify(value));
/** Advance only this command's acknowledged versions; retain input typed in flight. */
export function acknowledgeConfiguration(
  { catalog, versions, settings, endpoints, bindings, quiet },
  operation,
  receipt,
) {
  const body = operation.body;
  Object.assign(versions, receipt.applied_versions || {});
  catalog.versions = { ...versions };
  if (operation.operation === "endpoint") {
    const source = {
      id: body.endpointId,
      kind: body.kind,
      base_url: body.baseUrl.replace(/\/+$/, ""),
      secret_purpose: body.secretPurpose,
    };
    const original = endpoints.find(
      (e) =>
        e.id === operation.formId ||
        e._source?.id === source.id ||
        e.name === source.id,
    );
    if (original) {
      const previousId = original.id;
      original._source = source;
      original.id = source.id;
      for (const binding of bindings)
        if (binding.endpoint === previousId) binding.endpoint = source.id;
    }
    catalog.endpoints = catalog.endpoints
      .filter((e) => e.id !== source.id)
      .concat(source);
    catalog.bindings = catalog.bindings.filter(
      (b) => b.endpoint_id !== source.id,
    );
    for (const binding of bindings)
      if (binding._source?.endpoint_id === source.id)
        binding._source = undefined;
  }
  if (operation.operation === "binding") {
    const source = {
      role: body.role,
      endpoint_id: body.endpointId,
      model_id: body.modelId,
      timeout_seconds: body.timeoutSeconds,
    };
    const field = bindings.find((b) => b.role === source.role);
    if (field) {
      field._source = source;
    }
    catalog.bindings = catalog.bindings
      .filter((b) => b.role !== source.role)
      .concat(source);
  }
  for (const [key, value] of Object.entries(body.values || {})) {
    catalog.effective_values[key] = clone(value);
    if(catalog.value_sources)catalog.value_sources[key]='stored';
    for(const group of settings)if(group.valueSources&&group.fields.some(field=>field._settingKey===key))group.valueSources[key]='stored';
    for (const group of settings)
      for (const field of group.fields)
        if (field._settingKey === key) field._initial = value[field._key];
    if (key === "product.autonomy" && quiet.character === "全部角色")
      quiet._source = clone(value.quiet_hours || {});
  }
  if (
    operation.operation === "autonomy" &&
    quiet._policy?.fluctlight_id === operation.fid
  ) {
    quiet._policy = {
      ...quiet._policy,
      revision: receipt.revision,
      quiet_hours: clone(body.quietHours),
    };
    quiet._source = clone(body.quietHours);
  }
}

/** A secret receipt can clear only its own purpose's unchanged input. */
export function clearAcknowledgedCredentials(endpoints, submitted, body) {
  const purposes = new Set([
    ...Object.keys(body.secrets || {}),
    ...(body.clearSecrets || []),
  ]);
  const purpose = (e) =>
    e._source?.secret_purpose || "provider:" + (e._source?.id || e.name);
  for (const endpoint of endpoints) {
    const key = purpose(endpoint);
    if (!purposes.has(key)) continue;
    const original = submitted.find((e) => purpose(e) === key);
    if (!original) continue;
    if (endpoint.secret === original.secret) endpoint.secret = "";
    if (endpoint.clearSecret === original.clearSecret)
      endpoint.clearSecret = false;
  }
}
