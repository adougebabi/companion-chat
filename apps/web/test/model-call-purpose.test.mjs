import test from 'node:test';
import assert from 'node:assert/strict';
import {callPurpose,callPurposeSearchQuery} from '../src/approved-ui/task-model-call-presentation.mjs';
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
