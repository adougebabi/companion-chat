const text={type:'string'},integer={type:'integer',minimum:0},nullable=value=>({anyOf:[value,{type:'null'}]});
const array=items=>({type:'array',items});
const object=properties=>({type:'object',properties,required:Object.keys(properties),additionalProperties:false});
const ref=name=>({$ref:'#/components/schemas/'+name});
const response=name=>({responses:{200:{description:'Owner-authorized memory read',content:{'application/json':{schema:ref(name)}}}}});
const page=name=>object({items:array(ref(name)),next_cursor:text});
const limits=[{name:'cursor',in:'query',schema:{type:'string',maxLength:2048}},{name:'limit',in:'query',schema:{type:'integer',minimum:1,maximum:100,default:30}}];
export const memorySchemas={
 BrowserManagedMemory:{...object({id:text,owner_fluctlight_id:text,type:{type:'string',enum:['episodic','semantic','relationship','autobiographical']},content:text,actor_refs:array(text),conversation_id:nullable(text),event_refs:array(text),evidence_refs:array(text),personality_perspectives:array({type:'object',additionalProperties:true}),confidence:{type:'number'},importance:{type:'number'},emotional_significance:{type:'number'},visibility:{type:'string',enum:['private','owner','participants']},status:{type:'string',enum:['active','superseded','deprecated','forgotten']},revision:integer,provenance_status:text,occurred_at:nullable(text),created_at:text,last_confirmed_at:nullable(text),superseded_by_memory_id:nullable(text),supersedes_memory_id:nullable(text),revision_count:integer,source_count:integer})},
 BrowserMemoryRevision:object({revision:integer,base_revision:nullable(integer),operation:text,content:text,status:text,actor_id:text,evidence_refs:array(text),source_window:nullable(text),proposal_id:nullable(text),semantic_reason:text,related_memory_ids:array(text),reason_code:nullable(text),schema_version:text,created_at:text}),
 BrowserMemorySource:object({memory_revision:integer,source_ref:text,source_kind:text,source_id:nullable(text),source_revision:nullable(integer),occurred_at:nullable(text),recorded_at:text,status:text,live:{type:'boolean'},superseded_by_ref:nullable(text),conversation_id:nullable(text)}),
 BrowserMemoryRollback:object({targetRevision:integer,expectedRevision:integer,evidenceRefs:{...array({type:'string',minLength:1,maxLength:256}),minItems:1,maxItems:32}}),
 BrowserManagedMemoryPage:page('BrowserManagedMemory'),BrowserMemoryRevisionPage:page('BrowserMemoryRevision'),BrowserMemorySourcePage:page('BrowserMemorySource'),
};
export const memoryPaths={
 '/api/fluctlights/{fluctlightId}/memories':{get:{operationId:'listManagedMemories',parameters:[...limits,{name:'q',in:'query',schema:{type:'string',maxLength:500}},{name:'type',in:'query',schema:{type:'string',enum:['episodic','semantic','relationship','autobiographical']}},{name:'status',in:'query',schema:{type:'string',enum:['active','superseded','deprecated','forgotten']}}],...response('BrowserManagedMemoryPage')}},
 '/api/memories/{memoryId}/revisions':{get:{operationId:'memoryRevisions',parameters:limits,...response('BrowserMemoryRevisionPage')}},
 '/api/memories/{memoryId}/sources':{get:{operationId:'memorySources',parameters:[...limits,{name:'revision',in:'query',schema:integer}],...response('BrowserMemorySourcePage')}},
 '/api/memories/{memoryId}/rollback':{post:{operationId:'rollbackMemory',requestBody:{required:true,content:{'application/json':{schema:ref('BrowserMemoryRollback')}}},responses:{200:{description:'Appended compensation revision; existing lifecycle authority'}}}},
};
export const managedMemoryRead={operationId:'managedMemory',...response('BrowserManagedMemory')};
