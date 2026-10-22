import {ownerPathContracts} from './owner-contract-utils.mjs';
const text={type:'string'},number={type:'integer',minimum:0},data={type:'object',additionalProperties:true};
const ref=name=>({$ref:'#/components/schemas/'+name}),array=items=>({type:'array',items}),nullable=value=>({anyOf:[value,{type:'null'}]});
const object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false});
const response=(name,status=200)=>({responses:{[status]:{description:'Authoritative diary state; acceptance is not generation completion',content:{'application/json':{schema:ref(name)}}}}});
const request=name=>({requestBody:{required:true,content:{'application/json':{schema:ref(name)}}}});
const key={type:'string',minLength:1,maxLength:256};
const paging=[{name:'cursor',in:'query',schema:{type:'string',maxLength:2048}},{name:'limit',in:'query',schema:{type:'integer',minimum:1,maximum:100}}];
export const diarySchemas={
 BrowserStoryOverview:object({as_of:text,timezone:text,fluctlight_id:text,actor_id:text,profile_id:text,first_communication_at:nullable(text),metrics:{type:'object',additionalProperties:object({value:{},available:{type:'boolean'},complete:{type:'boolean'}})},footprints:array(data),footprints_complete:{type:'boolean'},semantics:data}),
 BrowserDiaryAnnotation:object({favorite:{type:'boolean'},note:text,revision:number}),
 BrowserDiaryEntry:object({id:text,fluctlight_id:text,date:text,timezone:text,status:{type:'string',enum:['pending','running','ready','no_content','failed','inactive']},revision:number,latest_attempt:number,last_error_code:nullable(text),body:nullable(text),source:{type:'string',enum:['agent']},refs:array(text),created_at:text,updated_at:text,active_attempt_id:nullable(text),source_snapshot:data,persona_provenance:data,model_provenance:data,annotation:ref('BrowserDiaryAnnotation'),attempts:array(object({id:text,attempt:number,status:text,error_code:nullable(text),created_at:text,finished_at:nullable(text),plan:data,task_id:nullable(text),workflow_id:nullable(text),workflow_status:nullable(text)})),has_more_attempts:{type:'boolean'}},['id','fluctlight_id','date','timezone','status','revision','latest_attempt','last_error_code','body','source','refs','created_at','updated_at','active_attempt_id','source_snapshot','persona_provenance','model_provenance','annotation']),
 BrowserDiaryPage:object({items:array(ref('BrowserDiaryEntry')),next_cursor:text,timezone:text}),
 BrowserDiaryGenerateBody:object({localDate:{type:'string',format:'date'},timezone:text,expectedRevision:number,reason:{type:'string',minLength:1,maxLength:1000},idempotencyKey:key},['localDate','reason','idempotencyKey']),
 BrowserDiaryGeneration:object({diary_id:text,fluctlight_id:text,status:text,attempt:number,status_url:text,audit_id:text},['diary_id','fluctlight_id','status','attempt','status_url']),
 BrowserDiaryAnnotationBody:object({favorite:{type:'boolean'},note:{type:'string',maxLength:5000},expectedRevision:number,idempotencyKey:key}),
 BrowserDiaryAnnotationResult:object({diary_id:text,favorite:{type:'boolean'},note:text,revision:number,audit_id:text},['diary_id','favorite','note','revision']),
 BrowserDiaryReplyBody:object({text:{type:'string',minLength:1,maxLength:2000},expectedRevision:number,idempotencyKey:key}),
 BrowserDiaryReplies:object({items:array(ref('BrowserOwnerRequest')),next_cursor:text}),
 BrowserSharedExperiencePage:object({items:array(object({id:text,kind:{type:'string',enum:['remote_communication']},title:text,author_actor_id:text,message_kind:text,occurred_at:text,source_ref:text,conversation_id:text,copresent:{type:'boolean',const:false},href:text})),next_cursor:text,physical_presence_source:text}),
};
export const diaryPaths={
 '/api/fluctlights/{fluctlightId}/shared-experiences/overview':{get:{operationId:'storyOverview',...response('BrowserStoryOverview')}},
 '/api/fluctlights/{fluctlightId}/diary':{get:{operationId:'listDiary',parameters:[...paging,{name:'favorite',in:'query',schema:{type:'boolean'}}],...response('BrowserDiaryPage')}},
 '/api/fluctlights/{fluctlightId}/diary/{entryId}':{get:{operationId:'readDiary',...response('BrowserDiaryEntry')}},
 '/api/fluctlights/{fluctlightId}/diary/generate':{post:{operationId:'generateDiary',...request('BrowserDiaryGenerateBody'),...response('BrowserDiaryGeneration',202)}},
 '/api/fluctlights/{fluctlightId}/diary/{entryId}/annotation':{put:{operationId:'annotateDiary',...request('BrowserDiaryAnnotationBody'),...response('BrowserDiaryAnnotationResult')}},
 '/api/fluctlights/{fluctlightId}/diary/{entryId}/replies':{get:{operationId:'diaryReplies',parameters:paging,...response('BrowserDiaryReplies')},post:{operationId:'replyToDiary',...request('BrowserDiaryReplyBody'),responses:{202:{description:'Owner response persisted with actual cognition intent',content:{'application/json':{schema:object({request:ref('BrowserOwnerRequest'),audit_id:text})}}}}}},
 '/api/fluctlights/{fluctlightId}/shared-experiences':{get:{operationId:'sharedExperiences',parameters:paging,...response('BrowserSharedExperiencePage')}},
};
ownerPathContracts(diaryPaths);
