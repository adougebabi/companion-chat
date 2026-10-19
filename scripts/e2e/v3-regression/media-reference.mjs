import {readFile} from 'node:fs/promises';
import {basename,extname} from 'node:path';
import {check,equal,hash} from './runtime.mjs';
import {job,readImageEvidence,reason} from './domain.mjs';
const png=new Uint8Array(Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=','base64'));
async function grantedImage(t,name,resource,purpose,bytes,filename,mime){
 const receipt=(await t.upload(name,{filename,mime,bytes})).value;equal(receipt.sha256,hash(bytes),'Reference upload must preserve actual source content');
 const asset=await t.get('/api/media-assets/'+encodeURIComponent(receipt.asset_id)),grant=(await t.command(name+'-grant','POST','/api/media-reference-grants',()=>({assetId:asset.id,expectedAssetRevision:asset.asset_revision,targetScope:'studio_resource',targetId:resource.id,purpose,confirm:true,reason,idempotencyKey:t.token(name+'-grant')}))).value;
 equal(grant.provenance,'owner_explicit_unverified_content','Owner permission is not a visual inspection result');equal(grant.asset_id,asset.id,'Grant must freeze the exact source');equal(grant.sha256,asset.sha256,'Grant must preserve immutable byte identity');
 const referenceAsset={assetId:asset.id,purpose,grantId:grant.id,grantRevision:grant.revision};
 const output=await job(t,name+'-job',resource,'image_preview',purpose==='person'?'生成一张虚构成人插画，只把所选图片作为明确授权的人物画面参考，不借用任何角色身份或生活事实。':'根据当前虚构书房定义生成画面，只采用所选图片作为物品/色彩参考，不出现人物或自拍。',{referenceAsset});
 equal(output.snapshot.reference_asset.grant_id,grant.id,'Actual job must freeze the authorization');equal(output.snapshot.reference_asset.grant_revision,grant.revision,'Actual job must freeze the exact authorization revision');
 const evidence=await readImageEvidence(t,output.result.asset_id),uses=await t.pages('/api/media-reference-grants/'+encodeURIComponent(grant.id)+'/uses');check(uses.some(u=>u.id===output.id&&u.resource_id===resource.id),'Actual job must appear in grant use records');
 return {grant,referenceAsset,jobId:output.id,assetId:output.result.asset_id,evidence};
}
export async function mediaReferenceRegression(t,resource){
 const original=await t.step('uploaded-reference','An uploaded original with explicit exact-target permission is consumed by a real Studio image task',()=>grantedImage(t,'uploaded-original',resource,'object',png,t.token('reference-original')+'.png','image/png'));
 await t.step('person-reference','Explicit person reference permission uses actual selected image bytes without borrowing an Actor identity',async()=>{
  const path=t.env.FLUCTLIGHT_E2E_PERSON_REFERENCE_FILE;if(!path)t.blocked('PERSON_REFERENCE_FILE_REQUIRED','Set FLUCTLIGHT_E2E_PERSON_REFERENCE_FILE to an image you explicitly choose for the isolated person-reference test; no image or model task was fabricated');
  const bytes=new Uint8Array(await readFile(path));check(bytes.length>0&&bytes.length<=32*1024*1024,'Person reference must contain at most 32MiB actual image bytes');const mime={'.png':'image/png','.jpg':'image/jpeg','.jpeg':'image/jpeg','.gif':'image/gif','.webp':'image/webp'}[extname(path).toLowerCase()];check(!!mime,'Person reference file must be PNG/JPEG/GIF/WebP');
  const result=await grantedImage(t,'person-original',resource,'person',bytes,basename(path),mime);t.review('explicit-person-reference-pixels','Compare the actual output with the image you selected. Owner permission and successful byte consumption do not establish visual fidelity or identity verification.',{jobId:result.jobId,assetId:result.assetId,grantId:result.grant.id});return {jobId:result.jobId,assetId:result.assetId,grantId:result.grant.id,...result.evidence};
 });
 await t.step('revoke-reference','Revoked permission blocks new admission, retains actual uses and cannot be reactivated by replaying an old receipt',async()=>{
  if(!original)t.blocked('REFERENCE_GRANT_PREREQUISITE_MISSING','A completed uploaded-reference branch is required');const g=original.grant;
  const revoked=(await t.command('revoke-reference','POST','/api/media-reference-grants/'+encodeURIComponent(g.id)+'/revoke',()=>({expectedRevision:g.revision,reason,idempotencyKey:t.token('revoke-reference')}))).value;equal(revoked.status,'revoked','Revocation must persist');
  await t.command('revoked-reference-admission','POST','/api/studio/jobs',()=>({resourceId:resource.id,resourceRevision:resource.revision,kind:'image_preview',name:resource.name,body:resource.body,instruction:'This revoked reference must never start a model task',referenceAsset:original.referenceAsset,reason,idempotencyKey:t.token('revoked-reference-admission')}),{expected:[409]});
  await t.command('uploaded-original-grant','POST','/api/media-reference-grants',t.state.commands[`${t.id}:uploaded-original-grant`].body);
  const current=await t.get('/api/media-reference-grants/'+encodeURIComponent(g.id));equal(current.status,'revoked','Latest authorization must remain revoked');const history=await t.pages('/api/media-reference-grants/'+encodeURIComponent(g.id)+'/history');check(history.some(r=>r.after_value?.status==='active')&&history.some(r=>r.after_value?.status==='revoked'),'History must preserve both actual grant and revocation');
  await t.request('/api/media-reference-grants/'+encodeURIComponent(g.id),{anonymous:true,expected:[401,403,404]});
 });
}
