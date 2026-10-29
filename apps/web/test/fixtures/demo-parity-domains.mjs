// Same-content adapters for production DTOs. Technical revisions/identities are
// test carriers; business copy and resource bodies come from the immutable Demo.
export function parityDomains({data,owner,clock,chars,clone}) {
 const empty=()=>({items:[],next_cursor:''});
 const at=text=>{
  if(!text)return clock;
  const relative=text.match(/(\d+)\s*(分钟|小时|天)前/);
  if(relative)return new Date(Date.parse(clock)-Number(relative[1])*({分钟:60000,小时:3600000,天:86400000})[relative[2]]).toISOString();
  const date=text.match(/(\d{2})\/(\d{2})/),time=text.match(/(\d{1,2}):(\d{2})/);
  return `2026-${date?date[1]:'10'}-${date?date[2]:text.includes('昨天')?'26':'27'}T${time?time[1].padStart(2,'0'):'15'}:${time?time[2]:'45'}:00+08:00`;
 };
 const about=clone(data.about);delete about.history;
 const dimensionFields={'好奇与开放':'openness','独立与主见':'independence','细腻与共情':'empathy','社交能量':'sociability'};
 const personality=Object.fromEntries(data.defaultProfileDimensions.map(d=>[dimensionFields[d.name],d.value/100]));
 const worlds=fid=>data.worlds.map(w=>({id:w.id,fluctlight_id:fid,name:w.name,background:w.background,rules:w.rules,flavor:w.genre,description:'',item_pool:[],revision:w.revision,source_resource_id:null,source_version:null,active:chars.find(c=>c.id===fid)?.worldId===w.id,venue_categories:w.sites.map(s=>({id:s.id,name:s.name}))}));
 const places=wid=>(data.worlds.find(w=>w.id===wid)?.sites||[]).flatMap(s=>s.places.map(p=>({...clone(p),world_id:wid,venue_category_id:s.id,desc:p.description,status:'active',revision:1,parent_place_id:p.parentPlaceId||null,source_resource_id:null,source_version:null,connection_place_ids:p.connections,room_items:[]})));
 const npcs=fid=>data.people.map(p=>({id:p.id,world_id:chars.find(c=>c.id===fid).worldId,actor_id:'parity-npc-'+p.id,name:p.name,role:p.occupation,bio:p.description,status:'active',revision:1,enabled:true,full_fluctlight:false,position_mode:'fixed',routine:[],planned_position:null,current_presence:null,confirmed_place_id:null,confirmed_at:null,home_place_id:places(chars.find(c=>c.id===fid).worldId).find(row=>[row.name,row.alias].includes(p.location))?.id||null,source_resource_id:null,source_version:null,local_overrides:{}}));
 const relationships=fid=>data.people.filter(p=>p.met).map(p=>({id:'parity-relation-'+fid+'-'+p.id,owner_fluctlight_id:fid,target_actor_id:'parity-npc-'+p.id,profile_id:data.personas[fid].active,role:{label:p.relation},summary:p.source}));
 const worldView=(fid,wid=chars.find(c=>c.id===fid).worldId)=>({available:true,world:worlds(fid).find(w=>w.id===wid),places:places(wid),npcs:chars.find(c=>c.id===fid).worldId===wid?npcs(fid):[],scenes:[]});
 function profile(fid){
  const p=data.profiles[fid],identity={},behavioral_policy={};
  const active=data.personas[fid].profiles.find(profile=>profile.id===data.personas[fid].active);
  const compilation={profile_id:data.personas[fid].active,foundation_revision:1,overlay_revision:0,status:'ready',working_persona:{profile_id:data.personas[fid].active,working_persona:{profile_id:data.personas[fid].active,portrait_text:[active?.description,active?.tone].filter(Boolean).join('\n')}}};
  for(const k of ['name','nickname','gender','age','occupation','residence','timezone','core_values','birthday','birthplace','height_cm','blood_type','worldview','notes'])identity[k]=p[k];
  identity.background_story=p.background;
  const character=chars.find(c=>c.id===fid);identity.tags=character.personality;identity.color=character.color;identity.tagline=character.tagline;
  for(const k of ['response_style','message_length','directness','initiative','conflict_style','refusal_style','emoji_frequency','humor_style','topic_initiation','response_delay','punctuation_style','sarcasm_tendency','silence_tolerance','emotional_expression','intimacy_expression'])behavioral_policy[k]=p[k];
  for(const k of ['directness','initiative'])if(p[k]!==''&&Number.isFinite(Number(p[k])))behavioral_policy[k]=Number(p[k]);
  const life_profile={preferences:{likes:p.interests,stable_preferences:p.stable_preferences,clothing_style:p.style},life_habits:p.habits,character_constraints:p.boundaries,appearance:{description:p.appearance}};
  return {identity,behavioral_policy,life_profile,current_revision:1,core_persona:{identity,behavioral_policy,life_profile,personality,personality_system:{mode:'multiple',default_profile_id:data.personas[fid].active,profiles:clone(data.personas[fid].profiles)}},personality,personality_runtime:{active_profile_id:data.personas[fid].active,revision:1},persona_compilation:compilation,persona_switch_history:clone(data.personas[fid].history),actor_user:{background:about},relationships:relationships(fid)};
 }
 const resources=clone(data.resources).map(r=>({...r,owner_actor_id:owner,revision:1,referenced_by:[],updated_at:at(r.updatedAt),versions:r.versions.map(v=>({...v,id:r.id,immutable:true,resource_id:r.id,kind:r.kind,owner_actor_id:owner,created_at:at(v.at)}))}));
 const media=Object.entries(data.media).flatMap(([fid,rows])=>rows.map(m=>({id:fid+'-'+m.id,asset_id:fid+'-'+m.id,title:m.title,kind:'image',origin:m.source==='创作素材'?'studio':'fluctlight',fluctlight_id:fid,created_at:at(m.at),ready_at:at(m.at),retained:m.retained,hidden:false,status:'ready',readable:true,read_state:'readable',url:'/api/media/parity-media-'+(m.art||'portrait'),asset_revision:1,metadata_revision:1,version:'parity-1',mime_type:'image/svg+xml',factual:false,source_state:'recorded',source_snapshot:{description:m.description}})));
 const comments=p=>p.comments.map(c=>({id:c.id,moment_id:p.id,author_actor_id:c.author==='我'?owner:'parity-npc-'+(data.people.find(p=>p.name===c.author)?.id||c.id),author_name:c.author,author_type:c.author==='我'?'user':'npc',text:c.text,created_at:at(c.time),source_interaction_id:null}));
 const feed=clone(data.feed).map(p=>({id:p.id,owner_fluctlight_id:p.characterId==='me'?null:p.characterId,author_actor_id:p.characterId==='me'?owner:p.characterId,author_type:p.characterId==='me'?'user':'fluctlight',author_name:p.author,text:p.text,status:p.hidden?'hidden':'visible',visibility:'owner',revision:1,media_asset_ids:p.image?['parity-feed-'+p.id]:[],media:p.image?[{id:'parity-feed-'+p.id,kind:'image',readable:true,read_state:'readable',title:p.location,url:'/api/media/parity-scene-'+p.image}]:[],created_at:at(p.time),request_id:null,source_event_id:null,reaction_count:p.likes,like_count:p.likes,viewer_reaction:p.liked?'like':null,comment_count:p.comments.length}));
 const pending=fid=>['capability','persona_proposal','studio_apply','failed_task','owner_request'].map(kind=>({kind,status:'ok',count:kind==='persona_proposal'?1:kind==='failed_task'?1:0,items:kind==='persona_proposal'?[{id:'parity-persona-proposal',title:data.notices[1].title,status:'pending',fluctlight_id:'linwan',href:'/c/linwan/profile'}]:kind==='failed_task'?[{id:'task-portrait',title:data.notices[0].title,status:'failed',fluctlight_id:'linwan',href:'/diag/task/task-portrait'}]:[],next_cursor:''})).map(g=>fid&&fid!=='linwan'?{...g,count:0,items:[]}:g);
 // Inline Demo MediaPage seeds, ResourcePages.tsx:5134–5195, adapted to formal DTOs.
 const visualBrief=fid=>({fluctlight_id:fid,revision:1,brief:{appearance:chars.find(c=>c.id===fid)?.gender==='男'?'自然的短发与清晰的轮廓，神态舒展。':'深色短发，自然的五官与清晰的轮廓，神态从容。',clothing:'低饱和色系的日常衣物，以当前实际穿着为准。',constraints:'保持同一角色的面部特征、发型与身体比例。',references:'柔和的窗边自然光，安静的日常氛围。'}});
 const visual=fid=>({schema_version:'visual-identity.v1',fluctlight_id:fid,status:'ready',current_revision:1,identity_snapshot:{name:chars.find(c=>c.id===fid)?.name},renderer_constraints:{},canonical_asset_id:fid+'-m-portrait',character_sheet_asset_id:null,session:{id:'parity-visual-'+fid,status:'completed',current_attempt:1,last_error:null,created_at:at('10/26 09:30'),attempts:[{id:'parity-visual-attempt-'+fid,attempt_number:1,status:'accepted',candidate_asset_id:fid+'-m-portrait',decision:'accepted',brief_revision:1,created_at:at('10/26 09:30'),review:{source:'owner_manual',summary:'五官、发型与设定一致，保留为规范参考。'}}]}});
 // UI reference values are adapted to the formal configuration schema. Demo
 // handoffs=3, failurePolicy=fallback and autonomy=balanced are not wire enums.
 const settings=data.settings,replySettings=settings.reply,runtime=settings.runtime,advanced=settings.advanced,autonomy=settings.autonomy;
 const workflowExample=JSON.parse(settings.media.workflow),workflow=workflowExample.class_type?{'1':workflowExample}:workflowExample;
 const configuration={effective_values:{
  'conversation.reply':{mode:replySettings.mode,expression_version:replySettings.expressionVersion,temperature:replySettings.temperature,top_p:replySettings.topP,timeout_seconds:replySettings.timeout,concurrency:replySettings.concurrency,retry_limit:replySettings.retryLimit,max_action_handoffs:Math.min(2,replySettings.handoffs),failure_policy:replySettings.failurePolicy==='fallback'?'legacy_fallback':'error'},
  'vision.understanding':{enabled:replySettings.vision},
  'product.autonomy':{mode:autonomy.outreachPaused?'paused':'active',allowed_actions:[...autonomy.actions],confirm_action_types:[...autonomy.confirm],budget_remaining:autonomy.budget,concurrency_limit:autonomy.concurrency,cooldown_seconds:autonomy.cooldown,outreach_paused:autonomy.outreachPaused,notifications_muted:false,quiet_hours:{start:autonomy.start,end:autonomy.end,timezone:'Asia/Shanghai',days:[0,1,2,3,4,5,6]}},
  'product.notifications':{messages:runtime.messages,goals:runtime.goals,media:runtime.media},
  'product.wakeup':{enabled:runtime.wake,interval_seconds:runtime.wakeInterval},
  'product.summary':{interval_seconds:runtime.summaryInterval,max_runes:runtime.summaryRunes},
  'product.goal_retry':{max_attempts:runtime.goalRetries,base_seconds:runtime.retryBase,max_seconds:runtime.retryMax},
  'llm.queue':{generated_concurrency:runtime.generatedConcurrency,embedding_concurrency:runtime.embeddingConcurrency},
  'media.comfyui':{baseUrl:settings.media.url,generation_timeout_seconds:settings.media.timeout,workflow},
  kev:{enabled:advanced.enabled,media_enabled:advanced.media,endpoint:advanced.endpoint,timeout_ms:advanced.timeout,budget_ms:advanced.budget,concurrency:advanced.concurrency,batch_size:advanced.batchSize,strategy:advanced.strategy,min_probability:advanced.probability,min_margin:advanced.margin,max_deferrals:advanced.deferrals,deferral_seconds:advanced.deferralSeconds,retention_days:advanced.retention,model:advanced.model,model_revision:advanced.revision}
 },stored_values:{},versions:Object.fromEntries(settings.services.map(s=>['endpoint:'+s.id,1])),endpoints:settings.services.map(s=>({id:s.id,kind:'openai-compatible',base_url:s.url,secret_configured:s.keyConfigured})),bindings:settings.roles.map(r=>({role:r.id,endpoint_id:r.serviceId,model_id:r.model,timeout_seconds:r.timeout})),configured_secrets:[]};

 return {profile,at,pending,configuration,async handle({path,method,url,reply}){
  if(method!=='GET')return false;
  if(path==='/api/studio/resources')return reply({items:resources.filter(r=>(url.searchParams.get('archived')==='true'?r.status==='archived':r.status!=='archived')&&(!url.searchParams.get('kind')||r.kind===url.searchParams.get('kind'))&&(!url.searchParams.get('q')||r.name.includes(url.searchParams.get('q')))&&(url.searchParams.get('published')!=='true'||!!r.published_version)),next_cursor:''});
  const resource=path.match(/^\/api\/studio\/resources\/([^/]+)(?:\/versions(?:\/([^/]+))?)?$/);
  if(resource){const row=resources.find(r=>r.id===resource[1]);if(!row)return false;if(path.includes('/versions'))return reply(resource[2]?row.versions.find(v=>v.version===resource[2]):{items:row.versions,next_cursor:''});return reply(row);}
  if(['/api/studio/jobs','/api/studio/materials','/api/media-reference-grants','/api/moment-suggestions'].includes(path))return reply(empty());
  if(path==='/api/media-library')return reply({items:media.filter(m=>(!url.searchParams.get('fluctlightId')||m.fluctlight_id===url.searchParams.get('fluctlightId'))&&(!url.searchParams.get('q')||m.title.includes(url.searchParams.get('q')))&&(!url.searchParams.get('kind')||m.kind===url.searchParams.get('kind'))&&(url.searchParams.get('scope')==='all'||!url.searchParams.get('scope')||m.origin===url.searchParams.get('scope'))&&m.hidden===(url.searchParams.get('hidden')==='true')),next_cursor:''});
  if(path.startsWith('/api/media-assets/')){const row=media.find(m=>m.id===path.split('/')[3]);if(row)return reply(row);}
  if(path==='/api/moment-feed')return reply({items:feed.filter(p=>(!url.searchParams.get('fluctlightId')||p.owner_fluctlight_id===url.searchParams.get('fluctlightId'))&&p.status===(url.searchParams.get('status')||'visible')&&(!url.searchParams.get('q')||p.text.includes(url.searchParams.get('q')))),next_cursor:''});
  const post=path.match(/^\/api\/moment-feed\/([^/]+)(?:\/(comments|npc-interactions))?$/);
  if(post){const row=feed.find(p=>p.id===post[1]);if(!row)return false;return reply(post[2]==='comments'?{items:comments(data.feed.find(p=>p.id===post[1])),next_cursor:''}:post[2]?empty():row);}
  const c=path.match(/^\/api\/fluctlights\/([^/]+)(\/.*)$/);if(!c||!chars.some(x=>x.id===c[1]))return false;
  const [,fid,end]=c;
  if(end==='/visual-brief')return reply(visualBrief(fid));
  if(end==='/visual-brief/history')return reply({items:[{id:'parity-brief-'+fid,revision:1,reason:'建立初始视觉参考',before_value:{},after_value:visualBrief(fid).brief,created_at:at('10/26 09:00')}],next_cursor:''});
  if(end==='/visual-identity')return reply(visual(fid));
  if(end==='/visual-identity/timeline')return reply({items:[{id:'parity-visual-history-'+fid,stage:'accepted',status:'completed',summary:'五官、发型与设定一致，保留为规范参考。',asset_ids:[fid+'-m-portrait'],occurred_at:at('10/26 09:30')}],next_cursor:''});
  if(end==='/display-avatar')return reply({fluctlight_id:fid,revision:1,mode:'canonical',selection:null,avatar:{asset_id:fid+'-m-portrait',url:'/api/media/parity-avatar'},read_state:'readable'});
  if(['/display-avatar/history','/media-generations'].includes(end))return reply(empty());
  if(end==='/world')return reply(worldView(fid));
  if(end==='/world/catalog')return reply({items:worlds(fid),active_binding:{world_id:chars.find(x=>x.id===fid).worldId,revision:1},next_cursor:''});
  if(end.startsWith('/world/catalog/'))return reply(worldView(fid,end.split('/')[3]));
  if(end==='/npcs')return reply({items:npcs(fid),next_cursor:''});
  if(end.startsWith('/npcs/')){const row=npcs(fid).find(n=>n.id===end.split('/')[2]);if(row)return reply(end.endsWith('/encounters')?empty():{...row,relationship_context:{relationships:relationships(fid).filter(r=>r.target_actor_id===row.actor_id)}});}
  if(end==='/actor-context/'+owner)return reply({actor_id:owner,actor_type:'human',context_version:'parity-actor-context-1',background:about,facts:[],history:[],relationships:[],relationship_history:[]});
  return false;
 }};
}
