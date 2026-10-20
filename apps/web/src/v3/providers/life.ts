import { randomId } from '../../random-id.mjs';
import {persistentCommand} from './persistent-command.mjs';
import type { BrowserClient, BrowserGoalCommand, BrowserPromiseCommandBody, BrowserPromiseRescheduleBody } from '@fluctlight/browser-client';
// @ts-expect-error JS service facade
import { configureDomainService } from '../proto/services.js';
// @ts-expect-error JS snapshot facade
import { get } from '../proto/store.js';

type Data = Record<string, unknown>;
const object = (value: unknown): Data => value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Data : {};
export function createLifeProvider(client: BrowserClient) {
  const command = (reason: string) => ({reason,idempotencyKey:randomId()});
  const writable = (fid: string) => {const row=get('characters',fid);if(!row || row.status==='已退休')throw new Error('当前角色不可写入');};
  const pendingKeys = new Map<string,string>();
  async function mutate<T>(fid:string,operation:string,body:Data,run:(key:string)=>Promise<T>) {
    writable(fid);
    const identity=JSON.stringify([get('session')?.actorId,fid,operation,body]);
    let key=pendingKeys.get(identity);if(!key){key=randomId();pendingKeys.set(identity,key);}
    const result=await run(key);pendingKeys.delete(identity);return result;
  }
  type PromiseDraft={title:string;startsAt?:string;dueAt:string;reason:string};
  const proposalChannels=new Map<string,ReturnType<typeof persistentCommand<PromiseDraft,Awaited<ReturnType<BrowserClient['proposePromise']>>>>>();
  function proposalChannel(fid:string){
    const actor=get('session')?.actorId;if(typeof actor!=='string'||!actor)throw new Error('请先登录再提交约定');
    const key='fluctlight:pending-promise:'+JSON.stringify([actor,fid]);
    let channel=proposalChannels.get(key);if(!channel){channel=persistentCommand<PromiseDraft,Awaited<ReturnType<BrowserClient['proposePromise']>>>({key,storage:()=>{if(!globalThis.sessionStorage)throw new Error('无法保留提交身份，请检查浏览器会话存储后再提交。');return globalThis.sessionStorage;},execute:(value,idempotencyKey)=>{writable(fid);if(get('session')?.actorId!==actor)throw new Error('登录身份已经变化，请回到原账户查询提交结果');return client.proposePromise(fid,{...value,idempotencyKey});}});proposalChannels.set(key,channel);}
    return channel;
  }
  const life = {
    detail: (fid:string) => client.detail(fid),
    async now(fid:string) {
      const [detail,wardrobe,nearby] = await Promise.all([client.detail(fid),client.wardrobe(fid),client.nearby(fid)]);
      return {detail,wardrobe,nearby};
    },
    wardrobe: (fid:string,cursor='') => client.wardrobe(fid,cursor),
    addItem: async (fid:string,body:Data) => {writable(fid);return client.addWardrobeItems(fid,body);},
    updateItem: async(fid:string,id:string,body:Data) => {writable(fid);return client.updateWardrobeItem(fid,id,body);},
    removeItem: async(fid:string,id:string) => {writable(fid);return client.deleteWardrobeItem(fid,id);},
    wear: (fid:string,itemId:string,slot:string,revision:number,reason:string) => mutate(fid,'wear',{itemId,slot,revision,reason},idempotencyKey=>client.requestWardrobeWear(fid,{itemId,slot,expectedRevision:revision,reason,idempotencyKey})),
    use: (fid:string,itemId:string,active:boolean,revision:number,reason:string) => mutate(fid,'use',{itemId,active,revision,reason},idempotencyKey=>client.requestItemUse(fid,itemId,{active,expectedRevision:revision,evidenceRefs:[`owner:${get('session')?.actorId}`],reason,idempotencyKey})),
    photo: (fid:string,itemId:string,body:Omit<import("@fluctlight/browser-client").BrowserMediaPhotoBody,"idempotencyKey">,idempotencyKey:string) => {writable(fid);return client.requestItemPhoto(fid,itemId,{...body,idempotencyKey});},
    request: (fid:string,id:string) => client.ownerRequest(id).then(request=>{if(request.fluctlight_id!==fid)throw new Error('请求范围不匹配');return request;}),
    requests: (fid:string,cursor?:string) => client.listOwnerRequests(fid,{cursor}),
    // Core partitions terminal history from current goals; true is not “include history”.
    goals: (fid:string,cursor?:string) => client.goals(fid,{history:false,limit:50,cursor}),
    goal: (fid:string,id:string) => client.goalDetail(fid,id),
    goalHistory: (fid:string,id:string,cursor='') => client.goalHistory(fid,id,cursor),
    goalEvidence: (fid:string,id:string,cursor='') => client.goalEvidence(fid,id,cursor),
    saveGoal: async(fid:string,id:string|null,body:BrowserGoalCommand) => {writable(fid);return id?client.goalCommand(fid,id,'update',body):client.createGoal(fid,body);},
    goalCommand: async(fid:string,id:string,action:'pause'|'resume'|'cancel'|'abandon'|'reassess',revision:number,reason:string) => {writable(fid);return client.goalCommand(fid,id,action,{expectedRevision:revision,...command(reason)});},
    goalSet: (fid:string)=>client.goalSet(fid),
    planningHistory: (fid:string)=>client.goalPlanningHistory(fid),
    plan: async(fid:string)=>{writable(fid);return client.requestGoalPlanning(fid,randomId());},
    updateGoalSet: async(fid:string,state:Data,changes:Data,reason:string)=>{writable(fid);return client.updateGoalSet(fid,{...changes,expectedVersion:Number(state.revision),expectedFactsRevision:String(state.facts_revision),...command(reason)});},
    promises:(fid:string,cursor='')=>client.promises(fid,cursor),
    readPromise:(fid:string,id:string)=>client.promise(fid,id),
    promiseHistory:(fid:string,id:string,cursor='')=>client.promiseHistory(fid,id,cursor),
    promiseCommand:(fid:string,id:string,operation:'cancel'|'retry',body:Omit<BrowserPromiseCommandBody,'idempotencyKey'>)=>mutate(fid,'promise-'+operation,{id,...body},idempotencyKey=>client.promiseCommand(fid,id,operation,{...body,idempotencyKey})),
    reschedulePromise:(fid:string,id:string,body:Omit<BrowserPromiseRescheduleBody,'idempotencyKey'>)=>mutate(fid,'promise-reschedule',{id,...body},idempotencyKey=>client.reschedulePromise(fid,id,{...body,idempotencyKey})),
    pendingPromise:(fid:string)=>proposalChannel(fid).pending(),
    retryPromise:(fid:string)=>proposalChannel(fid).retry(),
    promise: (fid:string,body:PromiseDraft)=>{writable(fid);const normalized={title:body.title,dueAt:body.dueAt,reason:body.reason,...(body.startsAt?{startsAt:body.startsAt}:{})};return proposalChannel(fid).submit(normalized);},
    async scheduleItem(fid:string,detail:Data,item:Data|null,body:Data,operation:string) {
      writable(fid);
      const schedule=object(detail.schedule), context=object(detail.context);
      if(typeof schedule.id!=='string' || !Number.isSafeInteger(schedule.revision) || typeof context.context_revision!=='string')throw new Error('当前日程或生活版本缺失，请重新读取后再建议');
      const scheduleId=schedule.id,lifeRevision=context.context_revision;
      return mutate(fid,'schedule-item',{scheduleId,itemId:item?.id,operation,...body,revision:schedule.revision,lifeRevision},idempotencyKey=>{
        const common={expectedRevision:Number(schedule.revision),expectedLifeContextRevision:lifeRevision,reason:String(body.reason),idempotencyKey};
        if(operation==='cancel')return client.deleteScheduleItem(fid,scheduleId,String(item?.id),common);
        const request={...common,activity:String(body.activity),scene:String(body.scene),location:String(body.location || ''),startAt:String(body.startAt),endAt:String(body.endAt),evidenceRefs:[`owner:${get('session')?.actorId}`],...(body.placeId?{placeId:String(body.placeId)}:{})};
        return item?client.updateScheduleItem(fid,scheduleId,String(item.id),request):client.suggestScheduleItem(fid,scheduleId,request);
      });
    },
  };
  return life;
}
export function installLifeProvider(client:BrowserClient) {const life=createLifeProvider(client);configureDomainService('life',life);return life;}
