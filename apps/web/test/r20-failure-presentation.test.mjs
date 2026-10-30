import test from 'node:test';
import assert from 'node:assert/strict';
import {taskNeedsModelConfiguration,taskFailureReason} from '../src/approved-ui/task-presentation.mjs';
import {wardrobePreparationStage} from '../src/approved-ui/wardrobe-preparation.mjs';

test('r20 stable context and assignment-read causes outrank legacy model-configuration heuristics',()=>{
 const task={task_id:'original',source:{last_error:'provider role generic_llm unavailable: connection refused'}};
 for(const code of ['wardrobe_preparation_context_failed','wardrobe_provider_assignment_unavailable']){
  const preparation={task_id:'original',status:'failed',can_recover:true,error_code:code,result:{failure_reason:'准确的已保存读取失败原因'}};
  assert.equal(taskNeedsModelConfiguration(task,preparation),false);
  assert.equal(taskFailureReason(task,preparation),preparation.result.failure_reason);
  const stage=wardrobePreparationStage(preparation,'',task);
  assert.equal(stage.note,preparation.result.failure_reason);assert.equal(stage.configSection,undefined);assert.equal(stage.retryable,true);
 }
 assert.equal(taskNeedsModelConfiguration(task,null),false);
 assert.doesNotMatch(taskFailureReason(task,null),/通用模型|尚未配置|尚未就绪/);
 assert.equal(taskNeedsModelConfiguration({...task,source:{last_error:'sql: no rows in result set'}},null),false);
});
test('r20 actual missing binding stays actionable while foreign-task configuration cannot replace the current cause',()=>{
 const preparation={task_id:'original',status:'failed',can_recover:true,error_code:'wardrobe_provider_unconfigured',result:{failure_reason:'通用语言模型用途尚未分配可用服务和模型'}};
 const task={task_id:'original',source:{last_error:'provider role generic_llm unavailable: no rows in result set'}};
 assert.equal(taskNeedsModelConfiguration(task,preparation),true);
 assert.equal(wardrobePreparationStage(preparation,'',task).note,preparation.result.failure_reason);
 assert.equal(wardrobePreparationStage(preparation,'',task).configSection,'模型用途');
 const current={...preparation,error_code:'wardrobe_preparation_failed',result:{failure_reason:'本任务的原始失败'}};
 const foreign=wardrobePreparationStage(current,'',{...task,task_id:'another-task'});
 assert.equal(foreign.note,current.result.failure_reason);assert.equal(foreign.configSection,undefined);
});
