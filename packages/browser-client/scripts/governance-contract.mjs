import {ownerPathContracts} from './owner-contract-utils.mjs';
const text={type:'string'},data={type:'object',additionalProperties:true},version={type:'integer',minimum:0,maximum:2147483647};
const ref=name=>({$ref:'#/components/schemas/'+name}),array=items=>({type:'array',items});
const object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false});
const nullable=value=>({anyOf:[value,{type:'null'}]});
const idempotencyKey={type:'string',minLength:1,maxLength:256},reason={type:'string',minLength:1,maxLength:1024};
export const governanceSchemas={
 BrowserPersonaPreparationBody:object({expectedRevision:version,idempotencyKey}),
 BrowserPersonaPreparation:object({id:text,workflow_id:text,status:{type:'string',enum:['pending','running','ready','failed']},intent_status:text,revision:version,error_code:text,created_at:text,can_recover:{type:'boolean'}},['status','revision','can_recover']),
 BrowserFoundationDecisionBody:object({expectedRevision:version,reason}),
 BrowserFoundationRestoreBody:object({expectedRevision:version,targetRevision:version,reason}),
 BrowserFoundationDecisionResult:object({id:text,fluctlight_id:text,revision:version,status:{type:'string',enum:['accepted','rejected']},reason,replayed:{type:'boolean'},target_revision:version,persona_preparation:ref('BrowserPersonaPreparation')},['id','fluctlight_id','revision','status','reason','replayed']),
 BrowserFoundationProposalBody:object({changes:data,expectedRevision:version,reason,idempotencyKey}),
 BrowserFoundationRevision:object({id:text,fluctlight_id:text,revision:version,base_revision:version,source:text,status:{type:'string',enum:['proposed','accepted','rejected']},actor_id:text,reason:nullable(text),created_at:text,accepted_at:nullable(text),rejected_at:nullable(text),changes:data,core_persona:data,identity:data,personality:data,behavioral_policy:data,life_profile:data,provenance:data,evidence_refs:array({}),base_snapshot:nullable(data),source_resource_id:nullable(text),source_version:nullable(text),source_choices:array(data)}),
 BrowserFoundationPage:object({items:array(ref('BrowserFoundationRevision')),next_cursor:text}),
 BrowserGovernanceHistory:object({items:array(data),next_cursor:text}),
 BrowserCapabilityRequest:object({id:text,fluctlight_id:text,capability_key:text,title:text,description:text,rationale:text,desired_contract:data,side_effect_class:text,priority:text,source_fact_id:text,evidence_refs:array({}),status:{type:'string',enum:['proposed','reviewing','accepted','rejected','fulfilled','cancelled']},revision:{...version,minimum:1},review_note:nullable(text),reviewer_actor_id:nullable(text),capability_version:nullable(text),created_at:text,updated_at:text,registered_capability:object({available:{type:'boolean'},version:text},['available']),semantics:text},['id','fluctlight_id','capability_key','title','description','rationale','desired_contract','side_effect_class','priority','source_fact_id','evidence_refs','status','revision','review_note','reviewer_actor_id','capability_version','created_at','updated_at']),
 BrowserCapabilityPage:object({items:array(ref('BrowserCapabilityRequest')),next_cursor:text}),
 BrowserCapabilityReviewBody:object({status:{type:'string',enum:['reviewing','accepted','rejected','fulfilled','cancelled']},note:{type:'string',minLength:1,maxLength:2000},capabilityVersion:{type:'string',minLength:1,maxLength:128},expectedRevision:{...version,minimum:1},idempotencyKey},['status','note','expectedRevision','idempotencyKey']),
};
governanceSchemas.BrowserFoundationRevision.additionalProperties=true;
governanceSchemas.BrowserCapabilityRequest.additionalProperties=true;
const response=name=>({responses:{200:{description:'Scoped governance authority',content:{'application/json':{schema:ref(name)}}}}});
const pageParams=[{name:'cursor',in:'query',schema:text},{name:'limit',in:'query',schema:{type:'integer',minimum:1,maximum:100}}];
const body=name=>({requestBody:{required:true,content:{'application/json':{schema:ref(name)}}}});
export const governancePaths=ownerPathContracts({
 '/api/fluctlights/{fluctlightId}/persona-preparation/recover':{post:{operationId:'recoverPersonaPreparation',...body('BrowserPersonaPreparationBody'),...response('BrowserPersonaPreparation')}},
 '/api/fluctlights/{fluctlightId}/foundation-revisions':{get:{operationId:'foundationRevisionPage',parameters:[...pageParams,...['q','status'].map(name=>({name,in:'query',schema:text}))],...response('BrowserFoundationPage')},post:{operationId:'submitFoundationRevision',...body('BrowserFoundationProposalBody'),responses:{200:{description:'Durable proposal receipt',content:{'application/json':{schema:data}}}}}},
 ...Object.fromEntries(['accept','reject'].map(action=>['/api/fluctlights/{fluctlightId}/foundation-revisions/{revisionId}/'+action,{post:{operationId:action+'FoundationRevision',...body('BrowserFoundationDecisionBody'),...response('BrowserFoundationDecisionResult')}}])),
 '/api/fluctlights/{fluctlightId}/foundation-revisions/rollback':{post:{operationId:'rollbackFoundationRevision',...body('BrowserFoundationRestoreBody'),...response('BrowserFoundationDecisionResult')}},
 '/api/fluctlights/{fluctlightId}/foundation-revisions/{revisionId}':{get:{operationId:'foundationRevision',...response('BrowserFoundationRevision')}},
 '/api/fluctlights/{fluctlightId}/foundation-revisions/{revisionId}/history':{get:{operationId:'foundationGovernanceHistory',parameters:pageParams,...response('BrowserGovernanceHistory')}},
 '/api/fluctlights/{fluctlightId}/capability-requests':{get:{operationId:'capabilityRequestPage',parameters:[...pageParams,...['q','status'].map(name=>({name,in:'query',schema:text}))],...response('BrowserCapabilityPage')}},
 '/api/fluctlights/{fluctlightId}/capability-requests/{requestId}':{get:{operationId:'capabilityRequest',...response('BrowserCapabilityRequest')}},
 '/api/fluctlights/{fluctlightId}/capability-requests/{requestId}/history':{get:{operationId:'capabilityReviewHistory',parameters:pageParams,...response('BrowserGovernanceHistory')}},
 '/api/fluctlights/{fluctlightId}/capability-requests/{requestId}/review':{post:{operationId:'reviewScopedCapabilityRequest',...body('BrowserCapabilityReviewBody'),...response('BrowserCapabilityRequest')}},
 '/api/capability-requests/{requestId}/review':{post:{operationId:'reviewCapabilityRequest',...body('BrowserCapabilityReviewBody'),...response('BrowserCapabilityRequest')}},
});
