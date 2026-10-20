import { randomId } from '../../random-id.mjs';
import type {BrowserClient,BrowserVisualBriefBody,BrowserVisualGenerateBody,BrowserVisualReviewBody} from '@fluctlight/browser-client';
// @ts-expect-error v3 service facade
import {configureDomainService} from '../proto/services.js';
// @ts-expect-error authenticated draft scope
import {get} from '../proto/store.js';
export function createVisualProvider(client:BrowserClient){
 const keys=new Map<string,string>();
 async function command<T>(scope:unknown,body:object,execute:(key:string)=>Promise<T>){const identity=JSON.stringify([get('session')?.actorId,scope,body]);let key=keys.get(identity);if(!key){key=randomId();keys.set(identity,key);}const result=await execute(key);keys.delete(identity);return result;}
 return {
  async read(fid:string){const value=await client.visualBrief(fid);if(value.fluctlight_id!==fid)throw new Error('视觉定义范围不匹配');return value;},
  async identity(fid:string){const value=await client.visualIdentityManagement(fid);if(value.fluctlight_id!==fid)throw new Error('视觉身份范围不匹配');return value;},
  history:(fid:string,cursor='')=>client.visualBriefHistory(fid,cursor),
  timeline:(fid:string,cursor='')=>client.visualIdentityTimeline(fid,cursor),
  save:(fid:string,body:Omit<BrowserVisualBriefBody,'idempotencyKey'>)=>command([fid,'visual-brief'],body,key=>client.saveVisualBrief(fid,{...body,idempotencyKey:key})),
  review:(fid:string,id:string,body:Omit<BrowserVisualReviewBody,'idempotencyKey'>)=>command([fid,id,'manual-review'],body,key=>client.reviewVisualIdentity(fid,id,{...body,idempotencyKey:key})),
  generate:(fid:string,body:Omit<BrowserVisualGenerateBody,'idempotencyKey'>)=>command([fid,'visual-generate'],body,key=>client.generateVisualIdentity(fid,{...body,idempotencyKey:key})),
 };
}
export function installVisualProvider(client:BrowserClient){const visual=createVisualProvider(client);configureDomainService('visual',visual);return visual;}
