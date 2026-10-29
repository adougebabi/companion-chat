import { randomId } from '../random-id.mjs';
export const creationRecoveryChanged='fluctlight:creation-recovery-changed';
export function notifyCreationRecovery(owner){
  if(typeof window==='undefined'||typeof window.dispatchEvent!=='function')return;
  window.dispatchEvent(new CustomEvent(creationRecoveryChanged,{detail:{owner}}));
}
export function creationRecordError(code,message,cause){return Object.assign(new Error(message,cause?{cause}:undefined),{code});}
export function parseCreationDraft(raw){
  let value;try{value=JSON.parse(raw);}catch(cause){throw creationRecordError('creation_draft_json','创建资料无法解析，原记录已保留。',cause);}
  if(value?.version!==1)throw creationRecordError('creation_draft_version','这份创建资料的格式版本暂不兼容，原记录已保留。');
  if(!value||Array.isArray(value)||!Number.isInteger(value.step)||value.step<0||value.step>3||typeof value.requestId!=='string'||!value.requestId||
    (value.activatedId!=null&&(typeof value.activatedId!=='string'||!value.activatedId.trim())))
    throw creationRecordError('creation_draft_invalid','创建草稿不可读取，暂时无法安全继续；原记录已保留。');
  return value;
}
// Only UI drafts and original command identities live in session storage.
// The server remains the authority for analysis freshness and activation replay.
export function creationDraft({owner,storage,timezone='Asia/Shanghai'}) {
  if(!owner)throw new Error('请先登录所有者会话');
  const key=`fluctlight-creation:${owner}:draft`;
  function read(){
    const raw=storage.getItem(key);
    if(raw===null||raw===undefined)return {version:1,step:0,useDefaultWorld:true,mode:'description',name:'',bio:'',traits:'',world:'',json:'',analysisText:'',groupId:'',templateId:'',templateVersion:'',requestId:randomId(),timezone,analysis:null,includeGoals:true,activatedId:null};
    return parseCreationDraft(raw);
  }
  return {key,commandKey:`fluctlight-creation:${owner}:command`,read,save(value){
    const previous=storage.getItem(key)?read():null;
    if(previous?.activatedId){
      if(previous.requestId!==value.requestId)throw new Error('原角色已经激活，请先完成该角色的分组核对');
      if(!value.activatedId)return; // An older mounted page cannot erase a receipt.
      if(previous.activatedId!==value.activatedId)throw new Error('激活结果身份不一致，请保留原记录核对');
    }
    const serialized=JSON.stringify(value),changed=storage.getItem(key)!==serialized;
    storage.setItem(key,serialized);if(changed)notifyCreationRecovery(owner);
  },clear(){storage.removeItem(key);notifyCreationRecovery(owner);}};
}

// Persist the accepted result before persistentCommand releases its pending
// identity. Page disposal is allowed; an Owner switch or newer draft is not.
export function checkpointCreation(recovery,submitted,result,ownerCurrent){
  if(typeof result.id!=='string'||!result.id)throw new Error('激活响应缺少角色标识；请保留原请求核对');
  if(!ownerCurrent())return;
  const saved=recovery.read();
  if(saved.requestId!==submitted.body.requestId)return;
  recovery.save({...saved,activatedId:result.id,activationSnapshot:structuredClone(submitted),activationGroupId:submitted.groupId,...(Object.hasOwn(submitted,'worldSource')?{worldSource:submitted.worldSource}:{}),...(Object.hasOwn(submitted,'useDefaultWorld')?{useDefaultWorld:submitted.useDefaultWorld}:{}),...(submitted.worldResourceId?{worldResourceId:submitted.worldResourceId,worldResourceVersion:submitted.worldResourceVersion}:{})});
}

// A different published source changes analysis context, never a pending command.
export function selectCreationWorld(draft,{resourceId,version,pending=false}){
 if(!resourceId||!version||pending)return false;
 if(draft.activatedId){
  // An explicitly chosen late source can complete an unconfigured world step.
  // An already bound source/runtime remains governed by world management.
  if(draft.useDefaultWorld||draft.worldResourceId||draft.worldRuntimeId)return false;
  draft.useDefaultWorld=false;draft.worldResourceId=resourceId;draft.worldResourceVersion=version;return true;
 }
 if(draft.worldResourceId===resourceId&&draft.worldResourceVersion===version)return false;
 draft.useDefaultWorld=false;draft.worldResourceId=resourceId;draft.worldResourceVersion=version;
 draft.analysis=null;draft.analysisText='';draft.worldContextWarning='';
 if(draft.step===3)draft.step=draft.mode==='analysis'?0:2;
 draft.requestId=randomId();return true;
}

export function checkpointCreationAnalysis(recovery,requestId,analysis,ownerCurrent){
 if(!ownerCurrent())return false;
 const saved=recovery.read();if(saved.requestId!==requestId||saved.activatedId)return false;
 recovery.save({...saved,analysis,analysisText:JSON.stringify(analysis,null,2)});return true;
}
