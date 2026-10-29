import {parseCreationDraft,creationRecordError} from '../v3/creation-draft.mjs';
import {persistentCommand} from '../v3/providers/persistent-command.mjs';
const messages={
 creation_draft_json:['创建资料暂时无法解析','原记录保留，当前无法自动修复；修复后可重读。'],
 creation_draft_version:['创建资料的格式暂不兼容','当前格式不支持自动迁移，原记录保留。'],
 creation_draft_invalid:['创建资料需要人工核对','原记录保留，当前不能自动修复或安全继续。'],
 creation_command_invalid:['原创建请求暂时无法读取','原请求身份和内容保留，暂时不能安全继续。'],
 creation_command_scope:['创建资料与原请求未能配对','资料与原请求未配对，核对前不会另起创建。'],
 creation_storage_unavailable:['暂时无法读取本机创建记录','浏览器未返回记录；恢复存储访问后可重读。'],
};
export function creationRecoveryProblem(cause,records=[]){
 const code=messages[cause?.code]?cause.code:cause instanceof SyntaxError?'creation_draft_json':'creation_storage_unavailable',[title,description]=messages[code];
 return {state:code,kind:code==='creation_storage_unavailable'?'error':'warning',title,description,blocked:true,records,diagnostic:{code,name:cause?.name||'Error',message:cause?.message||'创建记录读取未完成'}};
}
export function inspectCreationRecovery({owner,storage,resume=''}){
 const prefix=`fluctlight-creation:${owner}`,draftKey=resume?`${prefix}:history:${resume}`:`${prefix}:draft`,commandKey=resume?draftKey+':command':`${prefix}:command`,records=[];
 try{
  const raw=storage.getItem(draftKey),commandRaw=storage.getItem(commandKey);
  if(raw!==null)records.push({key:draftKey,value:raw});if(commandRaw!==null)records.push({key:commandKey,value:commandRaw});
  if(raw==null&&commandRaw==null)return {state:'empty',kind:'information',blocked:false,records};
  if(raw==null)throw creationRecordError('creation_command_scope','原创建请求存在，但对应创建资料尚未读到。');
  const draft=parseCreationDraft(raw);
  // Storage checkpoints can hold partial results; the entry additionally checks
  // the exact nested fields its controller reads synchronously before mounting.
  if((draft.analysis!=null&&(!draft.analysis.core_persona||typeof draft.analysis.core_persona!=='object'||Array.isArray(draft.analysis.core_persona)||!draft.analysis.core_persona.identity||typeof draft.analysis.core_persona.identity!=='object'||Array.isArray(draft.analysis.core_persona.identity)))||
    (draft.previewDraft!==undefined&&(!Array.isArray(draft.previewDraft)||draft.previewDraft.some(f=>!f||!Array.isArray(f.path)))))throw creationRecordError('creation_draft_invalid','创建预览所需的完整资料尚未读到。');
  if(resume&&draft.activatedId!==resume)throw creationRecordError('creation_command_scope','历史创建记录与所选角色不匹配。');
  let command=null;
  if(commandRaw!==null&&commandRaw!==undefined){
   if(!commandRaw)throw creationRecordError('creation_command_invalid','原创建请求记录为空，当前不能安全继续。');
   try{command=persistentCommand({key:commandKey,storage:()=>storage,execute(){throw Error('这里只读取原提交');}}).pending();}
   catch(cause){throw creationRecordError('creation_command_invalid','原创建请求记录不可读取。',cause);}
   if(!command?.value?.body||command.value.body.requestId!==draft.requestId)throw creationRecordError('creation_command_scope','原创建请求与当前草稿的身份不匹配。');
  }
  const href=resume?'#/library/create?resume='+encodeURIComponent(resume):'#/library/create';
  if(command&&!draft.activatedId)return {state:command.rejection?'activation-rejected':'activation-unknown',kind:command.rejection?'warning':'action-required',title:command.rejection?'上次创建未被接受':'上次创建的结果还待核对',description:command.rejection?'原输入保留，请先核对最新资料。':'将核对同一份原提交，避免重复创建角色。',actionLabel:command.rejection?'核对原创建资料':'核对创建结果',href,draft,command,blocked:false,records};
  if(draft.activatedId)return {state:'preparation',kind:'progress',title:'角色已建立，准备状态可继续查看',description:'查看实际准备阶段；未完成的步骤可在原任务中继续处理。',actionLabel:'查看准备进度',href,draft,command,blocked:false,records};
  return {state:'draft',kind:'action-required',title:'你有一份未完成的角色资料',description:'上次编辑的内容已经保留。',actionLabel:'继续创建',href,draft,command,blocked:false,records};
 }catch(cause){return creationRecoveryProblem(cause,records);}
}
export function requireCreationRecovery({owner,storage,resume=''}){
 const state=inspectCreationRecovery({owner,storage,resume});
 if(state.blocked)throw Object.assign(creationRecordError(state.state,state.title),{recoveryState:state});
 return state;
}
export function downloadCreationRecords(records){
 if(!records.length)return;
 const url=URL.createObjectURL(new Blob([JSON.stringify({kind:'creation-recovery-records',records},null,2)],{type:'application/json'}));
 const link=document.createElement('a');link.href=url;link.download='创建记录留存.json';link.click();setTimeout(()=>URL.revokeObjectURL(url),0);
}
