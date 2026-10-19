import {ownerPathContracts} from './owner-contract-utils.mjs';
const text={type:'string'},integer={type:'integer',minimum:0},data={type:'object',additionalProperties:true};
const version={type:'integer',minimum:1,maximum:2147483647};
const commandFields={expectedRevision:version,reason:{type:'string',minLength:1,maxLength:1024},idempotencyKey:{type:'string',minLength:1,maxLength:256}};
const array=items=>({type:'array',items}),ref=name=>({$ref:'#/components/schemas/'+name}),nullable=value=>({anyOf:[value,{type:'null'}]});
const object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false});
const reply=name=>({responses:{200:{description:'Actual scoped claim or governance history',content:{'application/json':{schema:ref(name)}}}}});
const pagination=[{name:'cursor',in:'query',schema:{type:'string',maxLength:2048}},{name:'limit',in:'query',schema:{type:'integer',minimum:1,maximum:100}}];
export const selfSchemas={
 BrowserSelfReview:object({id:text,fluctlight_id:text,claim_id:text,claim_revision:integer,owner_actor_id:text,reason:text,disposition:{type:'string',enum:['acknowledged']},created_at:text}),
 BrowserSelfClaim:object({id:text,fluctlight_id:text,category:text,claim:text,value:{},confidence:{type:'number',minimum:0,maximum:1},evidence_refs:array(text),provenance:data,status:text,expires_at:nullable(text),revision:integer,superseded_by:nullable(text),created_at:text,updated_at:text,source_valid:{type:'boolean'},expired:{type:'boolean'},owner_review:nullable(ref('BrowserSelfReview'))}),
 BrowserSelfPage:object({items:array(ref('BrowserSelfClaim')),next_cursor:text}),
 BrowserSelfHistory:object({items:array(data),next_cursor:text}),
 BrowserSelfCommandBody:object(commandFields),
 BrowserSelfRestoreBody:object({...commandFields,targetRevision:version}),
 BrowserSelfCommandResult:object({id:text,fluctlight_id:text,revision:integer,status:text,operation:{type:'string',enum:['review','forget','restore']},target_revision:integer,review_id:text,audit_id:text,semantics:text},['id','fluctlight_id','revision','status','operation','audit_id']),
};
export const selfPaths=ownerPathContracts({
 '/api/fluctlights/{fluctlightId}/self-claims':{get:{operationId:'ownerSelfClaims',parameters:[...pagination,...['q','status','category'].map(name=>({name,in:'query',schema:text}))],...reply('BrowserSelfPage')}},
 '/api/fluctlights/{fluctlightId}/self-claims/{claimId}':{get:{operationId:'ownerSelfClaim',...reply('BrowserSelfClaim')}},
 ...Object.fromEntries(['revisions','reviews'].map(kind=>[`/api/fluctlights/{fluctlightId}/self-claims/{claimId}/${kind}`,{get:{operationId:'selfClaim'+kind,parameters:pagination,...reply('BrowserSelfHistory')}}])),
 ...Object.fromEntries(['review','forget','restore'].map(command=>[`/api/fluctlights/{fluctlightId}/self-claims/{claimId}/${command}`,{post:{operationId:'selfClaim'+command,requestBody:{required:true,content:{'application/json':{schema:ref(command==='restore'?'BrowserSelfRestoreBody':'BrowserSelfCommandBody')}}},...reply('BrowserSelfCommandResult')}}])),
});
