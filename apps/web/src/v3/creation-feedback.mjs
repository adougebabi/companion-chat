export function creationFeedback(error){
 const details=error?.details||{},validation=details.validation_error||{};
 const code=error?.code||'initialization_unavailable';
 const messages={initialization_response_semantic_empty:'模型尚未返回可用的人物内容。原描述已保留，可以重新分析，或选择空白开始后逐步补充。',initialization_persona_invalid:'部分初始化内容需要补充。原描述和草稿已保留，请核对下面指出的字段。',initialization_provider_unavailable:'初始化模型暂时不可用。可以先保存草稿，在服务连接中确认认知模型后继续。',initialization_input_budget_exceeded:'当前初始化输入超过模型实际可用窗口。请核对认知模型的窗口与输出设置，原描述已保留。',browser_request_timeout:'等待分析结果超时。先核对本次关联记录；已保存的分析来源可恢复使用，草稿保持。'};
 return {message:messages[code]||error?.message||String(error),code,correlationId:details.correlation_id||error?.correlationId||null,path:validation.path||null,type:validation.type||null,retryable:details.retryable??null};
}

export function initializationWarnings(value){
 if(!Array.isArray(value))return value==null?[]:[{path:'extensions.initialization_warnings',message:'补充说明的格式不完整，原值保留在完整预览中，可继续审阅。'}];
 return value.map((warning,index)=>warning&&typeof warning==='object'&&typeof warning.message==='string'?{path:typeof warning.path==='string'?warning.path:'initialization',message:warning.message}:{path:`extensions.initialization_warnings[${index}]`,message:'此条补充说明的格式不完整，原值保留在完整预览中，可继续审阅。'});
}
