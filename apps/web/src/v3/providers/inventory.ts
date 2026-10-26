import { randomId } from '../../random-id.mjs';
import type {BrowserClient,BrowserInventoryCreateBody,BrowserInventoryUpdateBody,BrowserInventoryArchiveBody,BrowserInventoryActionBody,BrowserInventoryOutfitWriteBody} from '@fluctlight/browser-client';
// v3 service facade
import {configureDomainService} from '../proto/services.js';
// authenticated command identity
import {get} from '../proto/store.js';
type WithoutKey<T> = T extends unknown ? Omit<T,'idempotencyKey'> : never;
export function createInventoryProvider(client:BrowserClient){
 const keys=new Map<string,string>();
 async function command<T>(scope:unknown,body:object,execute:(key:string)=>Promise<T>){const identity=JSON.stringify([get('session')?.actorId,scope,body]);let key=(body as {idempotencyKey?:string}).idempotencyKey||keys.get(identity);if(!key){key=randomId();keys.set(identity,key);}const result=await execute(key);keys.delete(identity);return result;}
 return {
  async clothingCatalogue(fid:string){
   const items:import('@fluctlight/browser-client').BrowserInventoryItem[]=[],seen=new Set<string>();let cursor='',revision:number|undefined;
   do {const page=await client.inventoryPage(fid,{kind:'wearable',state:'current',limit:100,cursor});if(revision!==undefined&&page.revision!==revision)throw new Error('读取期间衣柜已变化，请重新读取后继续搭配。');revision=page.revision;items.push(...page.items);cursor=page.next_cursor;if(cursor&&seen.has(cursor))throw new Error('服装分页游标重复，请重新读取。');seen.add(cursor);}while(cursor);
   return {items,revision:revision!};
  },
  preparation:(fid:string)=>client.inventoryPreparation(fid),
  prepare:(fid:string,body:Omit<import('@fluctlight/browser-client').BrowserInventoryPreparationBody,'idempotencyKey'>)=>command([fid,'inventory-preparation'],body,key=>client.prepareInventory(fid,{...body,idempotencyKey:key})),
  tags:(fid:string)=>client.inventoryTags(fid),
  saveTags:(fid:string,body:Omit<import('@fluctlight/browser-client').BrowserInventoryTagsBody,'idempotencyKey'>)=>command([fid,'inventory-tags'],body,key=>client.saveInventoryTags(fid,{...body,idempotencyKey:key})),
  async list(fid:string,options:Record<string,string|number>={}){const [page,wardrobe]=await Promise.all([client.inventoryPage(fid,{...options,limit:20}),client.wardrobe(fid)]);if(!Array.isArray(wardrobe.worn_items)||!Array.isArray(wardrobe.used_items))throw new Error('当前穿着或使用读面未完整返回，请重新读取');return {...page,worn_items:wardrobe.worn_items,used_items:wardrobe.used_items};},
  async read(fid:string,id:string){const item=await client.inventoryItem(fid,id);if(item.fluctlight_id!==fid)throw new Error('物品不属于当前角色');return item;},
  async outfits(fid:string,cursor=''){const page=await client.inventoryOutfits(fid,cursor);if(typeof page.profile_id!=='string'||!page.profile_id||page.items.some(outfit=>outfit.fluctlight_id!==fid))throw new Error('套装人格或角色范围未完整返回');return page;},
  async outfit(fid:string,id:string){const outfit=await client.inventoryOutfit(fid,id);if(outfit.fluctlight_id!==fid||typeof outfit.active_profile_id!=='string'||!outfit.active_profile_id)throw new Error('套装不属于当前角色或未提供当前人格');return outfit;},
  outfitHistory:(fid:string,id:string,cursor='')=>client.inventoryOutfitRevisions(fid,id,cursor),
  renameOutfit:(fid:string,id:string,body:Omit<import('@fluctlight/browser-client').BrowserInventoryOutfitRenameBody,'idempotencyKey'>)=>command([fid,id,'outfit-rename'],body,key=>client.renameInventoryOutfit(fid,id,{...body,idempotencyKey:key})),
  saveOutfit:(fid:string,id:string|null,body:Omit<BrowserInventoryOutfitWriteBody,'idempotencyKey'>)=>command([fid,id,'outfit-save'],body,key=>id?client.updateInventoryOutfit(fid,id,{...body,idempotencyKey:key}):client.createInventoryOutfit(fid,{...body,idempotencyKey:key})),
  action:(fid:string,body:WithoutKey<BrowserInventoryActionBody>)=>command([fid,'inventory-action'],body,key=>client.requestInventoryAction(fid,{...body,idempotencyKey:key})),
  wearingHistory:(fid:string,cursor='')=>client.inventoryWearingHistory(fid,cursor),
  history:(fid:string,id:string,cursor='')=>client.inventoryHistory(fid,id,cursor),
  async current(fid:string){const page=await client.inventoryPage(fid,{limit:1});if(!Number.isSafeInteger(page.revision)||page.revision<0)throw new Error('衣柜当前版本未完整返回');return {revision:page.revision};},
  create:(fid:string,body:Omit<BrowserInventoryCreateBody,'idempotencyKey'>)=>command([fid,'inventory-add'],body,key=>client.createInventoryItem(fid,{...body,idempotencyKey:key})),
  update:(fid:string,id:string,body:Omit<BrowserInventoryUpdateBody,'idempotencyKey'>)=>command([fid,id,'inventory-update'],body,key=>client.updateInventoryItem(fid,id,{...body,idempotencyKey:key})),
  archive:(fid:string,id:string,body:Omit<BrowserInventoryArchiveBody,'idempotencyKey'>)=>command([fid,id,'inventory-archive'],body,key=>client.archiveInventoryItem(fid,id,{...body,idempotencyKey:key})),
 };
}
export function installInventoryProvider(client:BrowserClient){const inventory=createInventoryProvider(client);configureDomainService('inventory',inventory);return inventory;}
