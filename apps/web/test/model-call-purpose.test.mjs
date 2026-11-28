import test from 'node:test';
import assert from 'node:assert/strict';
import {callPurpose,callPurposeSearchQuery,modelCallGroups} from '../src/approved-ui/task-model-call-presentation.mjs';
import {diagnosticProjection} from '../src/approved-ui/presentation.mjs';

test('actual scenarios and schema fallbacks distinguish calls sharing a generic binding',()=>{
 for(const [scenario,label] of [['schedule_generation','日程生成'],['schedule_replan_planner','日程重排'],['wardrobe_preparation','初始穿搭准备'],['fact_only_reply','聊天回复'],['world_event_generation','世界随机事件生成']]){
  const call={scenario,role:'cognitive_assessment',binding_role:'generic_llm'};
  assert.equal(callPurpose(call).label,label);
  assert.equal(diagnosticProjection({ref:scenario,data:call},0,[]).label,label);
 }
 assert.equal(callPurpose({scenario:'unknown-version',role:'initialization'}).label,'人物初始化');
 assert.equal(callPurpose({role:'cognitive_assessment',metrics:{context_projection:{schema:'schedule_replan_plan'}}}).label,'日程重排');
 assert.match(callPurpose({scenario:'future-scenario',binding_role:'generic_llm'}).label,/future-scenario/);
});
test('Kev combines actual mixed points and world queries use recorded identifiers',()=>{
 assert.equal(callPurpose({scenario:'kev_decision',metrics:{decision_point:'world.event.admit'}}).label,'Kev 世界事件入库审核');
 const combined=callPurpose({scenario:'kev_decision',metrics:{decision_points:['media.capture_plan','media.view_plan']}});
 assert.equal(combined.label,'Kev 组合决策');assert.ok(combined.codes.includes('media.capture_plan'));
 assert.ok(JSON.parse(callPurposeSearchQuery('日程生成').qAny).includes('schedule_generation'));assert.ok(JSON.parse(callPurposeSearchQuery('记忆检索向量').qAny).includes('embedding'));
 assert.deepEqual(callPurposeSearchQuery('Kev 世界事件入库审核'),{q:'world.event.admit'});
 assert.deepEqual(callPurposeSearchQuery('raw-correlation'),{q:'raw-correlation'});assert.ok(JSON.parse(callPurposeSearchQuery('回复动作核验').qAny).includes('reply_action_consistency'));
});

test('Loop display groups recorded run identity and role without inferring task success',()=>{
 const row=(ref,patch={})=>({ref,kind:'model-runs',occurred_at:'2026-11-16T00:00:00Z',data:{logical_run_id:'run-a',correlation_id:'turn:a',fluctlight_id:'A',role:'conversation_cognition',binding_role:'generic_llm',...patch}});
 const first=row('one',{sequence:1,model_id:'model-a',status:'completed',stage:'tool_request'});
 const second=row('two',{sequence:2,model_id:'model-b',status:'failed'});
 const third=row('three',{sequence:3,status:'completed',round_count:50});
 const original=JSON.stringify([third,second,first]);
 const groups=modelCallGroups([third,second,first,row('writer',{logical_run_id:'reply:child',parent_run_id:'run-a',role:'fact_only_reply'}),row('kev',{role:'kev_decision'}),row('other-actor',{fluctlight_id:'B'}),row('other-run',{logical_run_id:'run-b'})]);
 assert.equal(groups.length,5);
 assert.deepEqual(groups[0].records.map(record=>record.ref),['one','two','three']);
 assert.equal(groups[0].summary,'已读取 3 次调用 · 1 次失败');
 assert.doesNotMatch(groups[0].summary,/50|任务完成|Loop.*完成/);
 assert.equal(JSON.stringify([third,second,first]),original);
 const partial=modelCallGroups([third,second])[0];assert.equal(partial.records.length,2);assert.match(partial.summary,/已读取 2/);
});
test('missing run identity never merges by correlation, time or inferred sequence',()=>{
 const row=(ref,data)=>({ref,kind:'model-runs',data});
 assert.equal(modelCallGroups([row('one',{correlation_id:'same',role:'conversation'}),row('two',{correlation_id:'same',role:'conversation'})]).length,2);
 assert.equal(modelCallGroups([row('one',{logical_run_id:'same'}),row('two',{logical_run_id:'same'})]).length,2);
 const group=modelCallGroups([row('one',{metrics:{run_id:'actual'},role:'conversation',sequence:0}),row('two',{metrics:{run_id:'actual'},role:'conversation',sequence:0})])[0];
 assert.equal(modelCallGroups([row('a',{metrics:{run_id:'same',fluctlight_id:'A'},role:'conversation'}),row('b',{metrics:{run_id:'same',fluctlight_id:'B'},role:'conversation'})]).length,2);
 assert.equal(group.repeatedSequence,true);assert.equal(group.missingTime,true);assert.deepEqual(group.records.map(r=>r.data.sequence),[0,0]);
});
