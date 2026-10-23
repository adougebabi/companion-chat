// Legacy budget columns are inert compatibility data. Model binding only
// chooses a service/model and a timeout; the provider owns token capacity.
export function bindingValues(role, binding) {
 return {role, endpointId: binding?.endpoint_id || '', modelId: binding?.model_id || '', timeoutSeconds: binding?.timeout_seconds ?? 120};
}
export function providerBindingBody(value, versions) {
 return {role:value.role, endpointId:value.endpointId, modelId:value.modelId, timeoutSeconds:value.timeoutSeconds, expectedVersions:{...versions,['binding:'+value.role]:versions['binding:'+value.role]??0,['endpoint:'+value.endpointId]:versions['endpoint:'+value.endpointId]??0}};
}
