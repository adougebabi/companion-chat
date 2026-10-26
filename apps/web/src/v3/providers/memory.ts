import type { BrowserClient, ManagedMemory, MemoryBrowseOptions } from '@fluctlight/browser-client';
// shared v3 service facade
import { configureDomainService } from '../proto/services.js';
// shared v3 server snapshot
import { get } from '../proto/store.js';

export function createMemoryProvider(client: BrowserClient, tell: (fid:string,text:string)=>Promise<void>) {
  async function read(fid:string,id:string) {
    const memory=await client.managedMemory(id);
    if(memory.owner_fluctlight_id!==fid)throw new Error('记忆不属于当前角色');
    return memory;
  }
  function evidence(reason:string) {
    const actor=get('session')?.actorId;
    if(!actor || !reason.trim())throw new Error('请填写修订说明并确认已登录');
    return [`owner:${actor}:${reason.trim()}`];
  }
  async function change(fid:string,base:ManagedMemory,operation:'revise'|'forget'|'rollback',reason:string,value?:string|number) {
    if(get('characters',fid)?.status==='已退休')throw new Error('当前角色不可写入');
    await read(fid,base.id);
    const command={expectedRevision:base.revision,evidenceRefs:evidence(reason)};
    const result=operation==='revise'?await client.reviseMemory(base.id,{...command,content:String(value)}):operation==='forget'?await client.forgetMemory(base.id,command):await client.rollbackMemory(base.id,{...command,targetRevision:Number(value)});
    const current=await read(fid,base.id);
    if(current.revision!==result.revision || (operation==='revise'&&current.content!==value) || (operation==='forget'&&current.status!=='forgotten'))throw new Error('操作已提交，但最新版本尚未确认一致；请重新读取，草稿仍保留');
    return current;
  }
  return {
    list:(fid:string,options:MemoryBrowseOptions)=>client.managedMemories(fid,options),
    read,
    async revisions(fid:string,id:string,cursor=''){await read(fid,id);return client.memoryRevisions(id,{cursor,limit:20});},
    async sources(fid:string,id:string,revision:number,cursor=''){await read(fid,id);return client.memorySources(id,{revision,cursor,limit:20});},
    revise:(fid:string,base:ManagedMemory,content:string,reason:string)=>change(fid,base,'revise',reason,content),
    forget:(fid:string,base:ManagedMemory,reason:string)=>change(fid,base,'forget',reason),
    rollback:(fid:string,base:ManagedMemory,target:number,reason:string)=>change(fid,base,'rollback',reason,target),
    async tell(fid:string,id:string,text:string){const memory=await read(fid,id);if(!text.trim())throw new Error('请填写要告诉她的内容');await tell(fid,`关于你的记忆“${memory.content}”（记忆 ${memory.id}，第 ${memory.revision} 版）：${text.trim()}`);},
  };
}
export function installMemoryProvider(client:BrowserClient,tell:(fid:string,text:string)=>Promise<void>){const memory=createMemoryProvider(client,tell);configureDomainService('memory',memory);return memory;}
