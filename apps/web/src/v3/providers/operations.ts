import { randomId } from '../../random-id.mjs';
import type {BrowserClient, BrowserPendingGroup} from '@fluctlight/browser-client';
// @ts-expect-error explicit v3 service registration
import {configureDomainService} from '../proto/services.js';

export function createOperationsProvider(client:BrowserClient,refresh:()=>Promise<unknown>){
  const keys=new Map<string,string>();
  async function command<T>(identity:string,run:(key:string)=>Promise<T>){
    let key=keys.get(identity);if(!key){key=randomId();keys.set(identity,key);}
    const result=await run(key);keys.delete(identity);
    // A projection refresh failure must not rewrite a committed command as
    // failed. Pages read the authority again and surface refresh warnings.
    return result;
  }
  return {
    health:()=>client.health('/health/ready'),
    notifications:(cursor='',unread=false)=>client.listNotifications({cursor,unread,limit:30}),
    notificationCount:()=>client.unreadNotificationCount(),
    readNotification:(id:string)=>command('notice:'+id,key=>client.markNotificationRead(id,{idempotencyKey:key})),
    readAllNotifications:(before:string)=>command('all-notices:'+before,key=>client.markAllNotificationsRead({before,idempotencyKey:key})),
    pending:(fid='',kind?:BrowserPendingGroup['kind'],cursor='')=>client.pending(fid,{kind,cursor,limit:20}),
    pendingCount:(fid='')=>client.pendingCount(fid),
    task:(id:string)=>client.diagnosticTask(id),
    request:(id:string)=>client.ownerRequest(id),
    lifecycle:()=>client.lifecycleDiagnostics({limit:40}),
    workflowHistory:(id:string)=>client.workflowHistory(id),
    workflowCommand:(id:string,action:'pause'|'resume'|'cancel'|'restart')=>action==='restart'?client.restartWorkflow(id):client.workflowCommand(id,action),
    refresh,
  };
}
export function installOperationsProvider(client:BrowserClient,refresh:()=>Promise<unknown>){const operations=createOperationsProvider(client,refresh);configureDomainService('operations',operations);return operations;}
