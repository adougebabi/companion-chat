import {ownerPathContracts} from './owner-contract-utils.mjs';
const text={type:'string'},version={type:'integer',minimum:0,maximum:2147483647},field={type:'string',maxLength:1000},data={type:'object',additionalProperties:true};
const ref=name=>({$ref:'#/components/schemas/'+name}),object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false});
const reason={type:'string',minLength:1,maxLength:1024},key={type:'string',minLength:1,maxLength:256};
export const visualBriefSchemas={
 BrowserVisualBrief:object({fluctlight_id:text,revision:version,appearance:field,clothing:field,constraints:field,references:field,updated_by_actor_id:text,created_at:text,updated_at:text}),
 BrowserVisualBriefRead:object({fluctlight_id:text,revision:version,brief:{anyOf:[ref('BrowserVisualBrief'),{type:'null'}]},semantics:text,audit_id:text},['fluctlight_id','revision','brief']),
 BrowserVisualBriefBody:object({appearance:field,clothing:field,constraints:field,references:field,expectedRevision:version,reason,idempotencyKey:key}),
 BrowserVisualGenerateBody:object({expectedVisualRevision:version,expectedBriefRevision:version,reason,idempotencyKey:key,supersedeSessionId:{type:"string",minLength:1,maxLength:128}},["expectedVisualRevision","expectedBriefRevision","reason","idempotencyKey"]),
 BrowserVisualReviewBody:object({expectedAttempt:{type:'integer',minimum:1,maximum:2147483647},expectedVisualRevision:version,decision:{type:'string',enum:['accepted','regenerate']},identityMatch:{type:'number',minimum:0,maximum:1},confidence:{type:'number',minimum:0,maximum:1},observations:{type:'array',minItems:1,maxItems:24,items:{type:'string',minLength:1,maxLength:500}},missingSections:{type:'array',maxItems:16,items:{type:'string',minLength:1,maxLength:200}},summary:{type:'string',minLength:1,maxLength:1000},feedback:{type:'string',maxLength:2000},reason,idempotencyKey:key}),
 BrowserVisualReviewResult:object({fluctlight_id:text,session_id:text,operation_id:text,replayed:{type:'boolean'},result:data}),
 BrowserVisualGenerateResult:object({fluctlight_id:text,session_id:text,task_id:text,status:{type:'string',enum:['queued']},brief_revision:version,audit_id:text}),
 BrowserVisualReview:object({source:text,identity_match:{anyOf:[{type:'number'},{type:'null'}]},confidence:{anyOf:[{type:'number'},{type:'null'}]},observations:{anyOf:[{type:'array',items:text},{type:'null'}]},missing_sections:{anyOf:[{type:'array',items:text},{type:'null'}]},summary:{anyOf:[text,{type:'null'}]},feedback:{anyOf:[text,{type:'null'}]},reviewed_at:{anyOf:[text,{type:'null'}]},reason:{anyOf:[text,{type:'null'}]}}),
 BrowserVisualAttempt:object({id:text,attempt_number:{type:'integer',minimum:1},status:text,candidate_asset_id:{anyOf:[text,{type:'null'}]},decision:{anyOf:[text,{type:'null'}]},brief_revision:{anyOf:[version,{type:'null'}]},created_at:text,review:{anyOf:[ref('BrowserVisualReview'),{type:'null'}]}}),
 BrowserVisualSession:object({id:text,status:text,last_error:{anyOf:[text,{type:'null'}]},current_attempt:{type:'integer',minimum:1},created_at:text,task_id:{anyOf:[text,{type:'null'}]},attempts:{type:'array',items:ref('BrowserVisualAttempt')}}),
 BrowserVisualManagement:object({schema_version:text,id:text,fluctlight_id:text,status:text,current_revision:version,identity_snapshot:data,renderer_constraints:data,canonical_asset_id:text,character_sheet_asset_id:text,adapter_version:text,active_session_id:text,session:{anyOf:[ref('BrowserVisualSession'),{type:'null'}]}},['schema_version','fluctlight_id','status','current_revision','identity_snapshot','renderer_constraints','adapter_version','session']),
 BrowserVisualRecordPage:object({items:{type:'array',items:data},next_cursor:text}),
};
const response=(name,status=200)=>({responses:{[status]:{description:'Authorized visual reference or actual session state',content:{'application/json':{schema:ref(name)}}}}});
const body=name=>({requestBody:{required:true,content:{'application/json':{schema:ref(name)}}}});
const paging=[{name:'cursor',in:'query',schema:text},{name:'limit',in:'query',schema:{type:'integer',minimum:1,maximum:100}}];
export const visualBriefPaths=ownerPathContracts({
 '/api/fluctlights/{fluctlightId}/visual-sessions/{sessionId}/review':{post:{operationId:'reviewVisualIdentity',...body('BrowserVisualReviewBody'),...response('BrowserVisualReviewResult')}},
 '/api/fluctlights/{fluctlightId}/visual-brief':{get:{operationId:'visualBrief',...response('BrowserVisualBriefRead')},put:{operationId:'saveVisualBrief',...body('BrowserVisualBriefBody'),...response('BrowserVisualBriefRead')}},
 '/api/fluctlights/{fluctlightId}/visual-brief/history':{get:{operationId:'visualBriefHistory',parameters:paging,...response('BrowserVisualRecordPage')}},
 '/api/fluctlights/{fluctlightId}/visual-identity':{get:{operationId:'visualIdentityManagement',...response('BrowserVisualManagement')}},
 '/api/fluctlights/{fluctlightId}/visual-identity/timeline':{get:{operationId:'visualIdentityTimeline',parameters:paging,...response('BrowserVisualRecordPage')}},
 '/api/fluctlights/{fluctlightId}/visual-identity/generate':{post:{operationId:'generateVisualIdentity',...body('BrowserVisualGenerateBody'),...response('BrowserVisualGenerateResult',202)}},
});

Object.assign(visualBriefSchemas.BrowserVisualBrief.properties,{source_resource_id:{anyOf:[text,{type:'null'}]},source_version:{anyOf:[text,{type:'null'}]},source_overrides:data});
