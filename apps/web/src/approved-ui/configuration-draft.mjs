// Persist connection inputs and their CAS baseline, never credentials or DTOs.
import {defaultModelTimeoutSeconds} from '../v3/provider-budget.mjs';
export const connectionDraftKey = owner => `fluctlight-studio:${owner}:configuration:connections:draft`;
export function connectionIsSaved(endpoint){
 const source=endpoint?._source;
 return !!source&&endpoint.url.replace(/\/+$/,'')===String(source.base_url||'').replace(/\/+$/,'')&&endpoint.protocol===source.kind&&!endpoint.secret&&!endpoint.clearSecret;
}
function safeURL(value) {
  if (!value) return '';
  try { const url=new URL(value); return ['http:','https:'].includes(url.protocol)&&!url.username&&!url.password&&!url.search&&!url.hash ? value : ''; }
  catch { return ''; }
}
export function connectionDraft({endpoints,bindings,versions}) {
  return {version:1,versions:{...versions},credentialPending:endpoints.some(e=>!!e.secret||!!e.clearSecret),
    endpoints:endpoints.map(e=>({id:e.id,name:e.name,protocol:e.protocol,url:safeURL(e.url)})),
    bindings:bindings.map(b=>({role:b.role,endpoint:b.endpoint,model:b.model,timeout:b.timeout}))};
}
export function connectionDraftDirty(draft,catalog){
 if(draft.credentialPending)return true;
 const endpoints=rows=>rows.map(e=>[e.id,e.name,e.protocol,e.url.replace(/\/+$/,'')]).sort((a,b)=>a[0].localeCompare(b[0]));
 const bindings=rows=>rows.filter(b=>b.endpoint||b.model||b.timeout!==''&&b.timeout!=null).map(b=>[b.role,b.endpoint,b.model.trim(),b.timeout===''||b.timeout==null?defaultModelTimeoutSeconds:Number(b.timeout)]).sort((a,b)=>a[0].localeCompare(b[0]));
 return JSON.stringify(endpoints(draft.endpoints))!==JSON.stringify(endpoints(catalog.endpoints.map(e=>({id:e.id,name:e.id,protocol:e.kind,url:e.base_url}))))||JSON.stringify(bindings(draft.bindings))!==JSON.stringify(bindings(catalog.bindings.map(b=>({role:b.role,endpoint:b.endpoint_id,model:b.model_id,timeout:b.timeout_seconds}))));
}
export function restoreConnectionDraft(raw,catalog) {
  let draft;try{draft=JSON.parse(raw);}catch{return null;}
  if(draft?.version!==1||!Array.isArray(draft.endpoints)||!Array.isArray(draft.bindings)||!draft.versions||typeof draft.versions!=='object')return null;
  if(!draft.endpoints.every(e=>e&&['id','name','protocol','url'].every(k=>typeof e[k]==='string'))||
     !draft.bindings.every(b=>b&&['role','endpoint','model'].every(k=>typeof b[k]==='string')&&['string','number'].includes(typeof b.timeout)))return null;
  const clean=connectionDraft(draft);
  clean.credentialPending=!!draft.credentialPending;
  const dirty=connectionDraftDirty(clean,catalog),connectionVersion=key=>['endpoint:','binding:','secret:'].some(prefix=>key.startsWith(prefix)),originalVersions=Object.fromEntries(Object.entries(clean.versions).filter(([key])=>connectionVersion(key)));
  return {...clean,dirty,versions:{...catalog.versions,...originalVersions},endpoints:clean.endpoints.map(e=>({...e,secret:'',clearSecret:false,models:[],status:dirty?'草稿已恢复':'已保存',_source:catalog.endpoints.find(source=>source.id===e.id)})),
    stale:dirty&&Object.entries(originalVersions).some(([key,value])=>catalog.versions[key]!==value)};
}
