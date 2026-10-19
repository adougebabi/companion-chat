import type {BrowserClient, BrowserAutonomyPolicyWriteBody} from '@fluctlight/browser-client';
// @ts-expect-error shared explicit v3 facade
import {configureDomainService} from '../proto/services.js';
export function createAutonomyProvider(client:BrowserClient){
 const identities=new Map<string,string>();
 return {
  async read(fid:string){const policy=await client.autonomyPolicy(fid);if(policy.fluctlight_id!==fid||policy.runtime_effect!=='enforced')throw new Error('当前服务尚未提供可执行的角色自治策略');return policy;},
  async save(fid:string,body:Omit<BrowserAutonomyPolicyWriteBody,'idempotencyKey'>){const identity=JSON.stringify([fid,body]);let key=identities.get(identity);if(!key){key=crypto.randomUUID();identities.set(identity,key);}const result=await client.saveAutonomyPolicy(fid,{...body,idempotencyKey:key});identities.delete(identity);return result;},
 };
}
export function installAutonomyProvider(client:BrowserClient){const autonomy=createAutonomyProvider(client);configureDomainService('autonomy',autonomy);return autonomy;}
