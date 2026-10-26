import {studioDraftStorage} from './studio-draft-storage.js';

export const studioWorkingDraftKey=(actor,id)=>`fluctlight-studio:${actor}:${id}:draft`;
const object=value=>value!==null&&typeof value==='object'&&!Array.isArray(value);
// Initialize missing UI fields only. An existing CAS baseline is never rebased by a read.
export function normalizeStudioDraft(saved,resource,{kind='event_template',body={},name=''}={}){
 if(saved!==null&&(!object(saved)||(saved.body!==undefined&&!object(saved.body))))throw new Error('工作室草稿格式损坏，请核对并恢复原草稿');
 const defaults={kind:resource?.kind||kind,name:resource?.name||name,body:structuredClone(resource?.body||body),expectedVersion:resource?.draft_version||resource?.published_version||undefined,reason:'',target:''};
 const draft={...defaults,...(saved||{})};
 if(typeof draft.kind!=='string'||typeof draft.name!=='string'||typeof draft.reason!=='string'||typeof draft.target!=='string'||(draft.expectedVersion!==undefined&&typeof draft.expectedVersion!=='string'))throw new Error('工作室草稿字段格式损坏，请核对原草稿');
 return draft;
}
export function readStudioDraft(actor,id,resource,options,storageProvider=studioDraftStorage){
 const raw=storageProvider().getItem(studioWorkingDraftKey(actor,id));let saved=null;
 if(raw!==null){try{saved=JSON.parse(raw);}catch{throw new Error('工作室草稿不可读取，请核对原草稿');}if(saved===null)throw new Error('工作室草稿格式损坏，请核对原草稿');}
 return normalizeStudioDraft(saved,resource,options);
}
export function writeStudioDraft(actor,id,draft,storageProvider=studioDraftStorage){storageProvider().setItem(studioWorkingDraftKey(actor,id),JSON.stringify(draft));}
