import {ownerPathContracts} from './owner-contract-utils.mjs';
const text={type:'string'},data={type:'object',additionalProperties:true},ref=n=>({$ref:'#/components/schemas/'+n}),object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false});
export const placeDefaultSchemas={
 BrowserPlaceDefaultView:object({place_id:text,world_id:text,place_revision:{type:'integer',minimum:1},default_revision:{type:'integer',minimum:0},baseline:{anyOf:[data,{type:'null'}]},diff:{type:'array',items:object({field:text,from:{},to:{}})},source:text,current:data,semantics:text}),
 BrowserPlaceDefaultBody:object({worldId:text,expectedRevision:{type:'integer',minimum:1},expectedDefaultRevision:{type:'integer',minimum:0},operation:{type:'string',enum:['capture','restore']},source:{type:'string',enum:['recorded','current_saved','current_definition']},reason:{type:'string',minLength:1,maxLength:1000},idempotencyKey:{type:'string',minLength:1,maxLength:256}}),
 BrowserPlaceDefaultHistory:object({items:{type:'array',items:data},next_cursor:{anyOf:[text,{type:'null'}]}},['items','next_cursor']),
};
const base='/api/fluctlights/{fluctlightId}/world/places/{placeId}/defaults',response=n=>({responses:{200:{description:'Actual place definition baseline or persisted history',content:{'application/json':{schema:ref(n)}}}}});
export const placeDefaultPaths=ownerPathContracts({
 [base]:{get:{operationId:'placeDefaults',parameters:['worldId','source'].map(name=>({name,in:'query',schema:text})),...response('BrowserPlaceDefaultView')},post:{operationId:'savePlaceDefaults',requestBody:{required:true,content:{'application/json':{schema:ref('BrowserPlaceDefaultBody')}}},responses:{200:{description:'Persisted baseline or exact field restoration receipt',content:{'application/json':{schema:data}}}}}},
 [base+'/history']:{get:{operationId:'placeDefaultHistory',parameters:['worldId','cursor'].map(name=>({name,in:'query',schema:text})),...response('BrowserPlaceDefaultHistory')}},
});
