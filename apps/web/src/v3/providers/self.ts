import type {BrowserClient,BrowserSelfCommandBody,BrowserSelfRestoreBody} from '@fluctlight/browser-client';
// @ts-expect-error explicit v3 facade
import {configureDomainService} from '../proto/services.js';
// @ts-expect-error authenticated snapshot identity
import {get} from '../proto/store.js';
export function createSelfProvider(client:BrowserClient){
 const identities=new Map<string,string>();
 return {
  list:(fid:string,options:{q?:string;status?:string;category?:string;cursor?:string}={})=>client.selfClaims(fid,{...options,limit:20}),
  async read(fid:string,id:string){const row=await client.selfClaim(fid,id);if(row.fluctlight_id!==fid)throw new Error('认识对象不属于当前角色');return row;},
  history:(fid:string,id:string,kind:'revisions'|'reviews',cursor='')=>client.selfClaimHistory(fid,id,kind,cursor),
  async summary(fid:string){const detail=await client.detail(fid);if(!Number.isSafeInteger(detail.current_revision))throw new Error('人格当前版本缺失，请重新读取');return detail;},
  reflections:(fid:string,cursor='')=>client.listReflections(fid,{cursor,limit:5}),
  async command(fid:string,id:string,...args:[operation:'review'|'forget',body:Omit<BrowserSelfCommandBody,'idempotencyKey'>]|[operation:'restore',body:Omit<BrowserSelfRestoreBody,'idempotencyKey'>]){
   const [operation,body]=args;
   const identity=JSON.stringify([get('session')?.actorId,fid,id,operation,body]);let key=identities.get(identity);if(!key){key=crypto.randomUUID();identities.set(identity,key);}
   const result=await (operation==='restore'?client.manageSelfClaim(fid,id,operation,{...(body as Omit<BrowserSelfRestoreBody,'idempotencyKey'>),idempotencyKey:key}):client.manageSelfClaim(fid,id,operation,{...body,idempotencyKey:key}));if(result.fluctlight_id!==fid||result.id!==id)throw new Error('操作已提交，但返回的对象范围不匹配，请重新查询');identities.delete(identity);return result;
  },
  decideFoundation:(fid:string,id:string,accept:boolean,expectedRevision:number,reason:string)=>accept?client.acceptFoundationRevision(fid,id,{expectedRevision,reason}):client.rejectFoundationRevision(fid,id,{expectedRevision,reason}),
 };
}
export function installSelfProvider(client:BrowserClient){const self=createSelfProvider(client);configureDomainService('self',self);return self;}
