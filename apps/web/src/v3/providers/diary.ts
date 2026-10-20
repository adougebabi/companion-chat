import { randomId } from '../../random-id.mjs';
import type {BrowserClient,BrowserDiaryGenerateBody,BrowserDiaryAnnotationBody} from '@fluctlight/browser-client';
// @ts-expect-error shared v3 service facade
import {configureDomainService} from '../proto/services.js';
// @ts-expect-error server identity for pending write identities
import {get} from '../proto/store.js';
export function createDiaryProvider(client:BrowserClient){
 const keys=new Map<string,string>();
 async function write<T>(identity:unknown,call:(key:string)=>Promise<T>){const encoded=JSON.stringify([get('session')?.actorId,identity]);let key=keys.get(encoded);if(!key){key=randomId();keys.set(encoded,key);}const result=await call(key);keys.delete(encoded);return result;}
 return {
  list:(fid:string,cursor='',favorite=false)=>client.listDiary(fid,cursor,{favorite,limit:20}),
  async read(fid:string,id:string){const result=await client.diary(fid,id);if(result.fluctlight_id!==fid||result.source!=='agent')throw new Error('日记归属或来源不匹配');return result;},
  generate:(fid:string,body:Omit<BrowserDiaryGenerateBody,'idempotencyKey'>)=>write([fid,'generate',body],key=>client.generateDiary(fid,{...body,idempotencyKey:key})),
  annotate:(fid:string,id:string,body:Omit<BrowserDiaryAnnotationBody,'idempotencyKey'>)=>write([fid,id,'annotation',body],key=>client.annotateDiary(fid,id,{...body,idempotencyKey:key})),
  replies:(fid:string,id:string,cursor='')=>client.diaryReplies(fid,id,cursor),
  reply:(fid:string,id:string,text:string,expectedRevision:number)=>write([fid,id,'reply',text,expectedRevision],key=>client.replyToDiary(fid,id,{text,expectedRevision,idempotencyKey:key})),
  shared:(fid:string,cursor='')=>client.sharedExperiences(fid,cursor),
 };
}
export function installDiaryProvider(client:BrowserClient){const diary=createDiaryProvider(client);configureDomainService('diary',diary);return diary;}
