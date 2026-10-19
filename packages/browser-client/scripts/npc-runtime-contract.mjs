import {ownerPathContracts} from './owner-contract-utils.mjs';
const text={type:'string'},data={type:'object',additionalProperties:true},ref=name=>({$ref:'#/components/schemas/'+name}),nullable=value=>({anyOf:[value,{type:'null'}]});
const object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false});
export const npcRuntimeSchemas={
 BrowserNPCPresence:{...object({id:text,world_id:text,npc_id:text,actor_id:text,place_id:text,definition_revision:{type:'integer',minimum:1},source_kind:{type:'string',enum:['npc_routine_execution','world_event_execution']},source_ref:text,planned_start:text,planned_end:text,start_at:text,end_at:text,ended_at:nullable(text),status:{type:'string',enum:['active','ended','cancelled']},activity:text,created_at:text}),additionalProperties:true},
 BrowserNPCRecord:{...object({id:text,world_id:text,npc_id:text,record_kind:{type:'string',enum:['encounters','presences','opportunities']},status:text,source_kind:text,source_ref:text,source_fact_id:nullable(text),task_id:nullable(text),error_code:nullable(text),revision:{type:'integer',minimum:1},summary:text,result:data,evidence_refs:{type:'array',items:text},life_event_id:text,relationship_id:nullable(text),memory_id:nullable(text),opportunity_id:nullable(text),created_at:text,occurred_at:text,start_at:text,end_at:text,expires_at:text,available_at:text},['id','world_id','npc_id','record_kind']),additionalProperties:true},
 BrowserNPCRecordPage:object({items:{type:'array',items:ref('BrowserNPCRecord')},next_cursor:text}),
 BrowserNPCOpportunityRetryBody:object({expectedRevision:{type:'integer',minimum:1,maximum:2147483647},reason:{type:'string',minLength:1,maxLength:500},idempotencyKey:{type:'string',minLength:1,maxLength:256}}),
 BrowserNPCOpportunityRetryResult:object({id:text,revision:{type:'integer',minimum:1},status:{type:'string',enum:['pending']},source:{type:'string',enum:['owner_requested_reassessment']},acquaintance_created:{type:'boolean',const:false},audit_id:text}),
};
const base='/api/fluctlights/{fluctlightId}/npcs/{npcId}';
const response=name=>({responses:{200:{description:'Authoritative runtime records; reading never creates an encounter',content:{'application/json':{schema:ref(name)}}}}});
const paging=[{name:'cursor',in:'query',schema:text},{name:'limit',in:'query',schema:{type:'integer',minimum:1,maximum:100}}];
export const npcRuntimePaths=ownerPathContracts({
 [`${base}/opportunities/{opportunityId}`]:{get:{operationId:"npcOpportunity",...response("BrowserNPCRecord")}},
 ...Object.fromEntries(['encounters','presences','opportunities'].map(kind=>[`${base}/${kind}`,{get:{operationId:'npc'+kind[0].toUpperCase()+kind.slice(1),parameters:paging,...response('BrowserNPCRecordPage')}}])),
 [`${base}/opportunities/{opportunityId}/retry`]:{post:{operationId:'retryNPCOpportunity',requestBody:{required:true,content:{'application/json':{schema:ref('BrowserNPCOpportunityRetryBody')}}},...response('BrowserNPCOpportunityRetryResult')}},
});
