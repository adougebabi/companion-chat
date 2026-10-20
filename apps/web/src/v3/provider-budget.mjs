export function bindingValues(role,binding,defaults){
 return {role,endpointId:binding?.endpoint_id||'',modelId:binding?.model_id||'',tokenBudget:binding?.token_budget??defaults?.output_tokens??'',timeoutSeconds:binding?.timeout_seconds??120,contextWindowTokens:binding?.context_window_tokens??'',maxInputTokens:binding?.max_input_tokens??'',safetyMarginTokens:binding?binding.safety_margin_tokens:defaults?.safety_margin_tokens,promptBudgetPolicyVersion:binding?binding.prompt_budget_policy_version:defaults?.policy_version};
}
export function deriveProviderBudget(fields,current){
 const window=Number(fields.contextWindowTokens),output=Number(fields.tokenBudget),margin=current.safetyMarginTokens;
 const available=window-output-margin;
 return {maxInputTokens:Number.isSafeInteger(window)&&window>0&&Number.isSafeInteger(output)&&output>0&&Number.isSafeInteger(margin)&&margin>=0&&available>0?available:''};
}
export function providerBindingBody(value,versions){
 const derived=deriveProviderBudget(value,value);
 if(!derived.maxInputTokens)throw new Error('上下文窗口必须大于输出上限与系统安全预留之和，请核对模型实际加载的窗口。');
 // Core owns the calculation. Never submit the derived display as a second authority.
 return {role:value.role,endpointId:value.endpointId,modelId:value.modelId,tokenBudget:value.tokenBudget,timeoutSeconds:value.timeoutSeconds,contextWindowTokens:value.contextWindowTokens,expectedVersions:{...versions,['binding:'+value.role]:versions['binding:'+value.role]??0,['endpoint:'+value.endpointId]:versions['endpoint:'+value.endpointId]??0}};
}
