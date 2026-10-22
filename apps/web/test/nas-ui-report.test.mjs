import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {userInstant,displayTimezone} from '../src/v3/presentation-time.mjs';
import {stateLabel,domainName} from '../src/v3/domain-presentation.mjs';
import {DiagnosticCard} from '../src/v3/proto/components/diagnostic-card.js';
import {AutonomyFields} from '../src/v3/proto/components/autonomy-fields.js';
import {normalizeRuntimeCatalog} from '../src/v3/runtime-catalog.mjs';
import {RuntimeCatalog} from '../src/v3/proto/components/runtime-catalog.js';
import {SemanticFields} from '../src/v3/proto/components/semantic-fields.js';
import {CurrentProfile} from '../src/v3/proto/components/current-profile.js';
import {CreationPreview} from '../src/v3/proto/components/creation-preview.js';
import {GoalRecords} from '../src/v3/proto/components/goal-records.js';

test('NAS instant reads use Beijing time including cross-day and midnight',()=>{
 assert.equal(userInstant('2026-10-20T14:28:40.520+00:00','Asia/Shanghai'),'2026-10-20 22:28 (北京时间)');
 assert.match(userInstant('2026-10-20T16:00:00Z','Asia/Shanghai'),/2026-10-21 00:00/);
 assert.equal(displayTimezone('Asia/Tokyo','Asia/Shanghai'),'Asia/Tokyo');
 assert.doesNotThrow(()=>userInstant('2026-10-20T14:00:00Z','not-a-zone'));
 assert.equal(userInstant('not-a-date'),'时间未知');
});
test('diagnostic primary content explains real lifecycle and failure without raw codes or UTC',()=>{
 const html=DiagnosticCard({kind:'lifecycle',id:'original-id',ref:'ref',occurred_at:'2026-10-20T14:28:40.520+00:00',data:{event_type:'lifecycle.wake_up.completed_noop',status:'no_op'}});
 const primary=html.split('<details')[0];assert.match(primary,/日常唤醒/);assert.match(primary,/无新增操作/);assert.doesNotMatch(primary,/lifecycle\.|original-id|T14:28/);assert.match(html,/T14:28/);
 assert.equal(domainName('schedule.current_day'),'今日日程准备');assert.equal(stateLabel('awaiting_trigger'),'等待触发条件');assert.equal(stateLabel('failed'),'执行失败');assert.equal(stateLabel('future_state'),'状态待确认');
});
test('autonomy groups keep existing form names and action attributes while separating controls',()=>{
 const html=AutonomyFields({mode:'active',outreachPaused:false,notificationsMuted:true,allowedActions:['moment'],confirmActionTypes:['capability'],concurrencyLimit:1,cooldownSeconds:0,quietHours:{}});
 assert.equal((html.match(/class="policy-section stack"/g)||[]).length,4);
 assert.equal((html.match(/data-allowed=/g)||[]).length,3);assert.equal((html.match(/data-confirm=/g)||[]).length,3);
 for(const name of ['mode','outreachPaused','notificationsMuted','concurrencyLimit','cooldownSeconds','quietEnabled','quietStart','quietEnd','quietTimezone'])assert.ok(html.includes(`name="${name}"`));
 assert.match(html,/autonomy-choice-group/);assert.doesNotMatch(html,/Owner/);
});
test('real Go PascalCase capability definitions become named catalog entries without altering source',()=>{
 const raw={agents:[{id:'init',name:'人物初始化',role:'initialization'}],capabilities:[{Name:'life.current',Version:'v1',Type:'query',Description:'查看当前生活'}]};
 const normalized=normalizeRuntimeCatalog(raw),html=RuntimeCatalog(normalized);
 assert.equal(normalized.capabilities[0].name,'life.current');assert.equal(raw.capabilities[0].name,undefined);assert.match(html,/life.current/);assert.match(html,/查询/);assert.match(html,/人物初始化/);assert.doesNotMatch(html,/>\s*·\s*·/);
 assert.equal(normalizeRuntimeCatalog({agents:[],capabilities:[{name:'legacy',version:'v0',type:'action'}]}).capabilities[0].name,'legacy');
});
test('actual initialization clothing shape has contextual availability/wear labels and no orphan booleans',async()=>{
 const fixture=JSON.parse(await readFile(new URL('../../../docs/fixtures/goal-closed-loop-acceptance-persona.json',import.meta.url),'utf8'));
 const life=structuredClone(fixture.core_persona.life_profile);life.appearance.wardrobe_items.push({category:'outdoor_clothing',slot:'outerwear',description:'海边散步的防风外套',ownership:'unknown',available:true,currently_worn:false});
 const html=SemanticFields(life);assert.match(html,/白色棉质衬衫/);assert.match(html,/防风外套/);assert.match(html,/初始未穿着/);assert.match(html,/可用/);assert.match(html,/户外服装/);assert.doesNotMatch(html,/>是<|>否<|outdoor_clothing|currently_worn|server_default/);
 const narrative=SemanticFields({voice:{speech_style:'清楚、口语化，偶尔温和调侃'},scenario_behavior:{custom_scene:{authored_note:'在陌生场合先倾听'}}});assert.match(narrative,/清楚、口语化/);assert.match(narrative,/在陌生场合先倾听/);assert.doesNotMatch(narrative,/custom_scene|authored_note/);
});
test('current profile distinguishes unknown/cleared body and known-empty wear, preserving initial boundary',()=>{
 const current={inner_state:{mood:{label:'平静',source:'server_default',intensity:0}},context:{source:'pending',timezone:'Asia/Shanghai',instant:'2026-10-20T14:28:40.520Z'},appearance:{body_fields:{hair_color:{status:'cleared',value:'stale'},hair_length:{status:'unknown'}},wearing_state:'known',worn_items:[]}};
 const known=CurrentProfile(current);assert.match(known,/已清除，当前未记录/);assert.match(known,/当前没有已登记的穿着/);assert.match(known,/2026-10-20 22:28 \(北京时间\)/);assert.doesNotMatch(known,/server_default|stale|T14:28/);
 const unknown=CurrentProfile({...current,appearance:{...current.appearance,wearing_state:'unknown'}});assert.match(unknown,/当前穿着尚未确认/);assert.doesNotMatch(unknown,/当前没有已登记的穿着/);
});
test('initial goal review keeps all JSON under closed disclosures and uses actual goal vocabulary',()=>{
 const html=CreationPreview({timezone:'Asia/Shanghai',analysisText:'{}',analysis:{core_persona:{},extensions:{initialization_warnings:[{path:'actor_user',message:'原值已保留，缺失结构已补齐'}]},initial_goals:[{desired_outcome:'完成一次海边摄影',success_criteria:['选出三张喜欢的照片'],status:'candidate',importance:.7}]}});
 assert.match(html,/完成一次海边摄影/);assert.match(html,/选出三张喜欢的照片/);assert.match(html,/关于你资料/);assert.doesNotMatch(html.split('<details')[0],/actor_user/);
 let depth=0;for(const tag of html.matchAll(/<\/?(?:details|pre)\b[^>]*>/g)){if(tag[0].startsWith('<details')){assert.ok(!/\bopen\b/.test(tag[0]));depth++;}else if(tag[0].startsWith('</details'))depth--;else if(tag[0].startsWith('<pre'))assert.ok(depth>0,'JSON was expanded in review body');}
});
test('goal history translates actual creation and planner states while preserving raw records',()=>{
 const html=GoalRecords([{reason:'goal created',status:'active'},{status:'awaiting_trigger'}]);assert.match(html,/目标已创建/);assert.match(html,/运行中/);assert.match(html,/等待触发条件/);assert.doesNotMatch(html.split('<details')[0],/goal created|awaiting_trigger/);assert.match(html,/goal created/);
});
