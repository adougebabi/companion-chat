import {ownerPathContracts} from './owner-contract-utils.mjs';
const text={type:'string'},data={type:'object',additionalProperties:true},version={type:'integer',minimum:0,maximum:2147483647},ref=name=>({$ref:'#/components/schemas/'+name});
const object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false}),nullable=value=>({anyOf:[value,{type:'null'}]});
const fields={itemKind:{type:'string',enum:['wearable','object']},category:{type:'string',minLength:1,maxLength:64},slot:{type:'string',maxLength:64},description:{type:'string',minLength:1,maxLength:512},ownership:{type:'string',enum:['owned','borrowed','unknown']},availability:{type:'string',enum:['available','unavailable','lost']}};
const strings={type:'array',items:{type:'string'},maxItems:32};
const seasons={type:'array',uniqueItems:true,maxItems:4,items:{type:'string',enum:['spring','summer','autumn','winter']}};
const clothingType={type:'string',enum:['outer','upper_body','underwear','panties','lower_body','socks','shoes','accessory']};
const common={expectedRevision:version,reason:{type:'string',minLength:1,maxLength:1024},idempotencyKey:{type:'string',minLength:1,maxLength:256}};
export const inventorySchemas={
 BrowserInventoryPreparation:object({status:text,fluctlight_id:text,can_recover:{type:'boolean'},id:text,task_id:text,profile_id:text,result:data,error_code:nullable(text),created_at:text,updated_at:text},["status","fluctlight_id"]),
 BrowserInventoryPreparationBody:object(common),
 BrowserInventoryBorrowItemBody:object(Object.fromEntries(['itemKind','category','slot','description'].map(key=>[key,fields[key]]))),
 BrowserInventoryActionBody:{anyOf:[
  object({...common,reason:{type:'string',minLength:1,maxLength:500},kind:{type:'string',enum:['borrow']},lender:{type:'string',minLength:1,maxLength:256},items:{type:'array',minItems:1,maxItems:8,items:ref('BrowserInventoryBorrowItemBody')}}),
  object({...common,reason:{type:'string',minLength:1,maxLength:500},kind:{type:'string',enum:['return']},itemIds:{type:'array',minItems:1,maxItems:8,uniqueItems:true,items:{type:'string',minLength:1,maxLength:128}}}),
  object({...common,reason:{type:'string',minLength:1,maxLength:500},kind:{type:'string',enum:['remove']},slots:{type:'array',minItems:1,maxItems:16,uniqueItems:true,items:{type:'string',minLength:1,maxLength:64}}}),
  object({...common,reason:{type:'string',minLength:1,maxLength:500},kind:{type:'string',enum:['outfit']},outfitId:{type:'string',minLength:1,maxLength:128},expectedOutfitRevision:{...version,minimum:1},profileId:{type:'string',minLength:1,maxLength:128}}),
 ]},
 BrowserInventoryActionResult:object({request:ref('BrowserOwnerRequest'),audit_id:text}),
 BrowserInventoryTags:object({tags:strings,revision:version}),
 BrowserInventoryTagsBody:object({...common,tags:strings,renames:{type:'object',additionalProperties:{type:'string'}}},[...Object.keys(common),'tags']),
 BrowserInventoryOutfit:object({tags:strings,id:text,fluctlight_id:text,profile_id:text,name:text,source_resource_id:nullable(text),source_version:nullable(text),source_overrides:data,revision:{...version,minimum:1},created_at:text,updated_at:text,items:{type:'array',items:data},active_profile_id:text},['id','fluctlight_id','profile_id','name','revision','created_at','updated_at','items']),
 BrowserInventoryOutfitPage:object({items:{type:'array',items:ref('BrowserInventoryOutfit')},next_cursor:text,profile_id:text}),
 BrowserInventoryOutfitWriteBody:object({...common,tags:strings,name:{type:'string',minLength:1,maxLength:128},itemIds:{type:'array',minItems:1,maxItems:16,uniqueItems:true,items:{type:'string',minLength:1,maxLength:128}},profileId:{type:'string',minLength:1,maxLength:128},expectedOutfitRevision:version},[...Object.keys(common),'name','itemIds','profileId','expectedOutfitRevision']),
 BrowserInventoryOutfitRenameBody:object({...common,name:{type:'string',minLength:1,maxLength:128},profileId:{type:'string',minLength:1,maxLength:128},expectedOutfitRevision:{...version,minimum:1}}),
 BrowserInventoryOutfitWriteResult:object({outfit:ref('BrowserInventoryOutfit'),revision:version,audit_id:text}),
 BrowserInventoryItemBody:object({...fields,clothingType,seasons},Object.keys(fields)),
 BrowserInventoryItem:{...object({clothing_type:nullable(clothingType),seasons,id:text,fluctlight_id:text,item_kind:fields.itemKind,category:text,slot:text,description:text,ownership:fields.ownership,availability:fields.availability,source_kind:text,source_ref:text,source_item_key:text,source_resource_id:nullable(text),source_version:nullable(text),source_world_item_id:nullable(text),source_world_item_revision:nullable({...version,minimum:1}),source_world_item_snapshot:nullable(data),revision:{...version,minimum:1},acquired_at:text,updated_at:text,archived_at:nullable(text),source_verified:{type:'boolean'},source_event:nullable(data),worn:nullable(data),use:nullable(data),media:{type:'array',items:object({asset_id:text,created_at:text})}}),additionalProperties:true},
 BrowserInventoryPage:object({items:{type:'array',items:ref('BrowserInventoryItem')},next_cursor:text,revision:version,inventory_complete:{type:'boolean'}}),
 BrowserInventoryCreateBody:object({...common,item:ref('BrowserInventoryItemBody')}),
 BrowserInventoryUpdateBody:object({...common,expectedItemRevision:{...version,minimum:1},item:ref('BrowserInventoryItemBody')}),
 BrowserInventoryArchiveBody:object({...common,expectedItemRevision:{...version,minimum:1}}),
 BrowserInventoryResult:object({item:ref('BrowserInventoryItem'),revision:version,audit_id:text,operation:{type:'string',enum:['add','update','archive']}}),
 BrowserInventoryHistory:object({items:{type:'array',items:data},next_cursor:text}),
};
// Metadata is additive for pre-0096 clients and recorded transport fixtures.
inventorySchemas.BrowserInventoryItem.required=inventorySchemas.BrowserInventoryItem.required.filter(key=>!['clothing_type','seasons'].includes(key));
const response=name=>({responses:{200:{description:'Actual inventory authority or immutable history',content:{'application/json':{schema:ref(name)}}}}});
const body=name=>({requestBody:{required:true,content:{'application/json':{schema:ref(name)}}}});
const paging=[{name:'cursor',in:'query',schema:text},{name:'limit',in:'query',schema:{type:'integer',minimum:1,maximum:100}}];
export const inventoryPaths=ownerPathContracts({
 '/api/fluctlights/{fluctlightId}/inventory/preparation':{get:{operationId:'inventoryPreparation',...response('BrowserInventoryPreparation')},post:{operationId:'prepareInventory',...body('BrowserInventoryPreparationBody'),...response('BrowserInventoryPreparation')}},
 '/api/fluctlights/{fluctlightId}/inventory/tags':{get:{operationId:'inventoryTags',...response('BrowserInventoryTags')},put:{operationId:'saveInventoryTags',...body('BrowserInventoryTagsBody'),...response('BrowserInventoryTags')}},
 '/api/fluctlights/{fluctlightId}/inventory/requests':{post:{operationId:'requestInventoryAction',...body('BrowserInventoryActionBody'),responses:{202:{description:'Owner proposal persisted with actual cognition workflow; no loan/wear effect yet',content:{'application/json':{schema:ref('BrowserInventoryActionResult')}}}}}},
 '/api/fluctlights/{fluctlightId}/inventory/outfits':{get:{operationId:'inventoryOutfits',parameters:paging,...response('BrowserInventoryOutfitPage')},post:{operationId:'createInventoryOutfit',...body('BrowserInventoryOutfitWriteBody'),...response('BrowserInventoryOutfitWriteResult')}},
 '/api/fluctlights/{fluctlightId}/inventory/outfits/{outfitId}':{get:{operationId:'inventoryOutfit',...response('BrowserInventoryOutfit')},patch:{operationId:'renameInventoryOutfit',...body('BrowserInventoryOutfitRenameBody'),...response('BrowserInventoryOutfitWriteResult')},put:{operationId:'updateInventoryOutfit',...body('BrowserInventoryOutfitWriteBody'),...response('BrowserInventoryOutfitWriteResult')}},
 '/api/fluctlights/{fluctlightId}/inventory/outfits/{outfitId}/revisions':{get:{operationId:'inventoryOutfitRevisions',parameters:paging,...response('BrowserInventoryHistory')}},
 '/api/fluctlights/{fluctlightId}/inventory/wearing-history':{get:{operationId:'inventoryWearingHistory',parameters:paging,...response('BrowserInventoryHistory')}},
 '/api/fluctlights/{fluctlightId}/inventory':{get:{operationId:'inventoryPage',parameters:[...paging,...['q','kind','ownership','availability','state'].map(name=>({name,in:'query',schema:text}))],...response('BrowserInventoryPage')}},
 '/api/fluctlights/{fluctlightId}/inventory/items':{post:{operationId:'createInventoryItem',...body('BrowserInventoryCreateBody'),...response('BrowserInventoryResult')}},
 '/api/fluctlights/{fluctlightId}/inventory/items/{itemId}':{get:{operationId:'inventoryItem',...response('BrowserInventoryItem')},put:{operationId:'updateInventoryItem',...body('BrowserInventoryUpdateBody'),...response('BrowserInventoryResult')},delete:{operationId:'archiveInventoryItem',...body('BrowserInventoryArchiveBody'),...response('BrowserInventoryResult')}},
 '/api/fluctlights/{fluctlightId}/inventory/items/{itemId}/history':{get:{operationId:'inventoryHistory',parameters:paging,...response('BrowserInventoryHistory')}},
});
