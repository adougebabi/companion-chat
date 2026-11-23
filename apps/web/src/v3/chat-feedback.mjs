import {errorMessage} from './domain-presentation.mjs';
// A scheduled activity is a plan, even when its scheduled time is now.
export function chatContextText(context){
 const text=[context?.activity,context?.location].filter(Boolean).join(' · ');
 return text&&context?.source==='schedule'?'安排 · '+text:text;
}
// Presentation only: the conversation store retains the original failure and retry identity.
export function chatFailureText(value, delivered = false) {
  if(delivered)return '回复已经保存，后续处理尚未完成。可以查看本次运行。';
  const text=String(value || '');
  if(!text)return '';
  if(/direct_media_delivery_unresolved|direct_media_delivery_unverified|direct_media_delivery_stale/.test(text))return '回复未发送：图片发送状态与回复内容未能通过核对。原消息已保留，可查看本次回应中的被拦内容和校验依据后重试。';
  if(/direct_media_delivery_check_failed/.test(text))return '回复未发送：图片发送状态检查未完成。原消息已保留，可查看本次运行后重试。';
  if(/direct_media_reply_stalled/.test(text))return '回复未发送：图片与文字处理未能继续，已停止重复尝试。原消息已保留，请查看本次运行。';
  if(/direct_reply_arguments_unresolved/.test(text))return '回复未发送：模型多次未能提供可用的回复参数。原消息已保留，请查看本次运行后重试。';
  if(/adk_run_failed|agent_run_failed/.test(text))return '回复未发送：生成或发送前处理失败。原消息已保留，请查看本次回应的失败环节；执行结束不表示回复已送达。';
  if(/persona_preparation_(?:failed|pending)/.test(text))return errorMessage({message:text});
  if(text==='取消请求结果尚未确认，请刷新进度核对。原消息已保留。')return text;
  if(/reply_media_no_progress/.test(text))return '图片准备尚未推进，原消息和草稿已保留。请查看本次运行后恢复。';
  if(/media_subject_not_authorized/.test(text))return '图片中的人物范围尚未确认，生成没有开始。原消息已保留，请查看本次运行后核对人物。';
  if(/capability_prepare_failed/.test(text))return '本次工具准备没有完成，原消息已保留。请查看本次运行中的失败原因后重试。';
  if(/agent_partial_committed/.test(text))return '这次回应没有完成，有些行动可能已经发生。请先查看近况。';
  if(/tool_discovery_no_progress/.test(text))return '工具准备未能继续，已停止重复尝试。原消息已保留，可以查看本次运行后重试。';
  if(/gateway_timeout|\b524\b/.test(text))return '模型中转服务等待超时，没能收到回应。原消息已保留，可以检查模型通道后再试。';
  if(/budget_exceeded/.test(text))return '模型服务未能处理本次资料。原消息已保留，请查看本次运行。';
  if(/timeout|timed.out|deadline exceeded/.test(text))return '暂时没能回应：模型服务响应超时。原消息已保留，可以稍后重试。';
  if(/request_cancelled|context canceled|reply_cancelled/.test(text))return '本次处理被中断，消息已保留。这不一定是手动取消，可查看本次运行记录。';
  if(/unavailable|http_error|request_failed/.test(text))return '暂时没能回应：模型服务连接失败，请检查模型通道后重试。';
  if(/已取消|cancelled|canceled/.test(text))return '已停止等待这次回应。';
  return '暂时没能回应。可以稍后再试一次。';
}

export function chatFailureStage(code='') {
 if(/direct_media_delivery|direct_media_reply/.test(code))return '发送前核对图片状态';
 if(/direct_reply_arguments/.test(code))return '核对回复参数';
 if(/agent_cognition_settlement/.test(code))return '保存本轮处理结果';
 if(/agent_output_publication/.test(code))return '发布回复';
 if(/timeout|provider_|request_failed/.test(code))return '调用模型服务';
 return '生成或发送前处理';
}

export function chatDiagnosticTurn(state){
  const pending=state.messages.findLast(message=>message.kind==='user'&&!/^(?:local-|stream-)/.test(message.id)&&message.turnId!==state.queuedTurn?.turnId&&['pending','running'].includes(message.turnStatus));
  if(pending?.turnId)return pending.turnId;
  // History/page errors have no turn identity. A queued message can be newer
  // than the failed request and must never become that failure's correlation.
  const error=String(state.error || '');
  if(!/^(?:回复未完成[:：]|本次生成已执行部分操作|回复已取消|正在结束本次生成|发送状态未确认|(?:发送|重试)请求未被接受|消息未被接受)/.test(error))return null;
  const retry=state.retryTurn;
  if(retry?.conversationId===state.conversation?.id&&retry?.fluctlightId===state.fluctlightId&&retry?.turnId&&retry.turnId!==state.queuedTurn?.turnId)return retry.turnId;
  if(state.activeTurnId&&state.activeTurnId!==state.queuedTurn?.turnId)return state.activeTurnId;
  const failed=state.messages.findLast(message=>message.kind==='user'&&!/^(?:local-|stream-)/.test(message.id)&&['failed','cancelled'].includes(message.turnStatus));
  return failed?.turnId&&failed.turnErrorCode&&error.includes(failed.turnErrorCode)?failed.turnId:null;
}

export function chatWaitText(state, startedAt, now=Date.now()) {
 return chatWaitingPresentation(state,startedAt,now).label;
}

export function chatWaitingPresentation(state, startedAt, now=Date.now(), name='对方') {
 const busy=state.sending||state.hasPendingTurn;
 const seconds=Math.max(0,Math.floor((now-(Number.isFinite(startedAt)?startedAt:now))/1000));
 return {seconds,label:busy?`${name} 正在输入`:'',detail:busy&&seconds>=30?`${name}还在想…`:'',showDiagnostics:busy&&seconds>=30,showStop:busy&&seconds>=90};
}
