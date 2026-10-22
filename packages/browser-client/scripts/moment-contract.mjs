import {ownerPathContracts} from './owner-contract-utils.mjs';
const text={type:'string'},integer={type:'integer',minimum:1},ref=n=>({$ref:'#/components/schemas/'+n}),nullable=s=>({anyOf:[s,{type:'null'}]}),data={type:'object',additionalProperties:true},object=(properties,required=Object.keys(properties))=>({type:'object',properties,required,additionalProperties:false});
const command={expectedRevision:integer,reason:{type:'string',minLength:1,maxLength:1000},idempotencyKey:{type:'string',minLength:1,maxLength:256}};
export const momentSchemas={
 BrowserMomentNPCPage:object({items:{type:'array',items:data},next_cursor:text}),
 BrowserMomentSuggestions:object({items:{type:'array',items:ref('BrowserOwnerRequest')},next_cursor:text}),
 BrowserMomentRecord:object({id:text,owner_fluctlight_id:nullable(text),author_actor_id:text,author_type:text,author_name:nullable(text),text,status:{type:'string',enum:['visible','hidden']},visibility:text,revision:integer,media_asset_ids:{type:'array',items:text},media:{type:'array',items:data},created_at:text,request_id:nullable(text),source_event_id:nullable(text),like_count:{type:'integer',minimum:0},reaction_count:{type:'integer',minimum:0},viewer_reaction:nullable(text),comment_count:{type:'integer',minimum:0}}),
 BrowserMomentPage:object({items:{type:'array',items:ref('BrowserMomentRecord')},next_cursor:text}),
 BrowserMomentComments:object({items:{type:'array',items:object({id:text,moment_id:text,author_actor_id:text,author_type:text,author_name:nullable(text),text,created_at:text,source_interaction_id:nullable(text)})},next_cursor:text}),
 BrowserMomentCommentBody:object({...command,text:{type:'string',minLength:1,maxLength:2000}}),
 BrowserMomentReactionBody:object({...command,kind:{type:'string',enum:['like','care','celebrate','none']}}),
 BrowserMomentStatusBody:object(command),
};
const response=name=>({responses:{200:{description:'Persistent authorized Moment state',content:{'application/json':{schema:ref(name)}}}}}),body=name=>({requestBody:{required:true,content:{'application/json':{schema:ref(name)}}},...response('BrowserMomentRecord')}),param=(name,schema=text)=>({name,in:'query',schema});
export const momentPaths=ownerPathContracts({
 '/api/moment-feed/{momentId}/npc-interactions':{get:{operationId:'momentNPCInteractions',parameters:[param('cursor')],...response('BrowserMomentNPCPage')}},
 '/api/moment-feed/{momentId}/npc-interactions/{interactionId}':{get:{operationId:'momentNPCInteraction',responses:{200:{description:'Actual NPC interaction',content:{'application/json':{schema:data}}}}}},
 '/api/moment-feed/{momentId}/npc-interactions/{interactionId}/retry':{post:{operationId:'retryMomentNPCInteraction',requestBody:{required:true,content:{'application/json':{schema:ref('BrowserMomentStatusBody')}}},responses:{200:{description:'Durable retry with source fences; not a completed interaction',content:{'application/json':{schema:data}}}}}},
 '/api/moment-suggestions':{get:{operationId:'momentSuggestions',parameters:[param('fluctlightId'),param('cursor')],...response('BrowserMomentSuggestions')}},
 '/api/moment-feed':{get:{operationId:'momentFeed',parameters:[param('fluctlightId'),param('status',{type:'string',enum:['visible','hidden','all']}),param('q'),param('cursor'),param('limit',{type:'integer',minimum:1,maximum:100})],...response('BrowserMomentPage')}},
 '/api/moment-feed/{momentId}':{get:{operationId:'momentRecord',...response('BrowserMomentRecord')}},
 '/api/moment-feed/{momentId}/comments':{get:{operationId:'momentComments',parameters:[param('cursor')],...response('BrowserMomentComments')},post:{operationId:'writeMomentComment',...body('BrowserMomentCommentBody')}},
 '/api/moment-feed/{momentId}/reaction':{post:{operationId:'writeMomentReaction',...body('BrowserMomentReactionBody')}},
 '/api/moment-feed/{momentId}/hide':{post:{operationId:'hideMomentRecord',...body('BrowserMomentStatusBody')}},
 '/api/moment-feed/{momentId}/restore':{post:{operationId:'restoreMomentRecord',...body('BrowserMomentStatusBody')}},
});
