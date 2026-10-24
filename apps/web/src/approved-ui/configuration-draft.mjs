// Persist connection inputs and their CAS baseline, never credentials or DTOs.
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
export function restoreConnectionDraft(raw,catalog) {
  let draft;try{draft=JSON.parse(raw);}catch{return null;}
  if(draft?.version!==1||!Array.isArray(draft.endpoints)||!Array.isArray(draft.bindings)||!draft.versions||typeof draft.versions!=='object')return null;
  if(!draft.endpoints.every(e=>e&&['id','name','protocol','url'].every(k=>typeof e[k]==='string'))||
     !draft.bindings.every(b=>b&&['role','endpoint','model'].every(k=>typeof b[k]==='string')&&['string','number'].includes(typeof b.timeout)))return null;
  const clean=connectionDraft(draft);
  return {...clean,credentialPending:!!draft.credentialPending,endpoints:clean.endpoints.map(e=>({...e,secret:'',clearSecret:false,models:[],status:'草稿已恢复',_source:catalog.endpoints.find(source=>source.id===e.id)})),
    stale:Object.entries(clean.versions).some(([key,value])=>catalog.versions[key]!==value)};
}
