import { randomId } from '../../random-id.mjs';
import type {BrowserClient,BrowserCapabilityReviewBody,BrowserFoundationProposalBody} from '@fluctlight/browser-client';
// explicit v3 service facade
import {configureDomainService} from '../proto/services.js';
// authenticated draft scope
import {get} from '../proto/store.js';
export function createGovernanceProvider(client:BrowserClient,refresh:()=>Promise<unknown>){
 const keys=new Map<string,string>();
 async function command<T>(scope:unknown,body:object,execute:(key:string)=>Promise<T>){const identity=JSON.stringify([get('session')?.actorId,scope,body]);let key=keys.get(identity);if(!key){key=randomId();keys.set(identity,key);}const result=await execute(key);keys.delete(identity);return result;}
 const owned=<T extends {fluctlight_id:string}>(fid:string,row:T)=>{if(row.fluctlight_id!==fid)throw new Error('治理对象与当前角色不匹配，请重新读取');return row;};
 return {
  refresh,
  async current(fid:string){const detail=await client.detail(fid);if(!Number.isSafeInteger(detail.current_revision))throw new Error('身份当前版本缺失');return {...detail,revision:detail.current_revision};},
  list:(fid:string,kind:'foundation'|'capability',options:Record<string,string|number>={})=>client.governancePage(fid,kind,{...options,limit:20}),
  async foundation(fid:string,id:string){return owned(fid,await client.governanceObject(fid,'foundation',id));},
  async capability(fid:string,id:string){return owned(fid,await client.governanceObject(fid,'capability',id));},
  history:(fid:string,kind:'foundation'|'capability',id:string,cursor='')=>client.governanceHistory(fid,kind,id,cursor),
  propose:(fid:string,body:Omit<BrowserFoundationProposalBody,'idempotencyKey'>)=>command([fid,'propose'],body,key=>client.submitFoundationRevision(fid,{...body,idempotencyKey:key})),
  decide:(fid:string,id:string,action:'accept'|'reject',body:{expectedRevision:number;reason:string})=>action==='accept'?client.acceptFoundationRevision(fid,id,body):client.rejectFoundationRevision(fid,id,body),
  restore:(fid:string,targetRevision:number,body:{expectedRevision:number;reason:string})=>client.rollbackFoundationRevision(fid,{...body,targetRevision}),
  review:(fid:string,id:string,body:Omit<BrowserCapabilityReviewBody,'idempotencyKey'>)=>command([fid,id,'review'],body,async key=>owned(fid,await client.reviewScopedCapabilityRequest(fid,id,{...body,idempotencyKey:key}))),
 };
}
export function installGovernanceProvider(implementation:ReturnType<typeof createGovernanceProvider>){configureDomainService('governance',implementation);}
