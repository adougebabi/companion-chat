import {ownerPathContracts} from './owner-contract-utils.mjs';
const text={type:'string'},integer={type:'integer',minimum:0},data={type:'object',additionalProperties:true};
const version={type:'integer',minimum:1,maximum:2147483647};
const commandFields={expectedRevision:version,reason:{type:'string',minLength:1,maxLength:1024},idempotencyKey:{type:'string',minLength:1,maxLength:256}};
const array=items=>({type:'array',items}),ref=name=>({$ref:'#/components/schemas/'+name}),nullable=value=>({anyOf:[value,{type:'null'}]});
const object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false});
const reply=(name,description='Actual scoped claim or governance history')=>({responses:{200:{description,content:{'application/json':{schema:ref(name)}}}}});
const expressionReply=name=>reply(name,'Scoped expression learning result');
const pagination=[{name:'cursor',in:'query',schema:{type:'string',maxLength:2048}},{name:'limit',in:'query',schema:{type:'integer',minimum:1,maximum:100}}];
export const selfSchemas={
 BrowserSelfReview:object({id:text,fluctlight_id:text,claim_id:text,claim_revision:integer,owner_actor_id:text,reason:text,disposition:{type:'string',enum:['acknowledged']},created_at:text}),
 BrowserSelfClaim:object({id:text,fluctlight_id:text,category:text,claim:text,value:{},confidence:{type:'number',minimum:0,maximum:1},evidence_refs:array(text),provenance:data,status:text,expires_at:nullable(text),revision:integer,superseded_by:nullable(text),created_at:text,updated_at:text,source_valid:{type:'boolean'},expired:{type:'boolean'},owner_review:nullable(ref('BrowserSelfReview'))}),
 BrowserSelfPage:object({items:array(ref('BrowserSelfClaim')),next_cursor:text}),
 BrowserSelfHistory:object({items:array(data),next_cursor:text}),
 BrowserSelfCommandBody:object(commandFields),
 BrowserSelfRestoreBody:object({...commandFields,targetRevision:version}),
 BrowserSelfCommandResult:object({id:text,fluctlight_id:text,revision:integer,status:text,operation:{type:'string',enum:['review','forget','restore']},target_revision:integer,review_id:text,audit_id:text,semantics:text},['id','fluctlight_id','revision','status','operation','audit_id']),
 BrowserExpressionSettings:object({learning_enabled:{type:'boolean'},use_enabled:{type:'boolean'},revision:integer,_replay:{type:'boolean'}},['learning_enabled','use_enabled','revision']),
 BrowserExpressionCandidate:object({id:text,source_message_id:text,source_conversation_id:text,source_actor_id:text,source_text:text,source_valid:{type:'boolean'},excerpt:text,situation:text,trigger_text:text,status:{type:'string',enum:['pending','enabled','disabled','revoked']},revision:version,reviews:array(data)}),
 BrowserExpressionLearning:object({settings:ref('BrowserExpressionSettings'),items:array(ref('BrowserExpressionCandidate'))}),
 BrowserExpressionSource:object({id:text,conversation_id:text,text}),
 BrowserExpressionSourceItem:object({id:text,conversation_id:text,text,created_at:text}),
 BrowserExpressionSourcePage:object({items:array(ref('BrowserExpressionSourceItem')),next_cursor:text}),
 BrowserExpressionSettingsBody:object({learningEnabled:{type:'boolean'},useEnabled:{type:'boolean'},expectedRevision:integer,idempotencyKey:commandFields.idempotencyKey}),
 BrowserExpressionExtractionBody:object({sourceMessageId:{type:'string',minLength:1,maxLength:128},idempotencyKey:commandFields.idempotencyKey}),
 BrowserExpressionExtractionResult:object({status:{type:'string',enum:['completed']},candidate_ids:array(text),_replay:{type:'boolean'}},['status','candidate_ids']),
 BrowserExpressionExtractionStatus:object({status:{type:'string',enum:['not_found','running','completed','failed']},no_effect:{type:'boolean'},result:data},['status','no_effect']),
 BrowserExpressionReviewBody:object({...commandFields,triggerText:nullable({type:'string',maxLength:256})},['expectedRevision','reason','idempotencyKey']),
 BrowserExpressionEnableBody:object({...commandFields,triggerText:{type:'string',minLength:1,maxLength:256}}),
 BrowserExpressionReviewResult:object({id:text,status:{type:'string',enum:['enabled','disabled','revoked']},revision:version,action:{type:'string',enum:['enable','disable','revoke']},_replay:{type:'boolean'}},['id','status','revision','action']),
};
export const selfPaths=ownerPathContracts({
 '/api/fluctlights/{fluctlightId}/self-claims':{get:{operationId:'ownerSelfClaims',parameters:[...pagination,...['q','status','category'].map(name=>({name,in:'query',schema:text}))],...reply('BrowserSelfPage')}},
 '/api/fluctlights/{fluctlightId}/self-claims/{claimId}':{get:{operationId:'ownerSelfClaim',...reply('BrowserSelfClaim')}},
 ...Object.fromEntries(['revisions','reviews'].map(kind=>[`/api/fluctlights/{fluctlightId}/self-claims/{claimId}/${kind}`,{get:{operationId:'selfClaim'+kind,parameters:pagination,...reply('BrowserSelfHistory')}}])),
 ...Object.fromEntries(['review','forget','restore'].map(command=>[`/api/fluctlights/{fluctlightId}/self-claims/{claimId}/${command}`,{post:{operationId:'selfClaim'+command,requestBody:{required:true,content:{'application/json':{schema:ref(command==='restore'?'BrowserSelfRestoreBody':'BrowserSelfCommandBody')}}},...reply('BrowserSelfCommandResult')}}])),
});
export const expressionPaths=ownerPathContracts({
 '/api/fluctlights/{fluctlightId}/expression-learning':{get:{operationId:'expressionLearning',...expressionReply('BrowserExpressionLearning')}},
 '/api/fluctlights/{fluctlightId}/expression-learning/sources':{get:{operationId:'expressionLearningSources',parameters:pagination,...expressionReply('BrowserExpressionSourcePage')}},
 '/api/fluctlights/{fluctlightId}/expression-learning/sources/{messageId}':{get:{operationId:'expressionLearningSource',...expressionReply('BrowserExpressionSource')}},
 '/api/fluctlights/{fluctlightId}/expression-learning/settings':{put:{operationId:'updateExpressionLearning',requestBody:{required:true,content:{'application/json':{schema:ref('BrowserExpressionSettingsBody')}}},...expressionReply('BrowserExpressionSettings')}},
 '/api/fluctlights/{fluctlightId}/expression-learning/extractions':{post:{operationId:'extractExpressionLearning',requestBody:{required:true,content:{'application/json':{schema:ref('BrowserExpressionExtractionBody')}}},...expressionReply('BrowserExpressionExtractionResult')}},
 '/api/fluctlights/{fluctlightId}/expression-learning/extractions/by-key':{get:{operationId:'expressionExtractionStatus',parameters:[{name:'key',in:'query',required:true,schema:{type:'string',minLength:1,maxLength:256}}],...expressionReply('BrowserExpressionExtractionStatus')}},
 ...Object.fromEntries(['enable','disable','revoke'].map(action=>[`/api/fluctlights/{fluctlightId}/expression-learning/candidates/{candidateId}/${action}`,{post:{operationId:'expressionLearning'+action,requestBody:{required:true,content:{'application/json':{schema:ref(action==='enable'?'BrowserExpressionEnableBody':'BrowserExpressionReviewBody')}}},...expressionReply('BrowserExpressionReviewResult')}}])),
});
