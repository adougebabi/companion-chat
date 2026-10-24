// An exported analysis is a preview, never an authority to bypass Core checks.
// Core rechecks its persisted source, Owner, freshness and activation replay.
function invalidAnalysis(path, kind = '对象') {
  return Object.assign(new Error(`整理结果缺少有效${kind}：${path}。原输入已保留，请返回修改或重新整理。`), {
    code: 'initialization_persona_invalid',
    details: { validation_error: { type: 'structure', path } },
  });
}
export function parseCreationAnalysis(text) {
  let value;
  try { value = JSON.parse(text); } catch { throw invalidAnalysis('provider_response'); }
  const object = candidate => candidate !== null && typeof candidate === 'object' && !Array.isArray(candidate);
  if (!object(value) || !/^initialization_source_[a-f0-9]{32}$/.test(value.analysis_id) || typeof value.correlation_id !== 'string' || !value.correlation_id || !Number.isSafeInteger(value.schema_version) || value.schema_version < 1) throw new Error('请导入当前所有者已完成分析的完整 JSON（含分析来源与版本）');
  for (const field of ['core_persona','developing_self','extensions']) if (!object(value[field])) throw invalidAnalysis(field);
  if (!object(value.core_persona.identity)) throw invalidAnalysis('core_persona.identity');
  for (const field of ['initial_goals','initial_intentions','initial_relationships']) if (!Array.isArray(value[field]) || !value[field].every(object)) throw invalidAnalysis(field, '列表');
  return value;
}

export function parseCreationInput(text) {
  if (new TextEncoder().encode(text).length > 60000) throw new Error('创建输入超过 60000 UTF-8 字节上限，请保留原文件并缩小本次输入');
  let value;
  try {value=JSON.parse(text);} catch(cause){
    const position=/position (\d+)/.exec(cause.message)?.[1];
    throw new Error('JSON 格式不正确'+(position?'，请检查第 '+(Number(position)+1)+' 个字符附近的引号、逗号或括号。':'，请检查引号、逗号和括号。')+'原内容已保留，可直接修正。');
  }
  if(!value||typeof value!=='object'||Array.isArray(value)||!Object.keys(value).length)throw new Error('创建 JSON 必须是非空对象');
  for(const key of ['analysis_id','correlation_id','id','status','current_revision','current_state','messages','memories','inventory'])if(key in value)throw new Error(`原始创建 JSON 不能携带分析凭证或运行存档字段：${key}。恢复分析预览请使用对应创建方式。`);
  return value;
}

export function creationAnalysisRequest(draft) {
  if(draft.mode==='json')return {json:parseCreationInput(draft.json)};
  if(draft.mode==='template'){
    if(!draft.templateId||!draft.templateVersion)throw new Error('请选定已发布的角色模板版本');
    return {resourceId:draft.templateId,resourceVersion:draft.templateVersion};
  }
  if(draft.mode!=='description')throw new Error('当前创建方式无需请求分析');
  const description=`${draft.bio}\n姓名：${draft.name}\n性格期望：${draft.traits}\n世界与生活：${draft.world}`;
  if(!draft.bio.trim()||new TextEncoder().encode(description).length>60000)throw new Error('请填写完整描述，全部输入须在 60000 UTF-8 字节以内');
  return {description};
}
