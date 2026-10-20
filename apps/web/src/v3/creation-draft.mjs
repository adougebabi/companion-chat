import { randomId } from '../random-id.mjs';
// Only UI drafts and original command identities live in session storage.
// The server remains the authority for analysis freshness and activation replay.
export function creationDraft({owner,storage,timezone=Intl.DateTimeFormat().resolvedOptions().timeZone}) {
  if(!owner)throw new Error('请先登录所有者会话');
  const key=`fluctlight-creation:${owner}:draft`;
  function read(){
    const raw=storage.getItem(key);
    if(!raw)return {version:1,step:0,mode:'description',name:'',bio:'',traits:'',world:'',json:'',analysisText:'',groupId:'',templateId:'',templateVersion:'',requestId:randomId(),timezone,analysis:null,includeGoals:true,activatedId:null};
    const value=JSON.parse(raw);
    if(value?.version!==1||!Number.isInteger(value.step)||value.step<0||value.step>3||typeof value.requestId!=='string'||!value.requestId)throw new Error('创建草稿不可读取，请先核对原创建请求，避免重复激活');
    return value;
  }
  return {key,commandKey:`fluctlight-creation:${owner}:command`,read,save(value){
    const previous=storage.getItem(key)?read():null;
    if(previous?.activatedId){
      if(previous.requestId!==value.requestId)throw new Error('原角色已经激活，请先完成该角色的分组核对');
      if(!value.activatedId)return; // An older mounted page cannot erase a receipt.
      if(previous.activatedId!==value.activatedId)throw new Error('激活结果身份不一致，请保留原记录核对');
    }
    storage.setItem(key,JSON.stringify(value));
  },clear(){storage.removeItem(key);}};
}

// Persist the accepted result before persistentCommand releases its pending
// identity. Page disposal is allowed; an Owner switch or newer draft is not.
export function checkpointCreation(recovery,submitted,result,ownerCurrent){
  if(typeof result.id!=='string'||!result.id)throw new Error('激活响应缺少角色标识；请保留原请求核对');
  if(!ownerCurrent())return;
  const saved=recovery.read();
  if(saved.requestId!==submitted.body.requestId)return;
  recovery.save({...saved,activatedId:result.id,activationGroupId:submitted.groupId,...(submitted.worldResourceId?{worldResourceId:submitted.worldResourceId,worldResourceVersion:submitted.worldResourceVersion}:{})});
}

// A different published source changes analysis context, never a pending command.
export function selectCreationWorld(draft,{resourceId,version,pending=false}){
 if(!resourceId||!version||pending)return false;
 if(draft.activatedId){
  // An explicitly chosen late source can complete an unconfigured world step.
  // An already bound source/runtime remains governed by world management.
  if(draft.worldResourceId||draft.worldRuntimeId)return false;
  draft.worldResourceId=resourceId;draft.worldResourceVersion=version;return true;
 }
 if(draft.worldResourceId===resourceId&&draft.worldResourceVersion===version)return false;
 draft.worldResourceId=resourceId;draft.worldResourceVersion=version;
 draft.analysis=null;draft.analysisText='';draft.worldContextWarning='';
 if(draft.step===3)draft.step=draft.mode==='analysis'?0:2;
 draft.requestId=randomId();return true;
}
