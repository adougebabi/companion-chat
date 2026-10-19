import {apiOrigin} from '../../runtime-config';
import {BrowserClient,type StudioResource,type StudioResourceQuery,type StudioCopyBody,type StudioCatalogCommand,type ApplyPreviewResponse,type ApplyResourceBody,type BrowserStudioJobBody,type BrowserStudioJobCommandBody} from '@fluctlight/browser-client';
// @ts-expect-error Existing renderer service boundary.
import {configureDomainService} from '../proto/services.js';
// @ts-expect-error Existing renderer read boundary.
import {get} from '../proto/store.js';
export function installStudioProvider(client:BrowserClient){
 const studio={
  referenceGrants:(options:{target_id:string;target_scope?:"studio_resource";cursor?:string})=>client.mediaReferenceGrants(options),
  mediaUrl:(assetId:string)=>new URL(`/api/media/${encodeURIComponent(assetId)}`,apiOrigin).toString(),
  material:(id:string)=>client.studioMaterial(id),
  materialHistory:(id:string,cursor='')=>client.studioMaterialHistory(id,cursor),
  materialRetention:(id:string,operation:'retain'|'release',body:BrowserStudioJobCommandBody)=>client.setStudioMaterialRetention(id,operation,body),
  createJob:(body:BrowserStudioJobBody)=>client.createStudioJob(body),
  job:(id:string)=>client.studioJob(id),
  jobs:(options:{resource_id?:string;cursor?:string}={})=>client.listStudioJobs(options),
  materials:(options:{resource_id?:string;cursor?:string;released?:boolean}={})=>client.listStudioMaterials(options),
  retryJob:(id:string,body:BrowserStudioJobCommandBody)=>client.retryStudioJob(id,body),
  discardJob:(id:string,body:BrowserStudioJobCommandBody)=>client.discardStudioJob(id,body),
  saveMaterial:(id:string,body:BrowserStudioJobCommandBody)=>client.saveStudioMaterial(id,body),
  list:(options:StudioResourceQuery={})=>client.listStudioResources(options),
  versions:(id:string,options:{cursor?:string;limit?:number}={})=>client.studioVersions(id,options),
  copy:(id:string,body:StudioCopyBody)=>client.copyStudioResource(id,body),
  archive:(id:string,body:StudioCatalogCommand)=>client.archiveStudioResource(id,body),
  restore:(id:string,body:StudioCatalogCommand)=>client.restoreStudioResource(id,body),
  publication:(id:string,version:string)=>client.studioPublication(id,version),
  resource:(id:string)=>client.studioResource(id),
  save:(id:string|null,fields:{kind:StudioResource['kind'];name:string;body:Record<string,unknown>;expectedVersion?:string;reason:string;idempotencyKey:string})=>id?client.updateStudioResource(id,fields):client.createStudioResource(fields),
  async publish(id:string,revision:number,key:string){await client.publishStudioResource(id,{expectedRevision:revision,reason:'所有者发布保存的资源定义',idempotencyKey:key});return client.studioResource(id);},
  preview:(fid:string,id:string,version:string):Promise<ApplyPreviewResponse>=>client.previewStudioApply(id,{fluctlightId:fid,resourceVersion:version}),
  apply:(id:string,body:ApplyResourceBody)=>{const character=get('characters',body.fluctlightId);if(!character||character.status==='已退休')throw new Error('目标角色不可写；请保留原提交并核对角色状态');return client.applyStudioResource(id,body);},
 };
 configureDomainService('studio',studio);return studio;
}
