import { errorMessage, stateLabel } from '../v3/domain-presentation.mjs';
export function wardrobePreparationStage(preparation, readError = '') {
  const terminal = !!preparation?.can_recover || ['failed','cancelled','dead_letter','deadletter'].includes(preparation?.status);
  const state = readError || !preparation ? 'unknown' : preparation.status === 'completed' ? 'ready' : terminal ? 'failed' : ['generating','awaiting_wear','running','started'].includes(preparation.status) ? 'running' : 'pending';
  return {key:'wardrobe',title:'初始衣柜',state,
    note:readError || (terminal ? preparation.result?.failure_reason || errorMessage({code:preparation.error_code},'初始穿搭准备未完成，请检查模型服务后恢复。') : stateLabel(preparation?.status,'等待后台准备')),
    retryable:terminal || preparation?.status === 'not_started',configSection:'模型用途',
    href:preparation?.task_id ? '#/diag/task/'+encodeURIComponent(preparation.task_id) : undefined};
}
