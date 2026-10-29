import { creationDraft,parseCreationDraft,creationRecordError } from '../v3/creation-draft.mjs';
import {requireCreationRecovery} from './creation-recovery-state.mjs';
export const creationHistoryKey=(owner,fid)=>`fluctlight-creation:${owner}:history:${fid}`;
export function migrateCreationRecovery(owner,storage,previous,{resume=''}={}){
  if(resume&&storage.getItem(creationHistoryKey(owner,resume)))return creationRecovery({owner,storage,query:new URLSearchParams({resume})});
  const recovery=creationDraft({owner,storage});
  const sourceDraft=previous?.getItem(recovery.key),sourceCommand=previous?.getItem(recovery.commandKey);
  if(sourceDraft==null){
    if(sourceCommand!=null)throw Object.assign(creationRecordError('creation_command_scope','旧会话的原创建请求仍在，但对应草稿尚未找到。'),{records:[{key:recovery.commandKey,source:'previous-session',value:sourceCommand}]});
    return recovery;
  }
  const source=parseCreationDraft(sourceDraft),target=storage.getItem(recovery.key)?recovery.read():null;
  // A different page may already own the durable slot. Keep the legacy pair
  // intact rather than attaching its unknown command to the newer draft.
  if(target&&target.requestId!==source.requestId)return recovery;
  if(sourceCommand&&JSON.parse(sourceCommand)?.value?.body?.requestId!==source.requestId)return recovery;
  const targetCommand=storage.getItem(recovery.commandKey);
  if(sourceCommand&&targetCommand&&sourceCommand!==targetCommand)return recovery;
  for(const key of [recovery.key,recovery.commandKey]){
    if(!storage.getItem(key)&&previous?.getItem(key)){
      storage.setItem(key,previous.getItem(key));
    }
  }
  // Release the legacy source only after both writes succeeded.
  for(const key of [recovery.key,recovery.commandKey])if(previous?.getItem(key)===storage.getItem(key))previous?.removeItem(key);
  return recovery;
}
export function creationRecovery({owner,storage,query}){
  const base=creationDraft({owner,storage});
  const fid=query.get('resume'),key=fid&&creationHistoryKey(owner,fid);
  if(key&&storage.getItem(key)){
    const archived=JSON.parse(storage.getItem(key));
    if(archived.activatedId!==fid)throw Error('原创建进度的角色范围不匹配');
    requireCreationRecovery({owner,storage,resume:fid});
    const scoped={getItem:k=>storage.getItem(k===base.key?key:k===base.commandKey?key+':command':k),setItem:(k,v)=>storage.setItem(k===base.key?key:k===base.commandKey?key+':command':k,v),removeItem:k=>storage.removeItem(k===base.key?key:k===base.commandKey?key+':command':k)};
    return {...creationDraft({owner,storage:scoped}),commandKey:key+':command',archived:true};
  }
  if(fid)throw creationRecordError('creation_command_scope','原角色的创建进度尚未找到，当前草稿不会替代它。');
  requireCreationRecovery({owner,storage});
  const saved=base.read();
  if(query.get('new')==='1'&&saved.activatedId&&!storage.getItem(base.commandKey)){
    storage.setItem(creationHistoryKey(owner,saved.activatedId),JSON.stringify(saved));
    base.clear();
  }
  return base;
}
