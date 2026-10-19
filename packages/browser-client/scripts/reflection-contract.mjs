const text={type:'string'},integer={type:'integer',minimum:0},data={type:'object',additionalProperties:true},nullable=shape=>({anyOf:[shape,{type:'null'}]});
const array=items=>({type:'array',items}),ref=name=>({$ref:'#/components/schemas/'+name});
const object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false});
const reply=name=>({responses:{200:{description:'Owner-authorized actual Reflection V2 data',content:{'application/json':{schema:ref(name)}}}}});
const paging=[{name:'cursor',in:'query',schema:{type:'string',maxLength:2048}},{name:'limit',in:'query',schema:{type:'integer',minimum:1,maximum:100,default:30}}];
const summary=object({id:text,fluctlight_id:text,title:text,summary:nullable(text),body:nullable(text),state:text,status:text,growth:{type:'null'},at:text,created_at:text,from_sequence:integer,to_sequence:integer,base_state_revision:integer,source_refs:array(text),evidence_refs:array(text),schema_version:text,model_version:nullable(text),prompt_version:nullable(text),policy_version:text,correlation_id:text,expected_revisions:data,result:data});
export const reflectionSchemas={
 BrowserReflectionSummary:summary,
 BrowserReflectionCatalog:object({items:array(ref('BrowserReflectionSummary')),next_cursor:text,window:nullable(data),latest_task:nullable(data)}),
 BrowserReflectionDetail:object({...summary.properties,source_window:nullable(data),source_window_status:text,proposal:data,references:array(object({ref:text,kind:text,entity_id:text,revision:integer})),dispositions:array(object({domain:text,candidate_id:text,candidate_index:integer,disposition:text,reason_code:text,target_ref:nullable(text),changed_ref:nullable(text),base_revision:integer,resulting_revision:integer,evidence_refs:array(text)}))}),
 BrowserReflectionFeedbackBody:object({text:{type:'string',minLength:1,maxLength:2000},stance:{type:'string',enum:['agree','disagree']},idempotencyKey:{type:'string',minLength:1,maxLength:256}},['text','idempotencyKey']),
 BrowserOwnerRequest:{type:'object',additionalProperties:true,properties:{id:text,kind:text,fluctlight_id:text,status:{type:'string',enum:['pending','processing','accepted','rejected','deferred','failed','completed']},payload:data,result:data,decided_by:nullable(text),decision_reason:nullable(text),result_refs:array(text),created_at:text,updated_at:text,revision:integer},required:['id','kind','fluctlight_id','status','payload','decided_by','decision_reason','result_refs','created_at','updated_at','revision']},
 BrowserReflectionFeedbackPage:object({items:array(ref('BrowserOwnerRequest')),next_cursor:text}),
};
export const reflectionPaths={
 '/api/fluctlights/{fluctlightId}/reflections':{get:{operationId:'listReflections',parameters:paging,...reply('BrowserReflectionCatalog')}},
 '/api/fluctlights/{fluctlightId}/reflections/{reflectionId}':{get:{operationId:'readReflection',...reply('BrowserReflectionDetail')}},
 '/api/fluctlights/{fluctlightId}/reflections/{reflectionId}/feedback':{get:{operationId:'reflectionFeedback',parameters:paging,...reply('BrowserReflectionFeedbackPage')},post:{operationId:'submitReflectionFeedback',requestBody:{required:true,content:{'application/json':{schema:ref('BrowserReflectionFeedbackBody')}}},responses:{202:{description:'Durable Owner request and cognition fact committed atomically; not acceptance of feedback contents',content:{'application/json':{schema:object({request:ref('BrowserOwnerRequest'),audit_id:text})}}}}}},
};
