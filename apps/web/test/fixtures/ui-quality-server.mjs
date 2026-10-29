// Task-owned same-origin fixture for production-build UI verification.
// Binds loopback only; never proxies to a real Core/Provider or persists user data.
import http from 'node:http';
import {demoParityFixture} from './demo-parity-fixture.mjs';
import {managementFixture} from './management-fixture.mjs';
import {scheduleReplayFixture} from './schedule-fixture.mjs';
import {goalPlanningFixture} from './goal-planning-fixture.mjs';
import {goalsFixture} from './goals-fixture.mjs';
import {diaryFixture} from './diary-fixture.mjs';
import {growthFixture} from './growth-fixture.mjs';
import {profileFixture} from './profile-fixture.mjs';
import {worldFixture} from './world-fixture.mjs';
import {peopleFixture} from './people-fixture.mjs';
import {feedFixture} from './feed-fixture.mjs';
import {mediaFixture} from './media-fixture.mjs';
import {studioFixture} from './studio-fixture.mjs';
import {creationHttpFixture} from './creation-fixture.mjs';
import {r16Fixture} from './r16-fixture.mjs';
import {reactWorkspaceFixture} from './react-workspace-fixture.mjs';
import {wardrobeWorldFixture} from './wardrobe-world-fixture.mjs';
import {readFile,writeFile} from 'node:fs/promises';
import {resolve,extname} from 'node:path';
import {fileURLToPath} from 'node:url';
const root=resolve(fileURLToPath(new URL('../../',import.meta.url)),'dist');
const port=Number(process.env.FLUCTLIGHT_UI_FIXTURE_PORT ?? 4175);
const nasAdditions=process.env.FLUCTLIGHT_UI_FIXTURE_NAS_ADDITIONS==='1';
const pendingChat=process.env.FLUCTLIGHT_UI_FIXTURE_PENDING_CHAT==='1';
const scheduleScenario=process.env.FLUCTLIGHT_UI_FIXTURE_SCHEDULE ?? '';
const lifeScenario=process.env.FLUCTLIGHT_UI_FIXTURE_LIFE ?? '';
let lifeScheduleReads=0;
const chatScenario=process.env.FLUCTLIGHT_UI_FIXTURE_CHAT ?? '';
let conversationReads=0,historyReads=0;
const libraryScenario=process.env.FLUCTLIGHT_UI_FIXTURE_LIBRARY ?? '';
let libraryDetailReads=0;
const injectFailures=process.env.FLUCTLIGHT_UI_FIXTURE_FAILURES === '1';
let actorReads=0,diagnosticClears=0;
const telemetryPath='/private/tmp/fluctlight-ui-fixture-telemetry.json';
const owner='human-fixture-owner',instance='fluctlight-fixture-one';
const date='2026-10-13T01:00:00.123Z';
const parityReplay=process.env.FLUCTLIGHT_UI_FIXTURE_DEMO?demoParityFixture({scenario:process.env.FLUCTLIGHT_UI_FIXTURE_DEMO}):null;
const managementReplay=process.env.FLUCTLIGHT_UI_FIXTURE_MANAGEMENT?managementFixture({owner,fid:instance,date,scenario:process.env.FLUCTLIGHT_UI_FIXTURE_MANAGEMENT}):null;
const r16=process.env.FLUCTLIGHT_UI_FIXTURE_R16==='1'?r16Fixture({owner,fid:instance,date}):null;
const wardrobeWorld=process.env.FLUCTLIGHT_UI_FIXTURE_WARDROBE_WORLD==='1'?wardrobeWorldFixture({fid:instance,owner,date,scenario:process.env.FLUCTLIGHT_UI_FIXTURE_WARDROBE}):null;
const clone=value=>JSON.parse(JSON.stringify(value));
const instances=[{id:instance,status:'active',identity:{name:'林岚 · 验收样本',age:26,timezone:'Asia/Shanghai'},unread_count:2,last_conversation_at:date},{id:'fluctlight-fixture-two',status:'paused',identity:{name:'季川 · 验收样本',timezone:'Asia/Shanghai'},unread_count:0,last_conversation_at:date}];
if(libraryScenario==='empty')instances.length=0;
const groups=[{id:'group-fixture',name:'默认',actor_ids:instances.map(value=>value.id),owner_actor_id:owner}];
let revision=1,factsRevision='facts-fixture-1',contextVersion='context-fixture-1';
let background={name:'验收用户',occupation:'编辑',background:'这是隔离验收环境中的样本资料。',interests:'阅读、绘画',preferences:'安静的交流',location_scope:'同城',location:'上海',timezone:'Asia/Shanghai',relationship_distance:'线上交流',meeting_confirmed:null};
const waveform={'1':{class_type:'ExampleModel',inputs:{prompt:'{{prompt}}',seed:'{{seed}}'}}},identityWorkflow={'2':{class_type:'ExampleIdentity',inputs:{prompt:'{{prompt}}'}}};
let settings={values:{kev:{enabled:true,media_enabled:true,endpoint:'http://fixture-service.invalid/v1/systemone',points:{'runtime.wakeup':true,'runtime.reflection':true,'goal.completion_check':true,'goal.replenish_plan':true,'tools.select':true,'persona.switch':true,'context.select':true,'media.subject_route':true,'media.capture_plan':true,'media.view_plan':true},timeout_ms:60000,budget_ms:120000,strategy:'choice_argmax',max_deferrals:3,retention_days:7,model_revision:'fixture'},'media.comfyui':{baseUrl:'http://fixture-service.invalid:8188',workflow:waveform,visual_identity_workflow:identityWorkflow,media_workflows:{back:{keep_branch:true}},visual_identity_workflows:{sheet:{keep_identity_branch:true}},custom_preserved:{owner:'fixture'}},'product.autonomy':{mode:'active',budget_remaining:1000,allowed_actions:['proactive_message','moment','memory_candidate'],quiet_hours:{from:'22:00',to:'08:00'},custom_preserved:'keep'},'product.wakeup':{enabled:true,interval_seconds:600},'llm.queue':{generated_concurrency:1,embedding_concurrency:1},'diagnostics.retention':{retention_days:14,max_rows:5000}},versions:{kev:7},configuredSecrets:[]};
let endpoints=[{id:'fixture-model',kind:'openai-compatible',base_url:'http://fixture-model.invalid/v1',secret_configured:process.env.FLUCTLIGHT_UI_FIXTURE_R16==='1',capability_status:'available',roles:[]}];
let bindings=[{role:'generic_llm',endpoint_id:'fixture-model',model_id:'fixture-model',token_budget:40960,timeout_seconds:300,endpoint_status:'available'},{role:'embedding',endpoint_id:'fixture-model',model_id:'fixture-embedding',token_budget:4096,timeout_seconds:60,endpoint_status:'available'}];
let goals=[{id:'goal-fixture-one',desired_outcome:'完成一幅新的水彩作品',description:'完成一幅新的水彩作品',motivation:'把观察到的街景画下来。',status:'active',criteria_version:1,revision:1,scope:'general',success_criteria:['完成并保存画作'],criterion_ids:['criterion-fixture'],deadline_policy:'soft',execution:{next_step:'选择构图',wait_condition:'有一段安静的时间',blocker:'',evaluation_request:{status:'pending'}},stages:[],commitments:[],evaluations:[],evidence:[],reviews:[],resolutions:[]},{id:'goal-fixture-two',desired_outcome:'根据真实偏好推荐一本小说',description:'根据真实偏好推荐一本小说',motivation:'分享有趣的阅读体验。',status:'active',criteria_version:1,revision:1,scope:'general',success_criteria:['完成推荐'],criterion_ids:['criterion-fixture-two'],deadline_policy:'soft',execution:{},stages:[],commitments:[],evaluations:[],evidence:[],reviews:[],resolutions:[]}];
let goalSet={revision:1,facts_revision:factsRevision,active_count:2,max_active_goals:5,auto_planning_enabled:true,ordering_mode:'automatic',capacity_violation:false,goals,dependencies:[],source_reviews:[]};
const planningReplay=process.env.FLUCTLIGHT_UI_FIXTURE_GOAL_PLANNING?goalPlanningFixture({scenario:process.env.FLUCTLIGHT_UI_FIXTURE_GOAL_PLANNING}):null;
const goalScenario=process.env.FLUCTLIGHT_UI_FIXTURE_GOALS;
const goalFixture=goalScenario?goalsFixture({goals,owner,scenario:goalScenario,date}):null;
const diaryScenario=process.env.FLUCTLIGHT_UI_FIXTURE_DIARY;
const diaryReplay=diaryScenario?diaryFixture({owner,scenario:diaryScenario}):null;
const growthScenario=process.env.FLUCTLIGHT_UI_FIXTURE_GROWTH;
const growthReplay=growthScenario?growthFixture({owner,scenario:growthScenario}):null;
const worldReplay=process.env.FLUCTLIGHT_UI_FIXTURE_WORLD?worldFixture({owner,scenario:process.env.FLUCTLIGHT_UI_FIXTURE_WORLD,autoRuns:true}):null;
const conversationId='conversation-fixture';
let messages=[{id:'message-fixture-one',conversationId,sequence:1,authorActorId:instance,kind:'assistant',text:'这是一条用于版式验收的示例消息。所有内容均为隔离样本。',attachmentRefs:[],createdAt:date,senderTimezone:'Asia/Shanghai',senderUtcOffsetMinutes:480,senderSentAt:date},{id:'message-fixture-two',conversationId,sequence:2,authorActorId:owner,kind:'user',text:'请查看回复失败时的反馈。',attachmentRefs:[],createdAt:date,senderTimezone:'Asia/Shanghai',senderUtcOffsetMinutes:480,senderSentAt:date,turnId:'turn-fixture-failed',idempotencyKey:'key-fixture-failed',turnStatus:'failed',turnErrorCode:'agent_run_failed',turnRetryable:true}];
if(nasAdditions){messages.at(-1).turnErrorCode=pendingChat?undefined:'prompt_tool_result_budget_exceeded';messages.at(-1).turnStatus=pendingChat?'running':'failed';messages.at(-1).turnRetryable=!pendingChat;messages.at(-1).text='请画一张海边水彩。';}
if(chatScenario==='empty')messages=[];
if(chatScenario==='normal')messages.at(-1).turnStatus='completed';
if(chatScenario==='pending'){messages.at(-1).turnStatus='running';messages.at(-1).turnRetryable=false;}
if(diaryReplay)messages.push({id:'shared-fixture-message',conversationId,sequence:3,authorActorId:owner,kind:'user',text:'一起分享一段正在读的故事',attachmentRefs:[],createdAt:'2026-10-26T12:00:00Z'});
let moments=[{id:'moment-fixture',owner_fluctlight_id:instance,author_actor_id:instance,text:'今天试着画了一盏路灯。这里的内容仅用于界面与交互验收。',created_at:date,media_asset_ids:[],media:[],status:'visible',comments:[{id:'comment-fixture',author_actor_id:owner,text:'期待你的新作品。',created_at:date}],reaction_count:1,viewer_reaction:null,unread_count:0}];
const modelRun={id:'model-fixture',role:'generic_llm',bindingRole:'generic_llm',scenario:'wake_up',modelId:'fixture-model',status:'failed',errorCode:'provider_request_failed',correlationId:'correlation-fixture',logicalRunId:'logical-fixture',sequence:1,roundCount:1,stage:'failed',prompt:[{role:'user',content:'这是样本 Prompt。'}],response:null,tools:[{type:'function',function:{name:'example.tool',parameters:{type:'object'}}}],createdAt:date,queuedAt:date,startedAt:date,completedAt:date};
const agentRun={fluctlightId:instance,agentId:'wake_up',runId:'agent-fixture',correlationId:'correlation-fixture',associationStatus:'linked',status:'failed',failureStage:'model',failureCode:'provider_request_failed',safeCause:'Fixture: provider unavailable',startedAt:date,finishedAt:date};
let events=[{id:'event-fixture',eventType:'lifecycle.wake_up.completed_noop',severity:'info',correlationId:'correlation-fixture',createdAt:date,payload:{occurred_at:date,reason_code:'superseded_by_cognition',stage:'activity',status:'cancelled',run_id:'run-fixture',nested:{text:'<script>escaped fixture text</script>'}}}];
let workflows=[{workflow_id:`go:wake_up:${instance}`,run_id:'workflow-run-new',status:'completed',start_time:date,close_time:date},{workflow_id:`go:wake_up:${instance}`,run_id:'workflow-run-old',status:'failed',start_time:'2026-10-12T01:00:00Z',close_time:'2026-10-12T01:05:00Z'}];
const wardrobe=[{id:'wardrobe-fixture',category:'sneakers',slot:'shoes',description:'白色运动鞋',ownership:'owned',availability:'available',source_kind:'initialization',revision:1}];
if(nasAdditions)Object.assign(wardrobe[0],{fluctlight_id:instance,category:'outdoor_clothing',slot:'outerwear',description:'海边散步的防风外套',ownership:'unknown',item_kind:'wearable',source_verified:true});
const scheduleFixture=()=>{
 const day=new Date(Date.now()+8*3600000).toISOString().slice(0,10),next=new Date(Date.parse(day+'T00:00:00Z')+86400000).toISOString().slice(0,10);
 const hours=[0,2,4,6,8,10,12,14,16,18,20,22,23,24];
 const stamp=hour=>hour===24?next+'T00:00:00+08:00':day+'T'+String(hour).padStart(2,'0')+':00:00+08:00';
 return {id:'schedule-nas-fixture',revision:1,local_date:day,timezone:'Asia/Shanghai',items:hours.slice(0,-1).map((hour,index)=>({id:'slot-'+index,start_at:stamp(hour),end_at:stamp(hours[index+1]),activity:index===11?'海边散步':index===12?'睡眠':'自由安排',scene:index===11?'海边':'日常',location:index===11?'家':'',status:'scheduled',action_status:null}))};
};
const scheduleReplay=scheduleReplayFixture({owner,nowSchedule:scheduleFixture,scenario:scheduleScenario,lifeScenario});
const reactWorkspace=reactWorkspaceFixture({settings:()=>settings,endpoints:()=>endpoints,bindings:()=>bindings,schedule:()=>lifeScenario==='empty'?{...scheduleFixture(),items:[]}:scheduleFixture(),owner,date});
const requests=[];let kevTests=0,goalCounter=2;
function detail(id){const value={id,status:instances.find(value=>value.id===id)?.status ?? 'active',identity:{...instances.find(value=>value.id===id)?.identity,background_story:'这是用于资料分层验收的角色背景。',occupation:'书籍编辑',core_values:['诚实','尊重'],height:null,height_cm:null,custom_field:'扩展字段仍可查看'},personality:{openness:.82,conscientiousness:.72,agreeableness:.74,update_policy:{cooldown_seconds:86400}},behavioral_policy:{reply_style:'自然交谈',conflict_style:'direct',directness:.7},life_profile:{life_habits:'闲暇时阅读与绘画。',appearance:{description:'齐肩长发，穿着简单舒适。',physical_features:{hair_color:'black'},daily_outfit_preferences:['白衬衫','深色长裤'],wardrobe_items:wardrobe},preferences:{likes:['小说','水彩']}},core_persona:{identity:{name:'验收样本'},personality_system:{profiles:[{id:'sunny',name:'晴岚'}]}},developing_self:{claims:[]},current_state:{appearance:{worn_items:wardrobe}},inner_state:{mood:{label:'平静',intensity:.5},pad:{pleasure:.6,arousal:.4,dominance:.5},momentum:{value:.5,trend:'stable'},regulation:{stability:.7,stress:.2}},context:{scene:'书桌前',activity:'阅读',location:'家中',source:'event'},actor_user:{background:clone(background),facts:[]},current_facts_revision:factsRevision,life_context_revision:'life-fixture-1',editable_actor_ids:[owner],relationships:[{id:'relationship-fixture',target_actor_id:owner,target_actor_type:'human',is_current_user:true,role:{label:'朋友'},trend:'stable',summary:'样本关系',revision:1}],memories:[{id:'memory-fixture',content:'这是一条用于只读列表验收的样本记忆。',type:'episodic',status:'active',revision:1}],goals:clone(goals),intentions:[],events:[],wake_ups:[],cognition_history:[],foundation_revisions:[{id:'foundation-fixture',revision:0,source:'initialization',status:'accepted'}],visual_identity:{status:'missing',current_revision:0,timeline:[],renderer_constraints:{chest_cup:null}},schedule:lifeScenario==='empty'?null:scheduleFixture()};if(lifeScenario==='empty'){value.context={};value.inner_state={};}if(wardrobeWorld)value.world={id:'fixture-world',name:'温和魔法世界'};value.schedule=scheduleReplay.schedule(id);value.context.context_revision=scheduleReplay.context(id);value.life_context_revision=scheduleReplay.context(id);return value;}
const profileReplay=process.env.FLUCTLIGHT_UI_FIXTURE_PROFILE?profileFixture({owner,scenario:process.env.FLUCTLIGHT_UI_FIXTURE_PROFILE,baseDetail:detail,onIdentity:(id,identity)=>{const row=instances.find(value=>value.id===id);if(row)row.identity=clone(identity);}}):null;
const peopleReplay=process.env.FLUCTLIGHT_UI_FIXTURE_PEOPLE?peopleFixture({owner,scenario:process.env.FLUCTLIGHT_UI_FIXTURE_PEOPLE,baseDetail:id=>profileReplay?profileReplay.state(id).detail:detail(id),worldState:worldReplay?fid=>worldReplay.state(fid):undefined,externalMemories:!!growthReplay}):null;
const mediaReplay=process.env.FLUCTLIGHT_UI_FIXTURE_MEDIA?mediaFixture({owner,defaultFid:instance,scenario:process.env.FLUCTLIGHT_UI_FIXTURE_MEDIA,externalWorld:!!worldReplay,externalInventory:!!wardrobeWorld,onAvatar:(fid,value)=>{const row=instances.find(c=>c.id===fid);if(row)row.avatar=value.avatar;if(profileReplay)profileReplay.state(fid).detail.avatar=value.avatar;}}):null;
const creationReplay=process.env.FLUCTLIGHT_UI_FIXTURE_CREATION?creationHttpFixture({owner,scenario:process.env.FLUCTLIGHT_UI_FIXTURE_CREATION,instances,groups,baseDetail:detail}):null;
const feedReplay=process.env.FLUCTLIGHT_UI_FIXTURE_FEED?feedFixture({owner,fid:instance,scenario:process.env.FLUCTLIGHT_UI_FIXTURE_FEED}):null;
const studioReplay=process.env.FLUCTLIGHT_UI_FIXTURE_STUDIO?studioFixture({owner,fid:instance,scenario:process.env.FLUCTLIGHT_UI_FIXTURE_STUDIO,autoJobs:true,externalWorld:!!worldReplay,worldRuntime:process.env.FLUCTLIGHT_UI_FIXTURE_STUDIO_RUNTIME==='1'?worldReplay?.studioWorld:undefined}):null;
if(mediaReplay&&feedReplay&&mediaReplay.assets.has('replica-landscape'))mediaReplay.assets.set('feed-image',{...mediaReplay.assets.get('replica-landscape'),id:'feed-image',title:'动态中的窗边阅读',source_filename:'feed-image.png',url:'/api/media/feed-image',version:'version-feed-image'});
function json(res,value,status=200){res.writeHead(status,{'content-type':'application/json; charset=utf-8','cache-control':'no-store'});res.end(JSON.stringify(value));}
async function telemetry(){await writeFile(telemetryPath,JSON.stringify({requests,settings,background,goalSet,messages},null,2));}
function page(id=instance){return {conversation:{id:conversationId,createdByActorId:owner,title:'隔离样本会话',revision:1,createdAt:date,updatedAt:date},participants:[{conversationId,actorId:owner,role:'owner',status:'active',joinedAt:date},{conversationId,actorId:id,role:'member',status:'active',joinedAt:date}],messages:clone(messages),nextBeforeSequence:null};}
const server=http.createServer(async(req,res)=>{
 try{
  const url=new URL(req.url,'http://127.0.0.1'),path=url.pathname,method=req.method ?? 'GET';
  let body={};if(!['GET','HEAD'].includes(method)){let raw='';for await(const chunk of req)raw+=chunk;if(raw)body=JSON.parse(raw);requests.push({method,path,query:Object.fromEntries(url.searchParams),body:{...body,...(body.secrets ? {secrets:'fixture-redacted'} : {})}});}
  else if(path.startsWith('/api/'))requests.push({method,path,query:Object.fromEntries(url.searchParams)});
  if(parityReplay&&await parityReplay.handler({path,method,body,url,json,res}))return;
  if(managementReplay&&await managementReplay.handler({path,method,body,url,json,res}))return;
  if(creationReplay&&await creationReplay.handler({path,method,body,url,json,res})){
    if(process.env.FLUCTLIGHT_UI_FIXTURE_CREATION_AUDIT)await writeFile(process.env.FLUCTLIGHT_UI_FIXTURE_CREATION_AUDIT,JSON.stringify({calls:creationReplay.calls,records:[...creationReplay.records.values()]},null,2));
    return;
  }
  if(studioReplay&&await studioReplay.handler({path,method,body,url,json,res})){
    if(process.env.FLUCTLIGHT_UI_FIXTURE_STUDIO_AUDIT)await writeFile(process.env.FLUCTLIGHT_UI_FIXTURE_STUDIO_AUDIT,JSON.stringify({calls:studioReplay.calls,resources:[...studioReplay.resources.values()],jobs:[...studioReplay.jobs.values()],materials:[...studioReplay.materials.values()]},null,2));
    return;
  }
  if(mediaReplay&&await mediaReplay.handler({path,method,body,url,json,res}))return;
  if(mediaReplay&&path.startsWith('/api/media/')&&mediaReplay.assets.has(decodeURIComponent(path.split('/')[3]))){const asset=mediaReplay.assets.get(decodeURIComponent(path.split('/')[3]));if(!asset.readable)return json(res,{code:'media_unavailable'},404);res.writeHead(200,{'content-type':'image/svg+xml','cache-control':'no-store'});res.end('<svg xmlns="http://www.w3.org/2000/svg" width="520" height="430" viewBox="0 0 520 430"><rect width="520" height="430" fill="#e5ecdd"/><rect x="50" y="35" width="145" height="230" rx="6" fill="#f8f6e9"/><path d="M122 35v230M50 150h145" stroke="#c5d3bc" stroke-width="7"/><ellipse cx="310" cy="360" rx="125" ry="85" fill="#8b9e80"/><ellipse cx="308" cy="190" rx="77" ry="94" fill="#c5b69c"/><path d="M230 188q-18-135 84-125q100 6 74 139l-24-54q-80 12-112-20Z" fill="#596654"/><text x="30" y="407" fill="#6f7d63" font-size="11">ISOLATED UI FIXTURE · '+String(asset.title).replace(/[<>&]/g,'')+'</text></svg>');return;}
  if(feedReplay&&await feedReplay.handler({path,method,body,url,json,res}))return;
  if(feedReplay&&path==='/api/media/feed-image'){res.writeHead(200,{'content-type':'image/svg+xml'});res.end('<svg xmlns="http://www.w3.org/2000/svg" width="760" height="265" viewBox="0 0 760 265"><rect width="760" height="265" fill="#e8ecdc"/><rect x="60" y="25" width="160" height="145" rx="8" fill="#f5f6ec"/><path d="M140 25v145M60 97h160" stroke="#c2d0b8" stroke-width="8"/><path d="M0 205h760v60H0" fill="#d3c4a5"/><rect x="490" y="145" width="80" height="60" rx="5" fill="#809578"/><path d="M540 145q-80-100-45-100q50 5 45 100M540 145q60-100 90-90q0 50-90 90" fill="#9aaa88"/><text x="280" y="242" fill="#6b785f" font-size="14">ISOLATED UI FIXTURE · READING MOMENT</text></svg>');return;}
  if(r16?.({path,method,body,url,json,res}))return;
  if(peopleReplay&&await peopleReplay.handler({path,method,body,url,json,res}))return;
  if(worldReplay&&await worldReplay.handler({path,method,body,url,json,res}))return;
  // The mutable wardrobe owns inventory reads and writes in composed scenes.
  // Profile's static wardrobe is only a fallback for other characters.
  if(wardrobeWorld&&await wardrobeWorld({path,method,body,url,json,res}))return;
  if(profileReplay&&await profileReplay.handler({path,method,body,url,json,res}))return;
  if(/^\/api\/fluctlights\/[^/]+\/schedules$/.test(path)){
   lifeScheduleReads++;
   if(lifeScenario==='loading'&&lifeScheduleReads===1)await new Promise(resolve=>setTimeout(resolve,3000));
   if(lifeScenario==='failure'&&lifeScheduleReads===1)return json(res,{code:'request_failed',message:'日程暂时无法读取'},503);
  }
  if(scheduleReplay.handler({path,method,body,url,json,res}))return;
  if(planningReplay&&await planningReplay.handle({path,method,body,json,res}))return;
  if(goalFixture&&await goalFixture({path,method,body,url,json,res}))return;
  if(diaryReplay&&await diaryReplay({path,method,body,url,json,res}))return;
  if(growthReplay&&await growthReplay({path,method,body,url,json,res}))return;
  if(reactWorkspace({path,method,body,url,json,res}))return;
  if(path==='/fixture/status')return json(res,{requests,settings,background,goals});
  if(path==='/runtime-config.js'){res.writeHead(200,{'content-type':'text/javascript; charset=utf-8','cache-control':'no-store'});res.end('window.__FLUCTLIGHT_RUNTIME_CONFIG__={apiOrigin:window.location.origin};');return;}
  if(path==='/auth/session'||path==='/auth/login')return json(res,{authenticated:true,actorId:owner});
  if(path==='/auth/setup-status')return json(res,{setupAvailable:false});
  if(path==='/health/live'||path==='/health/ready')return json(res,{status:'ok'});
  if(path==='/api/fluctlights'&&method==='GET')return json(res,instances);
  if(path==='/api/actor-groups')return json(res,groups);
  if(path==='/api/settings'&&method==='GET'){await new Promise(resolve=>setTimeout(resolve,150));return json(res,settings);}
  if(path==='/api/settings'&&method==='PUT'){settings.values={...settings.values,...clone(body.values ?? {})};settings.versions.kev++;await telemetry();return json(res,settings);}
  if(path==='/api/settings/kev/test-connection')return json(res,++kevTests===1?{call_status:'unavailable',error_code:'request_failed'}:{call_status:'ok'});
  if(path==='/api/providers')return json(res,bindings);
  if(path==='/api/providers/endpoints'&&method==='GET')return json(res,endpoints);
  if(path==='/api/providers/endpoints'&&method==='PUT'){endpoints=[...endpoints.filter(value=>value.id!==body.endpointId),{id:body.endpointId,kind:body.kind,base_url:body.baseUrl,secret_configured:false,capability_status:'available',roles:[]}];bindings=bindings.filter(value=>value.endpoint_id!==body.endpointId);await telemetry();return json(res,endpoints);}
  if(path==='/api/providers/roles'&&method==='PUT'){bindings=[...bindings.filter(value=>value.role!==body.role),{role:body.role,endpoint_id:body.endpointId,model_id:body.modelId,token_budget:body.tokenBudget,timeout_seconds:body.timeoutSeconds,endpoint_status:'available'}];await telemetry();return json(res,bindings);}
  if(/^\/api\/providers\/endpoints\/[^/]+\/models$/.test(path))return json(res,{models:['fixture-model','fixture-embedding']});
  if(path==='/api/fluctlight-creations/activate'){const id=`fluctlight-fixture-${instances.length+1}`;instances.push({id,status:'active',identity:{name:body.name || '新建验收样本',timezone:'Asia/Shanghai'},unread_count:0});groups[0].actor_ids.push(id);await telemetry();return json(res,{id,status:'active'});}
  if(path==='/api/fluctlight-creations/analysis')return json(res,{code:'analysis_unavailable',message:'Fluctlight analysis service is unavailable',details:{correlation_id:'analysis-fixture'}},503);
  if(/^\/api\/fluctlights\/[^/]+\/conversation$/.test(path)){
   conversationReads++;
   if(chatScenario==='loading'&&conversationReads===1)await new Promise(resolve=>setTimeout(resolve,3000));
   if(chatScenario==='read-failure'&&conversationReads===1)return json(res,{code:'request_failed',message:'对话暂时无法读取'},503);
   return json(res,page(path.split('/')[3]));
  }
  if(/^\/api\/fluctlights\/[^/]+\/conversations$/.test(path)&&method==='GET'){
   if(chatScenario==='history-failure'&&++historyReads===1)return json(res,{code:'request_failed',message:'会话历史暂时无法读取'},503);
   return json(res,{items:chatScenario==='empty'?[]:[{conversation:page().conversation,unreadCount:0,lastReadSequence:2,lastDeliveredSequence:2}],currentConversationId:chatScenario==='empty'?'':conversationId,nextCursor:''});
  }
  if(path.endsWith('/messages/search'))return json(res,{items:messages.filter(value=>value.text.includes(url.searchParams.get('q')||'')),nextCursor:''});
  if(/^\/api\/fluctlights\/[^/]+\/conversations\/[^/]+\/select$/.test(path)&&method==='POST')return json(res,page(path.split('/')[3]));
  if(path===`/api/conversations/${conversationId}/messages`)return json(res,page());
  if(path.startsWith(`/api/conversations/${conversationId}/messages/`)){const message=messages.find(row=>row.id===decodeURIComponent(path.split('/').at(-1)));return json(res,message||{code:'not_found',message:'当前会话没有此消息'},message?200:404);}
  if(path.endsWith('/read'))return json(res,{});
  if(path.endsWith('/turn')&&method==='POST'){
   const nextSequence=Math.max(0,...messages.map(value=>value.sequence))+1,turnId=body.turnId ?? `turn-fixture-${nextSequence}`;
   const accepted=messages.find(value=>value.turnId===turnId&&value.kind==='user');
   const seq=accepted?.sequence ?? nextSequence;
   messages=messages.filter(value=>value.turnId!==turnId);
   const user={...accepted,id:accepted?.id ?? `message-fixture-${seq}`,conversationId,sequence:seq,authorActorId:owner,kind:'user',text:body.text,attachmentRefs:body.attachmentRefs ?? [],createdAt:date,senderTimezone:body.senderTimezone,senderUtcOffsetMinutes:body.senderUtcOffsetMinutes,senderSentAt:body.senderSentAt,turnId,idempotencyKey:body.idempotencyKey,turnStatus:'completed',turnRetryable:false};
   const reply={id:`message-fixture-${seq+1}`,conversationId,sequence:seq+1,authorActorId:instance,kind:'assistant',text:'这是隔离验收服务的模拟回复。',attachmentRefs:[],createdAt:date};messages.push(user,reply);await telemetry();
   res.writeHead(200,{'content-type':'application/x-ndjson'});
   res.write(JSON.stringify({type:'token',turnId,sequence:0,payload:{text:'这是隔离验收服务的'}})+'\n');
   await new Promise(resolve=>setTimeout(resolve,500));
   res.write(JSON.stringify({type:'token',turnId,sequence:1,payload:{text:'模拟回复。'}})+'\n');
   await new Promise(resolve=>setTimeout(resolve,500));
   res.end(JSON.stringify({type:'completed',turnId,sequence:2,payload:{message:reply,messages:[reply]}})+'\n');return;
  }
  if(/^\/api\/fluctlights\/[^/]+\/detail$/.test(path)){
   libraryDetailReads++;
   if(libraryScenario==='slow')await new Promise(resolve=>setTimeout(resolve,2500));
   if(libraryScenario==='failure'&&libraryDetailReads===1)return json(res,{code:'request_failed',message:'角色资料暂时无法读取'},503);
   return json(res,detail(path.split('/')[3]));
  }
  if(path.includes('/actor-context/')){if(method==='GET' && injectFailures && ++actorReads===1)return json(res,{code:'request_failed',message:'Fixture read unavailable'},503);if(method==='PUT'){if(body.expectedContextVersion!==contextVersion)return json(res,{code:'revision_conflict',message:'资料已变化'},409);background={...background,...body.background};contextVersion=`context-fixture-${++revision}`;await telemetry();}return json(res,{actor_id:path.split('/').at(-1),context_version:contextVersion,background:clone(background),facts:[],history:[],relationships:[],relationship_history:[]});}
  if(path.endsWith('/actor-user-background')&&method==='PUT'){if(body.expectedCurrentFactsRevision!==factsRevision)return json(res,{code:'revision_conflict',message:'资料已变化'},409);background={...background,...body.background};factsRevision=`facts-fixture-${++revision}`;await telemetry();return json(res,{background:clone(background),current_facts_revision:factsRevision,facts:[]});}
  if(path.endsWith('/goal-set')){const ordered=goalSet.goals.map(value=>goals.find(goal=>goal.id===value.id)).filter(Boolean);goalSet.goals=[...ordered,...goals.filter(goal=>!ordered.some(value=>value.id===goal.id))];goalSet.facts_revision=factsRevision;goalSet.active_count=goals.filter(value=>value.status==='active').length;if(method==='PUT'){goalSet={...goalSet,revision:goalSet.revision+1,...(body.autoPlanningEnabled!==undefined?{auto_planning_enabled:body.autoPlanningEnabled}:{}),...(body.orderingMode?{ordering_mode:body.orderingMode}:{}),...(body.order?{goals:body.order.map(id=>goals.find(value=>value.id===id))}:{}),...(body.dependencies?{dependencies:Object.entries(body.dependencies).flatMap(([goalId,ids])=>ids.map(id=>({goal_id:goalId,prerequisite_id:id,satisfied:false})))}:{})};await telemetry();}return json(res,goalSet);}
  if(path.endsWith('/goal-planning'))return json(res,method==='GET'?[{id:'plan-fixture',created_at:date,status:'succeeded',mode:'apply',result:{decision:'no_change',reason:'当前目标仍然有效。',review_condition:'新的事实出现后复核。'}}]:{});
  if(path.endsWith('/goals')){if(method==='POST'){const id=`goal-fixture-${++goalCounter}`;goals.push({id,desired_outcome:body.desiredOutcome,motivation:body.motivation,scope:body.scope,status:body.candidateOnly?'candidate':'active',success_criteria:body.successCriteria,criterion_ids:body.successCriteria.map((_,index)=>`criterion-${index}`),revision:1,criteria_version:1,deadline_policy:body.deadlinePolicy,execution:{},stages:[],commitments:[],evaluations:[],evidence:[],reviews:[]});await telemetry();return json(res,{goal_id:id,revision:1,status:body.candidateOnly?'candidate':'active'});}return json(res,{items:clone(goals.filter(goal=>['completed','cancelled','abandoned'].includes(goal.status)===(url.searchParams.get('history')==='true'))),next_cursor:''});}
  if(/\/goals\/[^/]+\/history$/.test(path))return json(res,{items:[{id:'history-fixture',created_at:date,source:'owner',operation:'create',reason:'验收样本'}],next_cursor:''});
  if(/\/goals\/[^/]+\/evidence$/.test(path))return json(res,{items:[],next_cursor:''});
  if(/\/goals\/[^/]+$/.test(path)){const id=path.split('/').at(-1),goal=goals.find(value=>value.id===id) ?? goals[0];if(method==='PUT'){if(body.expectedRevision!==goal.revision)return json(res,{code:'revision_conflict',message:'目标版本已变化'},409);goal.revision++;goal.desired_outcome=body.desiredOutcome;goal.motivation=body.motivation;goal.success_criteria=body.successCriteria;await telemetry();return json(res,{goal_id:goal.id,revision:goal.revision,status:goal.status});}return json(res,clone(goal));}
  if(/\/goals\/[^/]+\/[^/]+$/.test(path)&&method==='POST'){const id=path.split('/').at(-2),goal=goals.find(value=>value.id===id);if(goal){goal.revision++;if(body.desiredOutcome)goal.desired_outcome=body.desiredOutcome;}await telemetry();return json(res,{goal_id:id,revision:goal?.revision ?? 1,status:goal?.status ?? 'active'});}
  if(nasAdditions&&path.endsWith('/promises'))return json(res,{items:[],next_cursor:''});
  if(nasAdditions&&path.endsWith('/inventory/outfits'))return json(res,{profile_id:'profile-fixture',items:[],next_cursor:''});
  if(nasAdditions&&path.endsWith('/inventory/wearing-history'))return json(res,{items:[],next_cursor:''});
  if(nasAdditions&&path.endsWith('/inventory'))return json(res,{revision:1,items:wardrobe,worn_items:[],used_items:[],inventory_complete:false,next_cursor:''});
  if(nasAdditions&&path==='/api/owner-requests/request-nas-fixture')return json(res,{id:'request-nas-fixture',fluctlight_id:instance,kind:'moment_suggestion',status:'failed',created_at:date,revision:1,decision_reason:'The request did not produce a committed result',result:{error_code:'owner_life_assessment_failed',mutation_committed:false},tasks:[],result_refs:[]});
  if(path.endsWith('/wardrobe'))return json(res,{worn_items:[],used_items:[],operation:'list',revision:1,inventory_complete:true,items:wardrobe,has_more:false,next_cursor:'',searched_scope:'all',can_conclude_absent:true});
  if(path==='/api/moments'||/\/fluctlights\/[^/]+\/moments$/.test(path))return json(res,moments);
  if(/\/moments\/[^/]+\/comments$/.test(path)&&method==='POST'){moments[0].comments.push({id:`comment-${moments[0].comments.length}`,author_actor_id:owner,text:body.text,created_at:date});await telemetry();return json(res,{});}
  if(path==='/api/diagnostics'&&method==='DELETE'){if(injectFailures && ++diagnosticClears===1){await telemetry();return json(res,{code:'request_failed',message:'Fixture delete unavailable'},503);}events=[];await telemetry();return json(res,{cleared:1});}
  if(path==='/api/diagnostics')return json(res,events);
  if(path==='/api/diagnostics/agent-runs')return json(res,{items:[agentRun],nextCursor:'',snapshot:'fixture'});
  if(path==='/api/diagnostics/model-runs')return json(res,{items:[modelRun],nextCursor:'',snapshot:'fixture'});
  if(path==='/api/diagnostics/media-prompts')return json(res,[{id:'media-fixture',mediaIntentId:'media-intent-fixture',fluctlightId:instance,kind:'image',mimeType:'image/png',prompt:{text:'Sample'},providerPrompt:'A sample painting.',qualityVerdict:'retry_accepted',status:'completed',correlationId:'correlation-fixture',createdAt:date,requestPayload:waveform,modelRun}]);
  if(path==='/api/diagnostics/lifecycle')return json(res,{events:[{id:'lifecycle-fixture',surface:'wake_up',transition:'completed_noop',stage:'activity',status:'no_op',reasonCode:'superseded_by_cognition',correlationId:'correlation-fixture',workflowId:workflows[0].workflow_id,createdAt:date,occurredAt:date}],workflowIntents:[],filters:{}});
  if(path==='/api/diagnostics/kev-decisions'){const cursor=url.searchParams.get('cursor'),row={id:cursor?'kev-fixture-older':'kev-fixture',decision_point:'tools.select',candidate_id:'sample.tool',started_at:date,request_id:'request-fixture',question_id:'question-fixture',call_status:'unavailable',policy_outcome:'use_original',application_status:'original_used',fallback_reason:'request_failed',request:{text:'样本原始数据'}};return json(res,{items:[row],next_cursor:cursor?'':'fixture-page-two'});}
  if(path==='/api/diagnostics/workflows')return json(res,workflows);
  if(path.endsWith('/status')&&path.includes('/diagnostics/workflows/'))return json(res,workflows[0]);
  if(path.endsWith('/history')&&path.includes('/diagnostics/workflows/'))return json(res,{events:[{event_id:1,event_type:'fixture'}]});
  if(path.endsWith('/autonomy-actions')||path==='/api/capability-requests')return json(res,[]);
  if(path.startsWith('/api/'))return json(res,{code:'fixture_route_missing',message:`Fixture route not implemented: ${method} ${path}`},404);
  let filename=path==='/'?'/index.html':path;
  if(!extname(filename))filename='/index.html';
  const absolute=resolve(root,'.'+filename);if(!absolute.startsWith(root+'/')){res.writeHead(403);res.end();return;}
  let data=await readFile(absolute);const type={'.html':'text/html; charset=utf-8','.js':'text/javascript; charset=utf-8','.css':'text/css; charset=utf-8','.svg':'image/svg+xml'}[extname(filename)] ?? 'application/octet-stream';
  if(filename==='/index.html')data=Buffer.from(data.toString().replace('<title>Fluctlight</title>','<title>Fluctlight · 隔离验收样本</title>'));
  res.writeHead(200,{'content-type':type,'cache-control':'no-store'});res.end(data);
 }catch(error){json(res,{code:'fixture_error',message:error.message},500);}
});
await writeFile(resolve(root,'runtime-config.js'),'window.__FLUCTLIGHT_RUNTIME_CONFIG__={apiOrigin:window.location.origin};\n');
server.listen(port,'127.0.0.1',()=>process.stdout.write(`Isolated UI fixture: http://127.0.0.1:${port}\n`));
