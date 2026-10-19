import {ownerPathContracts} from './owner-contract-utils.mjs';
const text={type:'string'},data={type:'object',additionalProperties:true},integer={type:'integer',minimum:1},ref=n=>({$ref:'#/components/schemas/'+n}),object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false});
export const worldItemSchemas={
 BrowserWorldItem:{...object({id:text,fluctlight_id:text,world_id:text,source_resource_id:text,source_version:text,name:text,definition:data,available:{type:'boolean'},place_id:{anyOf:[text,{type:'null'}]},revision:integer,local_overrides:data,created_at:text,updated_at:text}),additionalProperties:true},
 BrowserWorldItemPage:object({items:{type:'array',items:ref('BrowserWorldItem')},next_cursor:text,world_id:text}),
 BrowserWorldItemConfigBody:object({expectedRevision:integer,available:{type:'boolean'},placeId:{anyOf:[{type:'string',minLength:1,maxLength:128},{type:'null'}]},reason:{type:'string',minLength:1,maxLength:1000},idempotencyKey:{type:'string',minLength:1,maxLength:256}}),
};
const response=name=>({responses:{200:{description:'Actual catalogue state; registration does not grant inventory',content:{'application/json':{schema:ref(name)}}}}});
const base='/api/fluctlights/{fluctlightId}/world/items';
export const worldItemPaths=ownerPathContracts({
 [base]:{get:{operationId:'worldItems',parameters:[{name:'q',in:'query',schema:{type:'string',maxLength:200}},{name:'cursor',in:'query',schema:text},{name:'limit',in:'query',schema:{type:'integer',minimum:1,maximum:100}}],...response('BrowserWorldItemPage')}},
 [base+'/{itemId}']:{get:{operationId:'worldItem',...response('BrowserWorldItem')},put:{operationId:'configureWorldItem',requestBody:{required:true,content:{'application/json':{schema:ref('BrowserWorldItemConfigBody')}}},...response('BrowserWorldItem')}},
});
