import {ownerPathContracts} from './owner-contract-utils.mjs';
const text={type:'string'},data={type:'object',additionalProperties:true},ref=n=>({$ref:'#/components/schemas/'+n}),object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false});
const response=n=>({responses:{200:{description:'Actual Owner definition package or owning objects',content:{'application/json':{schema:ref(n)}}}}}),request=n=>({requestBody:{required:true,content:{'application/json':{schema:ref(n)}}}});
export const dataToolsSchemas={
 BrowserBusinessBackupJob:object({id:text,kind:{type:'string',enum:['backup','restore']},source_backup_id:{anyOf:[text,{type:'null'}]},target_name:{anyOf:[text,{type:'null'}]},status:{type:'string',enum:['queued','running','completed','failed','cancelled']},revision:{type:'integer',minimum:1},attempt:{type:'integer',minimum:1},reason:text,stage:text,result:data,steps:{type:'array',items:data},error_code:{anyOf:[text,{type:'null'}]},created_at:text,started_at:{anyOf:[text,{type:'null'}]},finished_at:{anyOf:[text,{type:'null'}]}}),
 BrowserBusinessBackupPage:object({items:{type:'array',items:ref('BrowserBusinessBackupJob')},next_cursor:text,strategy:text,execution:text,maintenance_required:{type:'boolean',const:true}}),
 BrowserBusinessBackupBody:object({reason:{type:'string',minLength:1,maxLength:1000},idempotencyKey:{type:'string',minLength:1,maxLength:256},expectedRevision:{type:'integer',minimum:1},confirm:{type:'boolean',const:true},targetName:{type:'string',pattern:'^fluctlight_restore_[a-z0-9]{8,32}$'}},['reason','idempotencyKey']),

 BrowserResourcePack:object({schema:{type:'string',const:'fluctlight-resource-pack.v1'},created_at:text,checksum:text,resources:{type:'array',items:data},groups:{type:'array',items:object({name:text})},preferences:data}),
 BrowserResourcePackValidation:object({valid:{type:'boolean',const:true},schema:text,digest:text,entries:{type:'array',items:data},publish_order:{type:'array',items:text},external_bindings:{type:'array',items:data},semantics:text}),
 BrowserResourcePackValidateBody:object({pack:ref('BrowserResourcePack')}),
 BrowserResourcePackImportBody:object({pack:ref('BrowserResourcePack'),expectedDigest:text,reason:{type:'string',minLength:1,maxLength:1000},idempotencyKey:{type:'string',minLength:1,maxLength:256}}),
 BrowserResourcePackImportResult:object({id:text,pack_id:text,status:text,resource_versions:{type:'array',items:data},digest:text,life_write:{type:'boolean',const:false},published:{type:'boolean',const:false},validation:ref('BrowserResourcePackValidation'),audit_id:text}),
 BrowserResourcePackExport:object({pack:ref('BrowserResourcePack'),available:{type:'boolean',const:true},complete:{type:'boolean',const:true},validation:ref('BrowserResourcePackValidation'),scope:text}),
 BrowserResourcePackGroupBody:object({name:text,expectedRevision:{type:'integer',minimum:1},reason:{type:'string',minLength:1,maxLength:1000},idempotencyKey:{type:'string',minLength:1,maxLength:256}}),
 BrowserObjectRevisionPage:object({items:{type:'array',items:object({id:text,kind:text,fluctlight_id:text,title:{anyOf:[text,{type:'null'}]},revision:{type:'integer'},history_count:{type:'integer'},updated_at:text,href:text})},next_cursor:text,filters:data,semantics:text}),
};
export const dataToolsPaths=ownerPathContracts({
 '/api/ops/backups':{get:{operationId:'ownerBackupJobs',parameters:[{name:'cursor',in:'query',schema:text}],...response('BrowserBusinessBackupPage')},post:{operationId:'createOwnerBackup',...request('BrowserBusinessBackupBody'),responses:{200:{description:'Durable maintenance job; queued is not a completed backup',content:{'application/json':{schema:data}}}}}},
 '/api/ops/backups/{backupId}':{get:{operationId:'ownerBackupJob',...response('BrowserBusinessBackupJob')}},
 ...Object.fromEntries(['restore','retry','cancel'].map(action=>['/api/ops/backups/{backupId}/'+action,{post:{operationId:'ownerBackup'+action[0].toUpperCase()+action.slice(1),...request('BrowserBusinessBackupBody'),responses:{200:{description:'Versioned maintenance request receipt',content:{'application/json':{schema:data}}}}}}])),

 '/api/studio/packs/export':{get:{operationId:'exportOwnerResourcePack',parameters:[{name:'resourceId',in:'query',schema:text}],...response('BrowserResourcePackExport')}},
 '/api/studio/packs/validate':{post:{operationId:'validateOwnerResourcePack',...request('BrowserResourcePackValidateBody'),...response('BrowserResourcePackValidation')}},
 '/api/studio/packs':{post:{operationId:'importOwnerResourcePack',...request('BrowserResourcePackImportBody'),...response('BrowserResourcePackImportResult')}},
 '/api/studio/packs/{packId}':{get:{operationId:'ownerResourcePack',responses:{200:{description:'Persisted import mapping and suggestion drafts',content:{'application/json':{schema:data}}}}}},
 '/api/studio/packs/{packId}/groups/{index}/adopt':{post:{operationId:'adoptOwnerResourcePackGroup',...request('BrowserResourcePackGroupBody'),responses:{200:{description:'Explicit group draft adoption; no member movement',content:{'application/json':{schema:data}}}}}},
 '/api/data/revisions':{get:{operationId:'ownerObjectRevisions',parameters:['kind','fluctlightId','objectId','q','cursor'].map(name=>({name,in:'query',schema:text})),...response('BrowserObjectRevisionPage')}},
});
