import {persistentCommand} from './persistent-command.mjs';

export async function describeMediaFile(file){
 if(!file||!Number.isInteger(file.size)||file.size<=0||file.size>64*1024*1024)throw new Error('请选择非空且不超过64MiB的媒体文件');
 const bytes=await file.arrayBuffer(),digest=await crypto.subtle.digest('SHA-256',bytes);
 return {filename:file.name,mime:file.type||'application/octet-stream',size:file.size,sha256:Array.from(new Uint8Array(digest),v=>v.toString(16).padStart(2,'0')).join('')};
}
// Files stay in memory; only the original identity and content fingerprint
// survive refresh. A lookup can confirm success without resending bytes.
export function mediaUploadController({slot,storage,client,isOwner}){
 let selected=null;
 const command=persistentCommand({key:slot+':command',storage,retainedValidationCodes:['media_upload_key_reused'],preAdmissionConflicts:[],execute:async(description,key)=>{
  if(!isOwner())throw new Error('会话已切换，请返回原账号核对上传');
  let asset;
  try{const record=await client.lookup(key);if(record.status==='ready'){
   asset=record.asset;
   if(!asset)throw new Error('原上传已记录但资产读回缺失，请核对原提交；不会重复上传');
  }}catch(error){if(error.status!==404)throw error;}
  if(!asset){
   if(!isOwner()||command.pending()?.idempotencyKey!==key)throw new Error('会话或原提交已切换，未继续上传');
   if(!selected)throw new Error('原提交尚未完成，请重新选择同一文件，再以原提交身份重试');
   const actual=await describeMediaFile(selected);
   if(JSON.stringify(actual)!==JSON.stringify(description))throw new Error('所选文件与原提交的名称、类型或内容不同，请选择原文件');
   if(!isOwner()||command.pending()?.idempotencyKey!==key)throw new Error('会话或原提交已切换，未继续上传');
   const receipt=await client.upload(selected,description,key);
   if(receipt.sha256!==description.sha256||receipt.byte_size!==description.size)throw new Error('上传回执与原文件不一致，请核对原提交');
   if(!isOwner()||command.pending()?.idempotencyKey!==key)throw new Error('会话已切换，请回原账号查询上传结果');
   asset=await client.asset(receipt.asset_id);
  }
  if(asset.origin!=='owner_upload'||asset.sha256!==description.sha256||asset.byte_size!==description.size||asset.source_filename!==description.filename)throw new Error('原上传来源或字节身份不一致，请核对原提交');
  const pending=command.pending();
  if(isOwner()&&pending?.idempotencyKey===key)storage().setItem(slot+':result',JSON.stringify({assetId:asset.id,idempotencyKey:key}));
  return asset;
 }});
 return {pending:command.pending,lastAsset(){const raw=storage().getItem(slot+':result');if(!raw)return null;const value=JSON.parse(raw);if(typeof value.assetId!=='string')throw new Error('上传结果标识损坏，请从媒体库查询');return value.assetId;},select(file){selected=file;},async submit(){const description=await describeMediaFile(selected);if(!isOwner())throw new Error('会话已切换，未提交上传');return command.submit(description);},retry:command.retry};
}
