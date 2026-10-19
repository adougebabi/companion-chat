import {check,equal,hash} from './runtime.mjs';
import {reason,fidPath,lifeSnapshot,sameInventoryAndAppearance} from './domain.mjs';
// A fixed valid PNG tests upload bytes without asking a model to manufacture it.
const png=new Uint8Array(Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=','base64'));
export async function ownerMediaRegression(t){
 const asset=await t.step('upload','Owner upload persists actual original bytes and confirms the same identity after refresh',async()=>{
  const receipt=(await t.upload('original-png',{filename:t.token('owner-media')+'.png',mime:'image/png',bytes:png})).value;
  equal(receipt.sha256,hash(png),'Upload receipt checksum must match fixture bytes');equal(receipt.byte_size,png.length,'Upload byte count must match actual content');equal(receipt.factual,false,'Owner material cannot become character fact');
  const key=t.state.commands['L09:upload:original-png'].idempotencyKey,lookup=await t.get('/api/media-uploads/by-key?'+new URLSearchParams({key}));
  equal(lookup.status,'ready','Refresh lookup must confirm a persisted producer');equal(lookup.asset_id,receipt.asset_id,'Lookup cannot substitute another asset');equal(lookup.asset.id,receipt.asset_id,'Lookup must include the actual ready asset');
  const replay=(await t.upload('original-png',{filename:'must-not-replace-original.png',mime:'image/png',bytes:new Uint8Array([1])})).value;
  equal(replay.asset_id,receipt.asset_id,'Same identity must not create a second asset');equal(replay.replayed,true,'Replay must be explicit');
  await t.remember('ownerMediaAsset',{id:receipt.asset_id});return lookup.asset;
 });
 await t.step('upload-bytes','Owner media library, kind filter, checksum and private byte authorization agree',async()=>{
  if(!asset)t.blocked('UPLOAD_PREREQUISITE_MISSING','A real upload is required');
  const detail=await t.get('/api/media-assets/'+encodeURIComponent(asset.id));for(const [key,value]of Object.entries({origin:'owner_upload',status:'ready',readable:true,source_state:'owner_original',factual:false}))equal(detail[key],value,'Upload metadata field '+key+' must be truthful');
  const rows=await t.pages('/api/media-library?'+new URLSearchParams({scope:'owner_upload',kind:'image',q:detail.title,hidden:String(detail.hidden)}));equal(rows.filter(r=>r.id===asset.id).length,1,'Paged query must include the exact asset once');
  const r=await t.request('/api/media/'+encodeURIComponent(asset.id),{stream:true}),bytes=new Uint8Array(await r.arrayBuffer());equal(bytes.length,detail.byte_size,'Proxy must return actual full bytes');equal(hash(bytes),detail.sha256,'Proxy bytes must match database SHA256');equal(hash(bytes),hash(png),'Proxy cannot substitute another object');
  await t.request('/api/media/'+encodeURIComponent(asset.id),{anonymous:true,expected:[401,403,404]});await t.request('/api/media-assets/'+encodeURIComponent(asset.id),{anonymous:true,expected:[401,403,404]});return {assetId:asset.id,bytes:bytes.length,sha256:hash(bytes)};
 });
 await t.step('upload-metadata','Dual CAS rejects stale metadata; hide and restore persist without changing bytes',async()=>{
  if(!asset)t.blocked('UPLOAD_PREREQUISITE_MISSING','A real upload is required');const path='/api/media-assets/'+encodeURIComponent(asset.id),before=await t.get(path);
  const hidden=(await t.command('hide-upload','PUT',path,()=>({expectedAssetRevision:before.asset_revision,expectedMetadataRevision:before.metadata_revision,title:t.token('renamed-media'),hidden:true,reason,idempotencyKey:t.token('hide-upload')}))).value;
  equal(hidden.asset_revision,before.asset_revision,'Annotation cannot change bytes revision');check(hidden.metadata_revision>t.state.commands['L09:hide-upload'].body.expectedMetadataRevision,'Metadata must advance from original baseline');
  const original=t.state.commands['L09:hide-upload'].body;
  await t.command('stale-upload-metadata','PUT',path,()=>({...original,title:'stale revision must not win',idempotencyKey:t.token('stale-upload-metadata')}),{expected:[409]});
  const restored=(await t.command('restore-upload','PUT',path,()=>({expectedAssetRevision:hidden.asset_revision,expectedMetadataRevision:hidden.metadata_revision,title:hidden.title,hidden:false,reason,idempotencyKey:t.token('restore-upload')}))).value;
  equal(restored.hidden,false,'Restore visibility must persist');const reread=await t.get(path);equal(reread.metadata_revision,restored.metadata_revision,'New query must see the saved revision');equal(reread.sha256,asset.sha256,'Annotation cannot replace media bytes');
  const visible=await t.pages('/api/media-library?'+new URLSearchParams({scope:'owner_upload',q:restored.title})),hiddenRows=await t.pages('/api/media-library?'+new URLSearchParams({scope:'owner_upload',q:restored.title,hidden:'true'}));check(visible.some(r=>r.id===asset.id),'Restored material must return to default library');check(!hiddenRows.some(r=>r.id===asset.id),'Restored material must leave hidden-only library');
 });
}

export async function ownerAvatarRegression(t,f){
 await t.step('avatar','Explicit display selection persists across detail reads without altering canonical or effective appearance',async()=>{
  const uploaded=t.object('ownerMediaAsset');if(!uploaded)t.blocked('UPLOAD_PREREQUISITE_MISSING','A real upload is required for display selection');
  const asset=await t.get('/api/media-assets/'+encodeURIComponent(uploaded.id)),before=await t.get(fidPath(f.id)+'/display-avatar'),life=await lifeSnapshot(t,f.id),visual=await t.get(fidPath(f.id)+'/visual-identity');
  const selected=(await t.command('display-avatar','PUT',fidPath(f.id)+'/display-avatar',()=>({mode:'asset',assetId:asset.id,expectedAssetRevision:asset.asset_revision,expectedRevision:before.revision,reason,idempotencyKey:t.token('display-avatar')}))).value;
  equal(selected.mode,'asset','Selection is a display preference');equal(selected.avatar?.asset_id,asset.id,'Receipt must use actual authorized image');
  const current=await t.get(fidPath(f.id)+'/display-avatar'),detail=await t.get(fidPath(f.id)+'/detail');equal(current.avatar?.asset_id,asset.id,'Refresh must preserve the display preference');equal(detail.avatar?.asset_id,asset.id,'Ordinary profile detail must consume the same display preference');
  const after=await t.get(fidPath(f.id)+'/visual-identity');equal(after.canonical_asset_id,visual.canonical_asset_id,'Display selection must not replace canonical identity');equal(after.current_revision,visual.current_revision,'Display selection must not commit a visual identity revision');sameInventoryAndAppearance(life,await lifeSnapshot(t,f.id));
  const history=await t.pages(fidPath(f.id)+'/display-avatar/history');check(history.some(row=>row.after_value?.asset_id===asset.id&&row.revision===selected.revision),'An immutable display selection history must retain the chosen source');
  return {assetId:asset.id,revision:current.revision,canonicalAssetId:after.canonical_asset_id||null};
 });
}
