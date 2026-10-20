import { randomId } from '../../random-id.mjs';
import type {BrowserClient} from '@fluctlight/browser-client';
// @ts-expect-error shared explicit v3 facade
import {configureDomainService} from '../proto/services.js';
// @ts-expect-error server snapshot identity
import {get} from '../proto/store.js';

export function createReflectionProvider(client:BrowserClient){
  const keys=new Map<string,string>();
  async function read(fid:string,id:string){const row=await client.reflection(fid,id);if(row.fluctlight_id!==fid)throw new Error('反思范围不匹配');return row;}
  return {
    list:(fid:string,cursor='')=>client.listReflections(fid,{cursor,limit:30}),
    read,
    feedback:(fid:string,id:string,cursor='')=>client.reflectionFeedback(fid,id,{cursor,limit:20}),
    async submit(fid:string,id:string,text:string,stance:'agree'|'disagree'){
      if(!text.trim())throw new Error('请填写反馈内容');
      if(get('characters',fid)?.status==='已退休')throw new Error('当前角色不可提交反馈');
      await read(fid,id);
      const identity=JSON.stringify([get('session')?.actorId,fid,id,text,stance]);let key=keys.get(identity);if(!key){key=randomId();keys.set(identity,key);}
      const result=await client.submitReflectionFeedback(fid,id,{text,stance,idempotencyKey:key});
      if(result.request.fluctlight_id!==fid||result.request.kind!=='reflection_feedback'||result.request.payload.reflection_id!==id)throw new Error('反馈已受理，但任务范围不匹配，请重新查询');
      keys.delete(identity);return result;
    },
  };
}
export function installReflectionProvider(client:BrowserClient){const reflection=createReflectionProvider(client);configureDomainService('reflection',reflection);return reflection;}
