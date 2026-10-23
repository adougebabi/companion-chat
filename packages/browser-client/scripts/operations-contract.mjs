import {ownerPathContracts} from './owner-contract-utils.mjs';
const text={type:'string'}, count={type:'integer',minimum:0}, bool={type:'boolean'};
const ref=name=>({$ref:'#/components/schemas/'+name});
const nullable=value=>({anyOf:[value,{type:'null'}]});
const array=items=>({type:'array',items});
const object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false});
const reply=name=>({responses:{200:{description:'Authorized durable operational projection',content:{'application/json':{schema:ref(name)}}}}});
const body=name=>({requestBody:{required:true,content:{'application/json':{schema:ref(name)}}}});
const page=[{name:'cursor',in:'query',schema:{type:'string',maxLength:2048}},{name:'limit',in:'query',schema:{type:'integer',minimum:1,maximum:100}}];
const scope={name:'fluctlightId',in:'query',schema:{type:'string',maxLength:128}};
export const operationsSchemas={
 BrowserAutonomyPolicy: {type:'object',additionalProperties:true,properties:{fluctlight_id:text,policy_source:{type:'string',enum:['inherit','override']},mode:text,allowed_actions:array(text),budget_remaining:{type:'number',minimum:0},quiet_hours:object({start:text,end:text,timezone:text,days:{type:'array',items:{type:'integer',minimum:0,maximum:6}}},[]),concurrency_limit:count,cooldown_seconds:count,outreach_paused:bool,notifications_muted:bool,confirm_action_types:array(text),revision:count,runtime_effect:{type:'string',enum:['enforced']},global_policy:{type:'object',additionalProperties:true},legacy_preferences:{type:'object',additionalProperties:true}},required:['fluctlight_id','policy_source','mode','allowed_actions','budget_remaining','quiet_hours','concurrency_limit','cooldown_seconds','outreach_paused','notifications_muted','confirm_action_types','revision','runtime_effect']},
 BrowserAutonomyPolicyWriteBody:object({policySource:{type:'string',enum:['inherit','override']},mode:{type:'string',enum:['active','paused','muted']},allowedActions:array(text),budgetRemaining:{type:'integer',minimum:0,maximum:1000000},quietHours:{type:'object',additionalProperties:false,properties:{start:text,end:text,timezone:text,days:{type:'array',items:{type:'integer',minimum:0,maximum:6},minItems:1,maxItems:7,uniqueItems:true}}},concurrencyLimit:{type:'integer',minimum:1,maximum:32},cooldownSeconds:{type:'integer',minimum:0,maximum:604800},outreachPaused:bool,notificationsMuted:bool,confirmActionTypes:array(text),expectedRevision:count,reason:{type:'string',minLength:1,maxLength:1000},idempotencyKey:{type:'string',minLength:1,maxLength:256}},['policySource','expectedRevision','reason','idempotencyKey']),
 BrowserNotification:object({id:text,fluctlight_id:nullable(text),title:text,body:text,link:text,quiet:bool,read_at:nullable(text),created_at:text,source_kind:text,source_id:text,source_event_id:text,source_occurred_at:text}),
 BrowserNotificationPage:object({items:array(ref('BrowserNotification')),next_cursor:text,as_of:text}),
 BrowserNotificationCount:object({count,alert_count:count}),
 BrowserNotificationReadBody:object({idempotencyKey:{type:'string',minLength:1,maxLength:256}}),
 BrowserNotificationReadAllBody:object({before:{type:'string',format:'date-time'},idempotencyKey:{type:'string',minLength:1,maxLength:256}}),
 BrowserNotificationReadResult:object({id:text,read_at:text}),
 BrowserNotificationReadAllResult:object({updated:count,before:text}),
 BrowserPendingItem:object({id:text,source_id:text,fluctlight_id:text,title:text,status:text,href:text,created_at:text}),
 BrowserPendingGroup:object({kind:{type:'string',enum:['capability','persona_proposal','studio_apply','failed_task','owner_request']},status:{type:'string',enum:['ok','failed']},items:array(ref('BrowserPendingItem')),count,next_cursor:text,error_code:text,message:text},['kind','status']),
 BrowserPendingPage:object({groups:array(ref('BrowserPendingGroup')),count,partial:bool}),
 BrowserPendingCount:object({count,partial:bool}),
 BrowserRequestTask:object({task_id:text,workflow_id:text,kind:text,status:text,attempt_count:count}),
 BrowserTask:object({task_id:text,workflow_id:text,fluctlight_id:nullable(text),kind:text,type:text,title:text,state:text,status:text,runs:count,steps:array(object({id:text,event_type:text,name:text,status:text,reason_code:text,at:text})),has_more_steps:bool,created_at:text,started_at:nullable(text),completed_at:nullable(text),next_attempt_at:nullable(text),correlation_id:text,source:{type:'object',additionalProperties:true}}),
};
export const operationsPaths={
 '/api/fluctlights/{fluctlightId}/autonomy-policy':{get:{operationId:'readAutonomyPolicy',...reply('BrowserAutonomyPolicy')},put:{operationId:'updateAutonomyPolicy',...body('BrowserAutonomyPolicyWriteBody'),...reply('BrowserAutonomyPolicy')}},
 '/api/notifications':{get:{operationId:'listNotifications',parameters:[...page,scope,{name:'unread',in:'query',schema:bool}],...reply('BrowserNotificationPage')}},
 '/api/notifications/unread-count':{get:{operationId:'notificationUnreadCount',...reply('BrowserNotificationCount')}},
 '/api/notifications/{notificationId}/read':{post:{operationId:'markNotificationRead',...body('BrowserNotificationReadBody'),...reply('BrowserNotificationReadResult')}},
 '/api/notifications/read-all':{post:{operationId:'markAllNotificationsRead',...body('BrowserNotificationReadAllBody'),...reply('BrowserNotificationReadAllResult')}},
 '/api/pending':{get:{operationId:'listPending',parameters:[...page,scope,{name:'kind',in:'query',schema:operationsSchemas.BrowserPendingGroup.properties.kind}],...reply('BrowserPendingPage')}},
 '/api/pending/count':{get:{operationId:'pendingCount',parameters:[scope],...reply('BrowserPendingCount')}},
 '/api/diagnostics/tasks/{taskId}':{get:{operationId:'diagnosticTask',...reply('BrowserTask')}},
};

ownerPathContracts(operationsPaths);
