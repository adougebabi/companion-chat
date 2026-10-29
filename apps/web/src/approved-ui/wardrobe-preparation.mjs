import { errorMessage, stateLabel } from '../v3/domain-presentation.mjs';
import {taskNeedsModelConfiguration,taskFailureReason} from './task-presentation.mjs';
export function wardrobePreparationStage(preparation, readError = '',task=null) {
  const terminal = !!preparation?.can_recover || ['failed','cancelled','dead_letter','deadletter'].includes(preparation?.status);
  const state = readError || !preparation ? 'unknown' : preparation.status === 'completed' ? 'ready' : terminal ? 'failed' : ['generating','awaiting_wear','running','started'].includes(preparation.status) ? 'running' : 'pending';
  return {key:'wardrobe',title:'初始衣柜',state,
    note:readError || (terminal ? task?.task_id===preparation?.task_id&&taskNeedsModelConfiguration(task,preparation)?taskFailureReason(task,null):preparation.result?.failure_reason || errorMessage({code:preparation.error_code},'初始穿搭准备未完成，请检查模型服务后恢复。') : stateLabel(preparation?.status,'等待后台准备')),
    retryable:terminal || preparation?.status === 'not_started',configSection:'模型用途',
    href:preparation?.task_id ? '#/diag/task/'+encodeURIComponent(preparation.task_id) : undefined};
}
