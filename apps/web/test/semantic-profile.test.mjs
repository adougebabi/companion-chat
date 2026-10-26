import test from 'node:test';
import assert from 'node:assert/strict';
import {IntentionRow} from '../src/v3/proto/components/cards.js';
import {SemanticFields} from '../src/v3/proto/components/semantic-fields.js';
import {ProfileGroups,PersonaSection} from '../src/v3/proto/components/profile.js';
import {profileSnapshotFromDetail} from '../src/v3/proto/data/profile.js';
import {readInitializationProgress,initializationRunning} from '../src/v3/initialization-progress.mjs';
import {InitializationProgress,mountInitializationProgress} from '../src/v3/proto/components/initialization-progress.js';
import {managementNavigation,workspaceForPath} from '../src/v3/workspace-navigation.mjs';

test('intentions present their action, Chinese lifecycle and linked goal before technical details',()=>{
 const html=IntentionRow({id:'intent',action:'保持镜头前的形象',status:'candidate',confidence:0,goal_id:'goal/x'},{fid:'role',goals:[{id:'goal/x',desired_outcome:'准备直播'}]});
 const main=html.split('<details')[0];assert.match(main,/保持镜头前的形象/);assert.match(main,/候选意向/);assert.match(main,/把握 0%/);assert.match(main,/准备直播/);assert.match(main,/goal%2Fx/);assert.doesNotMatch(main,/candidate|"action"|goal_id/);
 assert.match(IntentionRow({status:'unrecognized',confidence:null}),/状态待确认/);assert.doesNotMatch(IntentionRow({confidence:null}),/把握 0%/);
});
test('profile fields use readable settings and keep machine source only in collapsed details',()=>{
 const s=profileSnapshotFromDetail({id:'role',identity:{name:'角色'},personality:{agreeableness:null},core_persona:{personality_system:{profiles:[{id:'public',name:'镜头前',personality:{curiosity:.8},voice:{tone:'明快',speech_patterns:['先听再说']},desires:['完成直播']}]}},personality_runtime:{active_profile_id:'public',revision:1},persona_compilation:{status:'ready',source_hash:'SECRET_SOURCE_HASH',working_persona:{portrait_text:'说话明快，先听再说。'}},developing_self:{claims:[]}});
 const main=ProfileGroups(s).split(/<details\b[^>]*class="card profile-technical"[^>]*>/)[0];assert.match(main,/说话明快/);assert.doesNotMatch(main,/agreeableness|source_hash|SECRET_SOURCE_HASH/);
 const persona=PersonaSection(s,false).split('此人格的完整设定记录')[0];assert.match(persona,/好奇心/);assert.match(persona,/80%/);assert.match(persona,/声音与说话方式/);assert.match(persona,/明快/);assert.match(persona,/完成直播/);
 const escaped=SemanticFields({notes:'<img onerror=alert(1)>',agreeableness:null});assert.match(escaped,/&lt;img/);assert.doesNotMatch(escaped,/<img/);
});
const ready=value=>({state:'ready',value});
function progress(overrides={}){return {fid:'role',identity:ready({fluctlight_id:'role',session:{id:'session',status:'queued',current_attempt:1,attempts:[{attempt_number:1,candidate_asset_id:'candidate'}]}}),avatar:ready({fluctlight_id:'role',mode:'canonical',read_state:'source_unavailable',avatar:null}),timeline:ready({items:[{session_id:'old',stage:'completed',status:'completed',summary:'OLD_COMPLETED'}]}),task:{state:'not_requested'},...overrides};}
test('a candidate, old timeline or skipped review never becomes accepted artwork or completed avatar',()=>{
 const html=InitializationProgress(progress());assert.match(html,/候选图已生成，尚待接受/);assert.match(html,/排队中/);assert.doesNotMatch(html,/OLD_COMPLETED|已接受规范参考 ·/);assert.equal(initializationRunning(progress()),true);
 const skipped=InitializationProgress(progress({timeline:ready({items:[{session_id:'session',stage:'vision_ready',status:'skipped'}]})}));assert.match(skipped,/已跳过，尚未完成验证/);
 const failed=InitializationProgress(progress({identity:{state:'failed',message:'不可读取'}}));assert.match(failed,/视觉进度读取失败/);assert.match(failed,/人物资料仍可使用/);
});
test('progress reader isolates failed optional sources and rejects foreign task/asset scope',async()=>{
 const calls=[];const result=await readInitializationProgress({visualIdentityManagement:async()=>({fluctlight_id:'role',session:{id:'session',task_id:'task'}}),displayAvatar:async()=>{throw new Error('avatar offline');},visualIdentityTimeline:async()=>({items:[]}),diagnosticTask:async id=>{calls.push(id);return {fluctlight_id:'foreign'};}},'role');
 assert.equal(result.identity.state,'ready');assert.equal(result.avatar.state,'failed');assert.equal(result.task.state,'failed');assert.deepEqual(calls,['task']);
});
test('disposing profile progress prevents a late read from painting another route',async()=>{
 let resolve;const host={isConnected:true,innerHTML:'',querySelector(){throw new Error('late paint');}};
 const view=mountInitializationProgress(host,{fid:'role',read:()=>new Promise(done=>{resolve=done;})});view.dispose();host.innerHTML='later route';resolve(progress());await new Promise(done=>setImmediate(done));assert.equal(host.innerHTML,'later route');
});
test('people management uses the registered NPC surface and all mutation paths stay in management',()=>{
 assert.equal(managementNavigation.find(n=>n.k==='people').path,'people/npcs');
 for(const path of ['/c/role/people/manage','/c/role/people/npcs','/c/role/people/npc/new','/c/role/people/npc/actor/edit'])assert.equal(workspaceForPath(path),'管理');
 assert.equal(workspaceForPath('/c/role/people/npc/actor'),'交流');
});
