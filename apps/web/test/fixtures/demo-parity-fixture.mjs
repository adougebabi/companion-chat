// Same-content visual scenes only. Uses production HTTP DTOs and real components;
// this file and the extracted Demo seeds never enter the production import graph.
import {readFileSync} from 'node:fs';
import {parityDomains} from './demo-parity-domains.mjs';
const {data}=JSON.parse(readFileSync(new URL('./demo-parity-data.json',import.meta.url),'utf8'));
const clone=value=>JSON.parse(JSON.stringify(value));
export const parityClock='2026-10-27T07:45:00.000Z';
export function demoParityFixture({scenario='normal'}={}) {
 const owner='demo-parity-owner',calls=[],misses=[];
 const chars=clone(data.characters),timestamp='2026-10-27T07:44:00Z';
 const domains=parityDomains({data,owner,clock:parityClock,chars,clone});
 const noticeSeed=()=>clone(data.notices).map(n=>{
  const amount=Number(n.time?.match(/\d+/)?.[0]||0),age=n.time?.includes('分钟')?amount*60000:n.time?.includes('小时')?amount*3600000:n.time?.includes('昨天')?86400000:0;
  return {id:n.id,fluctlight_id:n.characterId,title:n.title,body:n.body,kind:n.type,created_at:new Date(Date.parse(parityClock)-age).toISOString(),read_at:n.read?timestamp:null,link:n.type==='task'?'/diag/task/'+n.sourceId:n.page==='profile'?'/c/'+n.characterId+'/profile':n.page==='goals'?'/c/'+n.characterId+'/life/goals':'/c/'+n.characterId+'/chat',quiet:false};
 });
 let notices=noticeSeed(),authMode='authenticated';
 const avatar=c=>c.id==='linwan'?{url:'/api/media/parity-avatar',asset_id:'parity-avatar'}:null;
 const directory=()=>scenario==='empty'?[]:chars.map(c=>({id:c.id,status:c.status,identity:{...c,tags:c.personality,background_story:c.background||c.tagline},avatar:avatar(c),last_conversation_at:({linwan:'2026-10-27T07:45:00Z',jiangyu:'2026-10-27T05:45:00Z',yuejian:'2026-10-26T07:45:00Z'})[c.id],unread_count:0}));
 const items=fid=>data.garments.map(g=>({id:g.id,fluctlight_id:fid,item_kind:'wearable',clothing_type:g.type,category:g.type,slot:g.type,description:g.name,seasons:g.seasons.map(s=>({春:'spring',夏:'summer',秋:'autumn',冬:'winter'})[s]),ownership:g.ownership,availability:g.availability,source_verified:true,source_kind:'initialization',revision:1,archived_at:null}));
 const wearing=fid=>({revision:1,items:items(fid),worn_items:items(fid).filter(g=>(fid==='linwan'?['g-cardigan','g-shirt','g-skirt','g-shoes']:['g-shirt','g-trousers']).includes(g.id)),used_items:[],next_cursor:'',inventory_complete:true});
 const schedule=(fid,day='2026-10-27')=>({id:'parity-schedule-'+fid+'-'+day,local_date:day,timezone:'Asia/Shanghai',revision:1,items:(data.schedules[fid]||[]).filter(s=>s.date===day).map(s=>({id:s.id,start_at:s.date+'T'+s.start+':00+08:00',end_at:s.date+'T'+s.end+':00+08:00',activity:s.activity,scene:s.scene,location:s.location,description:s.note,status:'scheduled',action_status:({done:'completed',current:'in_progress',planned:null})[s.status]}))});
 const goalRows=fid=>data.goals.map(g=>({id:g.id,fluctlight_id:fid,desired_outcome:g.title,description:g.title,motivation:g.motivation,status:g.status,owner_protected:false,scope:g.category==='关系'?'relationship':'general',progress:g.criteria.filter(c=>c.met).length/g.criteria.length,criteria_version:1,revision:1,success_criteria:g.criteria.map(c=>c.text),criterion_ids:g.criteria.map((_,i)=>g.id+'-'+i),deadline:g.deadline,deadline_policy:'soft',execution:{},stages:[],commitments:[],evaluations:[{id:'parity-evaluation-'+g.id,goal_id:g.id,criteria_version:1,judgments:g.criteria.map((c,i)=>({criterion_id:g.id+'-'+i,verdict:c.met?'satisfied':'not_satisfied',reason:'',evidence_refs:[]}))}],evidence:[],reviews:[],resolutions:[]}));
 const diaryRows=fid=>data.diaries.map(d=>({id:d.id,fluctlight_id:fid,date:d.date,timezone:'Asia/Shanghai',source:'agent',refs:[],latest_attempt:1,last_error_code:null,active_attempt_id:'parity-diary-'+d.id,attempts:[],has_more_attempts:false,body:d.content,status:'ready',revision:1,created_at:d.date+'T21:00:00+08:00',annotation:{note:d.annotation,favorite:false,revision:0},source_refs:[]}));
 const memoryRows=fid=>data.memories.map(m=>({id:m.id,owner_fluctlight_id:fid,content:m.content,type:({关系:'relationship',经历:'episodic',情景:'episodic',知识:'semantic',语义:'semantic',自我:'autobiographical'})[m.type]||'autobiographical',status:m.status,importance:m.importance/5,revision:m.revision,created_at:m.date+'T15:00:00+08:00',source_type:'conversation',tags:m.tags,visibility:'owner',provenance_status:'valid',actor_refs:[],event_refs:[],evidence_refs:[],confidence:.8,revision_count:0,source_count:0}));
 const snapshot=fid=>{
  const c=chars.find(c=>c.id===fid),world=data.worlds.find(w=>w.id===c.worldId),persona=data.personas[fid];
  return {id:fid,status:c.status,identity:{...c,tags:c.personality,background_story:c.background||c.tagline},avatar:avatar(c),core_persona:{identity:{...c,tags:c.personality,background_story:c.background||c.tagline},personality_system:{mode:'multiple',default_profile_id:persona.active,profiles:persona.profiles.map(p=>({...p,personality:{},behavioral_policy:{reply_style:p.tone}}))}},personality_runtime:{active_profile_id:persona.active,revision:1},personality:{openness:.72,conscientiousness:.78,extraversion:.4,agreeableness:.84,neuroticism:.32},behavioral_policy:{reply_style:'自然随和',conflict_style:'gentle',directness:.6},life_profile:{life_habits:'睡前阅读，午后喝一杯咖啡。',appearance:{description:'黑色长发，喜欢舒适自然的衣着。'},preferences:{likes:['读书','雨天','咖啡']}},developing_self:{claims:[]},context:{location:fid==='linwan'?data.runtime.location:c.worldName+' · 住处',activity:fid==='linwan'?data.runtime.activity:'整理今天的摄影作品',context_revision:'parity-context-1'},inner_state:{mood:{label:data.runtime.mood}},current_state:{appearance:{worn_items:wearing(fid).worn_items}},current_facts_revision:'facts_gen_1',life_context_revision:'parity-context-1',foundation_revisions:[],persona_switch_history:[],world:{id:world.id,name:world.name},schedule:schedule(fid),goals:goalRows(fid),memories:memoryRows(fid),...domains.profile(fid)};
 };
 const messages=fid=>(data.chats[fid]||[]).map((m,i)=>({id:m.id,conversationId:'parity-conversation-'+fid,sequence:i+1,authorActorId:m.role==='user'?owner:fid,kind:m.role,text:m.text,createdAt:'2026-10-27T'+m.time+':00+08:00',attachmentRefs:m.image?['parity-bookshop']:[],...(m.role==='user'?{turnId:'parity-turn-'+m.id,turnStatus:'completed'}:{})}));
 const page=fid=>({conversation:{id:'parity-conversation-'+fid,title:fid==='linwan'?'窗边的一场小雨':'今天的对话',createdAt:'2026-10-27T07:42:00Z',revision:1},participants:[{actorId:fid,role:'member'},{actorId:owner,role:'owner'}],messages:messages(fid),nextBeforeSequence:null});
 let initialReads=0;
 return {calls,misses,async handler({path,method,body,url,json,res}){
  if(path==='/fixture/parity-reset'){notices=noticeSeed();calls.length=0;misses.length=0;json(res,{reset:true});return true;}
  if(path==='/fixture/parity-auth'){authMode=url.searchParams.get('value')||'authenticated';json(res,{authMode});return true;}
  if(path==='/fixture/parity-status'){json(res,{scenario,clock:parityClock,calls,misses});return true;}
  if(path==='/runtime-config.js'){res.writeHead(200,{'content-type':'text/javascript','cache-control':'no-store'});res.end(`window.__FLUCTLIGHT_RUNTIME_CONFIG__={apiOrigin:window.location.origin};{const NativeDate=Date,now=NativeDate.parse(${JSON.stringify(parityClock)});window.Date=class extends NativeDate{constructor(...args){super(...(args.length?args:[now]));}static now(){return now;}};}`);return true;}
  if(path.startsWith('/auth/')){if(path.endsWith('setup-status')){json(res,{setupAvailable:authMode==='setup'});return true;}if(path.endsWith('login')||path.endsWith('setup')){if(authMode==='failure'){json(res,{error:'fixed-scene incorrect password'},401);return true;}authMode='authenticated';}json(res,{authenticated:authMode==='authenticated',...(authMode==='authenticated'?{actorId:owner}:{})});return true;}
  if(!path.startsWith('/api/'))return false;
  calls.push({path,method,query:Object.fromEntries(url.searchParams),...(method==='GET'?{}:{body:clone(body)})});
  const reply=value=>{json(res,clone(value));return true;};
  if(path==='/api/fluctlight-creations/analysis'&&method==='POST')return reply({...clone(body.json?.core_persona?body.json:data.creationFoundation),analysis_id:'initialization_source_'+('c'.repeat(32)),correlation_id:'parity-analysis'});
  if(path.startsWith('/api/media/parity-')){const kind=path.slice('/api/media/parity-'.length),svg=kind.startsWith('garment-')?data.svg.garments[kind.slice(8)]:data.svg[kind];if(svg){res.writeHead(200,{'content-type':'image/svg+xml','cache-control':'no-store'});res.end(svg.includes('xmlns=')?svg:svg.replace('<svg ','<svg xmlns="http://www.w3.org/2000/svg" '));return true;}}
  if(path==='/api/fluctlights')return reply(directory());
  if(path==='/api/actor-groups')return reply([...new Set(chars.map(c=>c.group))].map((name,i)=>({id:'parity-group-'+i,name,owner_actor_id:owner,actor_ids:chars.filter(c=>c.group===name).map(c=>c.id)})));
  if(path==='/api/configuration-catalog')return reply(domains.configuration);
  if(path==='/api/settings/agents')return reply({agents:[],capabilities:[]});
  if(path.startsWith('/api/providers/endpoints/')&&path.endsWith('/models'))return reply({models:data.settings.services.find(s=>s.id===path.split('/')[4])?.models||[]});
  if(path==='/api/notifications')return reply({items:notices.filter(n=>(!url.searchParams.get('fluctlightId')||n.fluctlight_id===url.searchParams.get('fluctlightId'))&&(url.searchParams.get('unread')!=='true'||!n.read_at)),as_of:parityClock,next_cursor:''});
  if(path==='/api/notifications/unread-count')return reply({count:notices.filter(n=>!n.read_at).length,alert_count:notices.filter(n=>!n.read_at&&!n.quiet).length});
  if(path==='/api/notifications/read-all'&&method==='POST'){let updated=0;for(const n of notices)if(!n.read_at&&Date.parse(n.created_at)<=Date.parse(body.before)){n.read_at=parityClock;updated++;}return reply({updated,before:body.before});}
  const noticeRead=path.match(/^\/api\/notifications\/([^/]+)\/read$/);
  if(noticeRead&&method==='POST'){const n=notices.find(n=>n.id===noticeRead[1]);if(!n){json(res,{code:'not_found'},404);return true;}n.read_at ||=parityClock;return reply({id:n.id,read_at:n.read_at});}
  if(path==='/api/pending/count')return reply({count:2});
  if(path==='/api/pending')return reply({groups:domains.pending(url.searchParams.get('fluctlightId')||'')});
  if(await domains.handle({path,method,url,reply}))return true;
  if(path.endsWith('/read')&&method==='POST')return reply({});
  const cm=path.match(/^\/api\/conversations\/parity-conversation-([^/]+)\/messages(?:\/([^/]+))?$/);
  if(cm)return reply(cm[2]?messages(cm[1]).find(m=>m.id===cm[2]):page(cm[1]));
  const memory=path.match(/^\/api\/memories\/([^/]+)(?:\/(sources|revisions))?$/);
  if(memory){const row=memoryRows('linwan').find(m=>m.id===memory[1]);if(row)return reply(memory[2]?{items:[],next_cursor:''}:row);}
  const match=path.match(/^\/api\/fluctlights\/([^/]+)(\/.*)?$/);
  if(match&&chars.some(c=>c.id===match[1])){
   const fid=match[1],end=match[2]||'';
   if(end==='/detail'||end===''){
    if(scenario==='loading'&&initialReads++===0)await new Promise(r=>setTimeout(r,3000));
    if(scenario==='failure'&&initialReads++===0){json(res,{code:'request_failed',message:'同场景读取暂时失败'},503);return true;}
    return reply(snapshot(fid));
   }
   if(end==='/conversation')return reply(page(fid));
   if(end==='/conversations')return reply({items:[{conversation:page(fid).conversation,unreadCount:0}],currentConversationId:page(fid).conversation.id,nextCursor:''});
   if(end==='/messages/search')return reply({items:messages(fid).filter(m=>m.text.includes(url.searchParams.get('q')||'')),nextCursor:''});
   if(end==='/wardrobe'||end==='/inventory')return reply(wearing(fid));
   if(end.startsWith('/inventory/items/')){const row=items(fid).find(i=>i.id===end.split('/')[3]);if(row)return reply(end.endsWith('/history')?{items:[],next_cursor:''}:row);}
   if(end==='/inventory/outfits')return reply({profile_id:'everyday',items:data.outfits.map(o=>({...o,fluctlight_id:fid,profile_id:'everyday',tags:[o.tag],items:items(fid).filter(g=>o.itemIds.includes(g.id))})),next_cursor:''});
   if(end==='/inventory/tags')return reply({tags:['日常','出门','工作'],revision:1,fluctlight_id:fid});
   if(end==='/inventory/preparation')return reply({id:'parity-preparation-'+fid,fluctlight_id:fid,status:'completed',profile_id:'everyday',result:{inventory_committed:true},can_recover:false});
   if(end==='/schedules'){const day=url.searchParams.get('from')||'2026-10-27';return reply({today:'2026-10-27',from:day,to:day,timezone:'Asia/Shanghai',context_revision:'parity-context-1',days:[{local_date:day,schedule:schedule(fid,day),request:null}]});}
   if(end.startsWith('/goals/')){const row=goalRows(fid).find(g=>g.id===end.split('/')[2]);if(row)return reply(['/evidence','/history'].some(tail=>end.endsWith(tail))?{items:[],next_cursor:''}:row);}
   if(end==='/goals')return reply({items:url.searchParams.get('history')==='true'?[]:goalRows(fid),next_cursor:''});
   if(end==='/diary')return reply({items:diaryRows(fid),next_cursor:'',timezone:'Asia/Shanghai'});
   if(end.startsWith('/diary/')){const row=diaryRows(fid).find(d=>d.id===end.split('/')[2]);if(row)return reply(end.endsWith('/replies')?{items:[],next_cursor:''}:row);}
   if(end==='/shared-experiences')return reply({items:[],next_cursor:'',physical_presence_source:'requires_explicit_confirmed_joint_participation'});
   if(end==='/memories')return reply({items:memoryRows(fid).filter(m=>(!url.searchParams.get('status')||m.status===url.searchParams.get('status'))&&(!url.searchParams.get('type')||m.type===url.searchParams.get('type'))&&(!url.searchParams.get('q')||m.content.includes(url.searchParams.get('q')))),next_cursor:''});
   if(end==='/reflections')return reply({items:data.reflections.map(r=>({...r,fluctlight_id:fid,body:r.summary,status:({pending:'pending',accepted:'applied',rejected:'rejected'})[r.decision],created_at:r.date+'T15:00:00+08:00',source_refs:[],proposal:{},dispositions:[],result:null})),next_cursor:'',window:{},latest_task:null});
   if(end==='/self-claims')return reply({items:data.claims.map(c=>({...c,fluctlight_id:fid,category:'self_perception',evidence_refs:[],created_at:timestamp,updated_at:timestamp,source_valid:true,expired:false})),next_cursor:''});
   if(['/promises','/schedule-requests','/owner-requests','/life-timeline'].includes(end))return reply({items:[],next_cursor:''});
  }
  misses.push({path,method});json(res,{code:'parity_fixture_not_implemented',message:'此同场景接口尚未建立；本项不得标记通过。'},501);return true;
 }};
}
