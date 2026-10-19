import {ownerPathContracts} from './owner-contract-utils.mjs';
const text={type:'string'},data={type:'object',additionalProperties:true},time={type:'string',format:'date-time'},version={type:'integer',minimum:1,maximum:2147483647},ref=name=>({$ref:'#/components/schemas/'+name});
const object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false});
const common={reason:{type:'string',minLength:1,maxLength:500},idempotencyKey:{type:'string',minLength:1,maxLength:256}};
export const promiseSchemas={
 BrowserPromiseProposalBody:object({title:{type:'string',minLength:1,maxLength:2000},dueAt:time,startsAt:time,...common},['title','dueAt',...Object.keys(common)]),
 BrowserPromiseCommandBody:object({expectedRevision:version,...common}),
 BrowserPromiseRescheduleBody:object({expectedRevision:version,dueAt:time,startsAt:time,...common},['expectedRevision','dueAt',...Object.keys(common)]),
 BrowserPromise:{...object({request_id:text,fluctlight_id:text,title:text,revision:version,profile_id:text,timezone:text,window_start:time,window_end:time,lifecycle:{type:'string',enum:['pending','processing','accepted','rejected','deferred','failed','awaiting_goal','fulfilled','missed','cancelled','paused','blocked','legacy_unlinked']},fulfillment_authority:{type:'string',enum:['goal_commitment_evaluation']},legacy_unlinked:{type:'boolean'},goal_id:{anyOf:[text,{type:'null'}]},commitment_id:{anyOf:[text,{type:'null'}]},intention_id:{anyOf:[text,{type:'null'}]},request:ref('BrowserOwnerRequest'),commitment:{anyOf:[data,{type:'null'}]},intention:{anyOf:[data,{type:'null'}]},evaluations:{type:'array',items:data},planning:{type:'array',items:data}},['request_id','fluctlight_id','title','lifecycle','request','fulfillment_authority']),additionalProperties:true},
 BrowserPromisePage:object({items:{type:'array',items:ref('BrowserPromise')},next_cursor:text}),
 BrowserPromiseHistory:object({items:{type:'array',items:data},next_cursor:text}),
 BrowserPromiseCommandResult:object({promise:data,audit_id:text,operation:text}),
 BrowserPromiseRequestResult:object({request:ref('BrowserOwnerRequest'),audit_id:text}),
};
const response=(name,status=200)=>({responses:{[status]:{description:'Actual promise request or evidence-based lifecycle projection',content:{'application/json':{schema:ref(name)}}}}});
const body=name=>({requestBody:{required:true,content:{'application/json':{schema:ref(name)}}}});
const paging=[{name:'cursor',in:'query',schema:text},{name:'limit',in:'query',schema:{type:'integer',minimum:1,maximum:100}}];
export const promisePaths=ownerPathContracts({
 '/api/fluctlights/{fluctlightId}/promises':{get:{operationId:'listPromises',parameters:paging,...response('BrowserPromisePage')},post:{operationId:'createPromise',...body('BrowserPromiseProposalBody'),...response('BrowserPromiseRequestResult',202)}},
 '/api/fluctlights/{fluctlightId}/promises/{promiseId}':{get:{operationId:'readPromise',...response('BrowserPromise')}},
 '/api/fluctlights/{fluctlightId}/promises/{promiseId}/history':{get:{operationId:'promiseHistory',parameters:paging,...response('BrowserPromiseHistory')}},
 '/api/fluctlights/{fluctlightId}/promises/{promiseId}/reschedule':{post:{operationId:'reschedulePromise',...body('BrowserPromiseRescheduleBody'),...response('BrowserPromiseRequestResult',202)}},
 '/api/fluctlights/{fluctlightId}/promises/{promiseId}/cancel':{post:{operationId:'cancelPromise',...body('BrowserPromiseCommandBody'),...response('BrowserPromiseCommandResult')}},
 '/api/fluctlights/{fluctlightId}/promises/{promiseId}/retry':{post:{operationId:'retryPromise',...body('BrowserPromiseCommandBody'),...response('BrowserPromiseCommandResult')}},
});
