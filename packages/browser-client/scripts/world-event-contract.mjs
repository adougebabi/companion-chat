import {ownerPathContracts} from './owner-contract-utils.mjs';
const text={type:'string'},data={type:'object',additionalProperties:true},integer={type:'integer',minimum:1},nullable=v=>({anyOf:[v,{type:'null'}]}),ref=n=>({$ref:'#/components/schemas/'+n}),array=items=>({type:'array',items});
const object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false});
const command={expectedRevision:integer,reason:{type:'string',minLength:1,maxLength:1000},idempotencyKey:{type:'string',minLength:1,maxLength:256}};
export const worldEventSchemas={
 BrowserWorldLifeEvent:{...object({id:text,fluctlight_id:nullable(text),world_id:nullable(text),title:text,kind:text,status:text,context_scope:{type:'string',enum:['life','ambient']},start_at:text,end_at:text,revision:integer,result:data,evidence_refs:array(text),impacts:array(data),more_memory_impacts:{type:'boolean'},impact_scope:text},['id','fluctlight_id','world_id','title','kind','status','context_scope','start_at','end_at','revision','result']),additionalProperties:true},
 BrowserWorldLifeEventPage:object({items:array(ref('BrowserWorldLifeEvent')),next_cursor:text,world_id:text,includes_unattributed_history:{type:'boolean'}}),
 BrowserWorldEventTemplate:{...object({id:text,template_id:text,world_id:text,name:text,category:text,definition:data,source_resource_id:nullable(text),source_version:nullable(text),generation_id:nullable(text),enabled:{type:'boolean'},revision:integer,permanent_harm:{type:'boolean',const:false},locked:{type:'boolean',const:false},next_check_at:text,last_triggered_at:nullable(text),last_checked_at:nullable(text),error_code:nullable(text)},['id','world_id','name','definition','source_resource_id','source_version','enabled','revision']),additionalProperties:true},
 BrowserWorldEventRun:{...object({id:text,world_id:text,template_id:text,fluctlight_id:nullable(text),owner_actor_id:text,template_revision:integer,definition:data,trigger_kind:{type:'string',enum:['manual','automatic']},status:{type:'string',enum:['queued','applied','filtered','failed']},source_key:text,due_at:text,expires_at:text,started_at:nullable(text),end_at:nullable(text),attempt_count:{type:'integer',minimum:0},revision:integer,life_event_id:nullable(text),result:data,error_code:nullable(text),recovery_status:{type:'string',enum:['not_required','pending','completed','failed']},recovery_result:data,created_at:text,updated_at:text}),additionalProperties:true},
 BrowserWorldEventPage:object({items:array(ref('BrowserWorldEventRun')),next_cursor:text}),
 BrowserWorldEventTriggerBody:object({...command,scheduledAt:{type:'string',format:'date-time'}},Object.keys(command)),
 BrowserWorldEventRetryBody:object({...command,expectedRunRevision:integer}),
 BrowserWorldEventCommandResult:{...object({id:text,status:text,revision:integer,audit_id:text}),additionalProperties:true},
};
const base='/api/fluctlights/{fluctlightId}/world/templates/{templateId}',response=n=>({responses:{200:{description:'Actual world execution or explicitly queued command; no implied success',content:{'application/json':{schema:ref(n)}}}}}),body=n=>({requestBody:{required:true,content:{'application/json':{schema:ref(n)}}}});
export const worldEventPaths=ownerPathContracts({
 '/api/fluctlights/{fluctlightId}/world/events':{get:{operationId:'worldLifeEvents',parameters:[{name:'cursor',in:'query',schema:text}],...response('BrowserWorldLifeEventPage')}},
 '/api/fluctlights/{fluctlightId}/world/events/{eventId}':{get:{operationId:'worldLifeEvent',...response('BrowserWorldLifeEvent')}},
 [base]:{get:{operationId:'worldEventTemplate',...response('BrowserWorldEventTemplate')}},
 [`${base}/runs`]:{get:{operationId:'worldEventRuns',parameters:[{name:'cursor',in:'query',schema:text},{name:'limit',in:'query',schema:{type:'integer',minimum:1,maximum:100}}],...response('BrowserWorldEventPage')},post:{operationId:'triggerWorldEvent',...body('BrowserWorldEventTriggerBody'),...response('BrowserWorldEventRun')}},
 [`${base}/runs/{runId}`]:{get:{operationId:'worldEventRun',...response('BrowserWorldEventRun')}},
 [`${base}/runs/{runId}/retry`]:{post:{operationId:'retryWorldEvent',...body('BrowserWorldEventRetryBody'),...response('BrowserWorldEventCommandResult')}},
});
