import type {BrowserClient,BrowserMediaMetadataBody,BrowserMediaAvatarBody,BrowserMediaReferenceGrantBody,BrowserMediaReferenceRevokeBody,BrowserMediaGenerateBody,BrowserMediaPhotoBody,BrowserMediaDeleteBody,BrowserMediaDeleteRetryBody} from '@fluctlight/browser-client';
// v3 service facade
import {configureDomainService} from '../proto/services.js';
import {apiOrigin} from '../../runtime-config';
export function createMediaProvider(client:BrowserClient,refreshProduct:()=>Promise<unknown>){return {
 refresh:refreshProduct,
 references:(id:string,cursor='')=>client.mediaAssetReferences(id,cursor),
 deleteAsset:(id:string,body:BrowserMediaDeleteBody)=>client.deleteMediaAsset(id,body),
 deletion:(id:string)=>client.mediaDeletion(id),
 retryDeletion:(id:string,body:BrowserMediaDeleteRetryBody)=>client.retryMediaDeletion(id,body),
 generate:(fid:string,body:BrowserMediaGenerateBody)=>client.requestMediaGeneration(fid,body),
 photo:(fid:string,id:string,body:BrowserMediaPhotoBody)=>client.requestItemPhoto(fid,id,body),
 generation:(fid:string,id:string)=>client.mediaGenerationRequest(fid,id),
 generations:(fid:string,cursor='')=>client.mediaGenerationRequests(fid,cursor),
 locations:(fid:string)=>client.world(fid),
 grants:(options:Parameters<BrowserClient['mediaReferenceGrants']>[0])=>client.mediaReferenceGrants(options),
 grant:(id:string)=>client.mediaReferenceGrant(id),
 createGrant:(body:BrowserMediaReferenceGrantBody)=>client.createMediaReferenceGrant(body),
 revokeGrant:(id:string,body:BrowserMediaReferenceRevokeBody)=>client.revokeMediaReferenceGrant(id,body),
 grantRecords:(id:string,kind:'history'|'uses',cursor='')=>client.mediaReferenceGrantRecords(id,kind,cursor),
 avatar:(fid:string)=>client.displayAvatar(fid),
 selectAvatar:(fid:string,body:BrowserMediaAvatarBody)=>client.selectDisplayAvatar(fid,body),
 avatarHistory:(fid:string,cursor='')=>client.displayAvatarHistory(fid,cursor),
 list:(options:Parameters<BrowserClient['ownerMediaLibrary']>[0])=>client.ownerMediaLibrary(options),
 asset:(id:string,fluctlightId?:string)=>client.ownerMediaAsset(id,fluctlightId),
 lookup:(key:string)=>client.ownerMediaUploadByKey(key),
 upload:(file:File,description:{mime:string;filename:string},key:string)=>client.uploadOwnerMedia(file,description.mime,key,description.filename),
 save:(id:string,body:BrowserMediaMetadataBody)=>client.saveMediaMetadata(id,body),
 url:(id:string)=>new URL(`/api/media/${encodeURIComponent(id)}`,apiOrigin).href,
};}
export function installMediaProvider(client:BrowserClient,refreshProduct:()=>Promise<unknown>){const media=createMediaProvider(client,refreshProduct);configureDomainService('media',media);return media;}
