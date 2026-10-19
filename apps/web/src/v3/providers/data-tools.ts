import type {BrowserClient,BrowserResourcePack,BrowserResourcePackImportBody,BrowserResourcePackGroupBody,BrowserBusinessBackupBody} from '@fluctlight/browser-client';
// @ts-expect-error explicit v3 domain facade
import {configureDomainService} from '../proto/services.js';
export function installDataToolsProvider(client:BrowserClient,refresh:()=>Promise<unknown>){const dataTools={refresh,
 backups:(cursor='')=>client.businessBackups(cursor),
 backup:(id:string)=>client.businessBackup(id),
 backupCommand:(id:string,action:'create'|'restore'|'retry'|'cancel',body:BrowserBusinessBackupBody)=>client.businessBackupCommand(id,action,body),
 exportPack:(id='')=>client.exportResourcePack(id),
 validatePack:(pack:BrowserResourcePack)=>client.validateResourcePack(pack),
 importPack:(body:BrowserResourcePackImportBody)=>client.importResourcePack(body),
 pack:(id:string)=>client.resourcePack(id),
 adoptGroup:(id:string,index:number,body:BrowserResourcePackGroupBody)=>client.adoptResourcePackGroup(id,index,body),
 revisions:(options:Record<string,string>={})=>client.ownerObjectRevisions(options),
};configureDomainService('dataTools',dataTools);return dataTools;}
