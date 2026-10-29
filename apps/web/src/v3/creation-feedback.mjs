import {errorMessage} from './domain-presentation.mjs';
export function creationFeedback(error){
 const details=error?.details||{},validation=details.validation_error||{};
 const code=error?.code||'initialization_unavailable';
 const messages={initialization_response_truncated:"模型在生成完整正文前结束，本次没有得到可用的人物资料。原描述已保留，请重试整理；可以在本次运行中查看结束原因。",initialization_role_unconfigured:"人物初始化的通用模型尚未就绪。请核对模型用途和服务连接，原输入已保留。",initialization_response_invalid_json:"模型返回的人物资料格式不完整。原描述已保留，可以重试整理。",initialization_provider_timeout:"整理人物资料等待超时。原输入已保留，可以检查模型连接后重试。",initialization_provider_cancelled:"本次整理已中断，原输入仍保留，可重新核对进度。",activation_analysis_stale:"已有更新的人物分析，请保留输入并重新整理后审阅。",initialization_response_semantic_empty:'模型尚未返回可用的人物内容。原描述已保留，可以重新分析，或选择空白开始后逐步补充。',initialization_persona_invalid:'模型返回的人物资料未通过字段校验。原描述和草稿已保留，可以重新整理并审阅结果。',initialization_provider_unavailable:'初始化模型暂时不可用。可以先保存草稿，在服务连接中确认认知模型后继续。',initialization_input_budget_exceeded:'当前初始化输入超过模型实际可用窗口。请核对认知模型的窗口与输出设置，原描述已保留。',browser_request_timeout:'等待分析结果超时。先核对本次关联记录；已保存的分析来源可恢复使用，草稿保持。'};
 messages.initialization_response_truncated='模型以长度终止状态结束了本次整理，完整的人物资料尚未确认。原输入已保留，可以重试本次整理。';
 let message=messages[code]||errorMessage(error,'人物资料暂未整理完成，原输入已保留。');
 const inputValidation=code==='initialization_input_invalid'||code==='invalid_request'&&validation.type==='input';
 if(inputValidation&&validation.path)message+=` 问题字段：${validation.path}。请修正原输入后重新整理，或切换创建方式。`;
 return {message,code,correlationId:details.correlation_id||error?.correlationId||null,path:validation.path||null,type:code==='initialization_role_unconfigured'?'configuration':validation.type||null,retryable:details.retryable??null};
}

export function initializationWarnings(value){
 if(!Array.isArray(value))return value==null?[]:[{path:'extensions.initialization_warnings',message:'补充说明的格式不完整，原值保留在完整预览中，可继续审阅。'}];
 return value.map((warning,index)=>warning&&typeof warning==='object'&&typeof warning.message==='string'?{path:typeof warning.path==='string'?warning.path:'initialization',message:warning.message}:{path:`extensions.initialization_warnings[${index}]`,message:'此条补充说明的格式不完整，原值保留在完整预览中，可继续审阅。'});
}
