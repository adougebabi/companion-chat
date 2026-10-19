const kinds=['character_template','persona_fragment','place','item','outfit','event_template','visual_style','world','npc'];
const object={type:'object',additionalProperties:true};
const publication={type:'object',additionalProperties:false,properties:{id:{type:'string'},version:{type:'string'},kind:{type:'string',enum:kinds},name:{type:'string'},body:object,immutable:{type:'boolean',enum:[true]}},required:['id','version','kind','name','body','immutable']};
export const studioCatalogSchemas={
 BrowserStudioPublication:publication,
 BrowserFluctlightCreationAnalysisRequest:{anyOf:[
  {type:'object',additionalProperties:false,properties:{description:{type:'string',minLength:1,maxLength:60000,'x-maxBytes':60000}},required:['description']},
  {type:'object',additionalProperties:false,properties:{json:{...object,minProperties:1}},required:['json']},
  {type:'object',additionalProperties:false,properties:{resourceId:{type:'string',minLength:1,maxLength:128},resourceVersion:{type:'string',minLength:1,maxLength:64}},required:['resourceId','resourceVersion']}
 ]}
};
const ref=name=>({$ref:`#/components/schemas/${name}`});
export const studioCatalogPaths={
 '/api/studio/resources':{get:{operationId:'listStudioResources',parameters:[
  {name:'kind',in:'query',schema:{type:'string',enum:kinds}},
  {name:'q',in:'query',schema:{type:'string',maxLength:200}},
  {name:'published',in:'query',schema:{type:'boolean'}},
  {name:'archived',in:'query',schema:{type:'boolean'}},
  {name:'cursor',in:'query',schema:{type:'string'}},
  {name:'limit',in:'query',schema:{type:'integer',minimum:1,maximum:100}}
 ],responses:{200:{description:'Owner catalogue, filters applied before pagination',content:{'application/json':{schema:{type:'object',properties:{items:{type:'array',items:object},next_cursor:{anyOf:[{type:'string'},{type:'null'}]}},required:['items','next_cursor']}}}}}}},
 '/api/studio/resources/{resourceId}/versions/{version}':{get:{operationId:'readStudioPublication',parameters:['resourceId','version'].map(name=>({name,in:'path',required:true,schema:{type:'string'}})),responses:{200:{description:'Exact immutable Owner publication',content:{'application/json':{schema:ref('BrowserStudioPublication')}}}}}}
};

const commandFields={expectedRevision:{type:'integer',minimum:1},reason:{type:'string',minLength:1,maxLength:1000},idempotencyKey:{type:'string',minLength:1,maxLength:256}};
studioCatalogSchemas.BrowserStudioCatalogBody={type:'object',additionalProperties:false,properties:commandFields,required:Object.keys(commandFields)};
studioCatalogSchemas.BrowserStudioCopyBody={type:'object',additionalProperties:false,properties:{...commandFields,source:{type:'string',enum:['draft','published']},sourceVersion:{type:'string',minLength:1,maxLength:64},name:{type:'string',minLength:1,maxLength:200}},required:[...Object.keys(commandFields),'source','sourceVersion','name']};
const mutation=(id,body)=>({operationId:id,requestBody:{required:true,content:{'application/json':{schema:ref(body)}}},responses:{200:{description:'Persisted resource and audit receipt',content:{'application/json':{schema:object}}}}});
studioCatalogPaths['/api/studio/resources/{resourceId}/copy']={post:mutation('copyStudioResource','BrowserStudioCopyBody')};
studioCatalogPaths['/api/studio/resources/{resourceId}/restore']={post:mutation('restoreStudioResource','BrowserStudioCatalogBody')};
studioCatalogPaths['/api/studio/resources/{resourceId}']={delete:mutation('archiveStudioResource','BrowserStudioCatalogBody')};
studioCatalogPaths['/api/studio/resources/{resourceId}/versions']={get:{operationId:'listStudioVersions',responses:{200:{description:'Owner exact publication metadata with keyset pagination',content:{'application/json':{schema:{type:'object',properties:{items:{type:'array',items:object},next_cursor:{anyOf:[{type:'string'},{type:'null'}]}},required:['items','next_cursor']}}}}}}};
