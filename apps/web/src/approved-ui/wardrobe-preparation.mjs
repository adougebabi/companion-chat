import { errorMessage, stateLabel } from '../v3/domain-presentation.mjs';
import {taskNeedsModelConfiguration,taskFailureReason} from './task-presentation.mjs';
export function wardrobePreparationStage(preparation, readError = '',task=null) {
  const terminal = !!preparation?.can_recover || ['failed','cancelled','dead_letter','deadletter'].includes(preparation?.status);
  const state = readError || !preparation ? 'unknown' : preparation.status === 'completed' ? 'ready' : terminal ? 'failed' : ['generating','awaiting_wear','running','started'].includes(preparation.status) ? 'running' : 'pending';
  const matchingTask=task?.task_id===preparation?.task_id?task:null;
  const needsConfiguration=taskNeedsModelConfiguration(matchingTask,preparation)||preparation?.error_code==='wardrobe_provider_timeout';
  return {key:'wardrobe',title:'初始衣柜',state,
    note:readError || (terminal ? task?.task_id===preparation?.task_id&&taskNeedsModelConfiguration(task,preparation)?taskFailureReason(task,preparation):preparation.result?.failure_reason || errorMessage({code:preparation.error_code},'初始穿搭准备未完成，原资料会保留。请查看任务记录后重试此环节。') : stateLabel(preparation?.status,'等待后台准备')),
    retryable:terminal || preparation?.status === 'not_started',configSection:needsConfiguration?'模型用途':undefined,
    href:preparation?.task_id ? '#/diag/task/'+encodeURIComponent(preparation.task_id) : undefined};
}
