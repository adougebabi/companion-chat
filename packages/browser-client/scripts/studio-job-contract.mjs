import {ownerPathContracts} from './owner-contract-utils.mjs';
const text={type:'string'},data={type:'object',additionalProperties:true},integer={type:'integer',minimum:1},ref=n=>({$ref:'#/components/schemas/'+n}),array=items=>({type:'array',items});
const object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false});
const nullable=v=>({anyOf:[v,{type:'null'}]});
const command={expectedRevision:integer,reason:{type:'string',minLength:1,maxLength:1000},idempotencyKey:{type:'string',minLength:1,maxLength:256}};
export const studioJobSchemas={
 BrowserStudioReferenceAssetBody:{...object({assetId:{type:'string',minLength:1,maxLength:128},purpose:{type:'string',enum:['scene','animal','object','person']},grantId:{type:'string',minLength:1,maxLength:128},grantRevision:{type:'integer',minimum:1}},['assetId','purpose']),allOf:[{if:{required:['grantId']},then:{required:['grantRevision']}},{if:{required:['grantRevision']},then:{required:['grantId']}},{if:{properties:{purpose:{const:'person'}}},then:{required:['grantId','grantRevision']}}]},
 BrowserStudioMediaWorkflow:object({intent_id:text,workflow_id:text,status:text,error_code:nullable(text)}),
 BrowserStudioMediaRequest:object({id:text,status:text,workflow_id:text,provider_job_id:nullable(text),quality_verdict:nullable(text),workflows:array(ref('BrowserStudioMediaWorkflow'))}),
 BrowserStudioJobBody:{...object({resourceId:{type:'string',minLength:1,maxLength:128},resourceRevision:integer,kind:{type:'string',enum:['assistant','text_preview','image_preview']},name:{type:'string',minLength:1,maxLength:200},body:data,instruction:{type:'string',minLength:1,maxLength:4000},reason:command.reason,idempotencyKey:command.idempotencyKey,referenceAsset:ref('BrowserStudioReferenceAssetBody'),purpose:{type:'string',enum:['foundation_prepare']},resourceVersion:{type:'string',minLength:1,maxLength:64}},['resourceId','resourceRevision','kind','name','body','instruction','reason','idempotencyKey']),allOf:[{if:{required:['purpose']},then:{required:['resourceVersion'],properties:{kind:{const:'assistant'}},not:{required:['referenceAsset']}}},{if:{required:['resourceVersion']},then:{required:['purpose']}},{if:{required:['referenceAsset']},then:{properties:{kind:{const:'image_preview'}}}}]},
 BrowserStudioJobCommandBody:object(command),
 BrowserStudioJobAttempt:{...object({id:text,job_id:text,attempt:integer,status:text,output:data,error_code:nullable(text),provenance:data,intent_id:nullable(text),intent_status:nullable(text),workflow_id:nullable(text),intent_error:nullable(text),created_at:text,finished_at:nullable(text)},['id','job_id','attempt','status','output','provenance','created_at']),additionalProperties:true},
 BrowserStudioJob:{...object({id:text,owner_actor_id:text,resource_id:text,resource_revision:integer,kind:{type:'string',enum:['assistant','text_preview','image_preview']},snapshot:data,source_digest:text,status:{type:'string',enum:['queued','running','completed','failed','discarded']},revision:integer,latest_attempt:integer,result:data,error_code:nullable(text),created_at:text,updated_at:text,attempts:array(ref('BrowserStudioJobAttempt')),media_requests:array(ref('BrowserStudioMediaRequest')),label:text,life_write:{type:'boolean',const:false}},['id','owner_actor_id','resource_id','resource_revision','kind','snapshot','source_digest','status','revision','latest_attempt','result','created_at','updated_at']),additionalProperties:true},
 BrowserStudioJobPage:object({items:array(ref('BrowserStudioJob')),next_cursor:text}),
 BrowserStudioMaterial:object({id:text,owner_actor_id:text,resource_id:text,job_id:text,source_digest:text,text:nullable(text),asset_id:nullable(text),label:text,revision:integer,released_at:nullable(text),created_at:text}),
 BrowserStudioMaterialHistory:object({items:array(data),next_cursor:text}),
 BrowserStudioMaterialPage:object({items:array(ref('BrowserStudioMaterial')),next_cursor:text}),
 BrowserStudioMaterialReceipt:{...object({id:text,job_id:text,resource_id:text,status:{type:'string',const:'saved'},life_write:{type:'boolean',const:false},asset_id:text},['id','job_id','resource_id','status','life_write']),additionalProperties:true},
};
const response=n=>({responses:{200:{description:'Actual durable Studio state; acceptance does not mean generation completed',content:{'application/json':{schema:ref(n)}}}}});
const body=n=>({requestBody:{required:true,content:{'application/json':{schema:ref(n)}}}});
const page=[{name:'resource_id',in:'query',schema:text},{name:'cursor',in:'query',schema:text},{name:'limit',in:'query',schema:{type:'integer',minimum:1,maximum:100}}];
export const studioJobPaths=ownerPathContracts({
 '/api/studio/jobs':{get:{operationId:'listStudioJobs',parameters:page,...response('BrowserStudioJobPage')},post:{operationId:'createStudioJob',...body('BrowserStudioJobBody'),...response('BrowserStudioJob')}},
 '/api/studio/jobs/{jobId}':{get:{operationId:'studioJob',...response('BrowserStudioJob')}},
 '/api/studio/jobs/{jobId}/retry':{post:{operationId:'retryStudioJob',...body('BrowserStudioJobCommandBody'),...response('BrowserStudioJob')}},
 '/api/studio/jobs/{jobId}/discard':{post:{operationId:'discardStudioJob',...body('BrowserStudioJobCommandBody'),...response('BrowserStudioJob')}},
 '/api/studio/jobs/{jobId}/save-material':{post:{operationId:'saveStudioMaterial',...body('BrowserStudioJobCommandBody'),...response('BrowserStudioMaterialReceipt')}},
 '/api/studio/materials':{get:{operationId:'listStudioMaterials',parameters:[...page,{name:'released',in:'query',schema:{type:'boolean'}}],...response('BrowserStudioMaterialPage')}},
 '/api/studio/materials/{materialId}':{get:{operationId:'studioMaterial',...response('BrowserStudioMaterial')}},
 '/api/studio/materials/{materialId}/history':{get:{operationId:'studioMaterialHistory',parameters:page.filter(p=>p.name!=='resource_id'),...response('BrowserStudioMaterialHistory')}},
 '/api/studio/materials/{materialId}/release':{post:{operationId:'releaseStudioMaterial',...body('BrowserStudioJobCommandBody'),...response('BrowserStudioMaterial')}},
 '/api/studio/materials/{materialId}/retain':{post:{operationId:'retainStudioMaterial',...body('BrowserStudioJobCommandBody'),...response('BrowserStudioMaterial')}},
});
