// Presentation only: the conversation store retains the original failure and retry identity.
export function chatFailureText(value) {
  const text=String(value || '');
  if(/prompt_(?:tool_result_)?budget_exceeded|prompt_context_budget_exceeded/.test(text))return '本次回复需要读取的资料超出模型可处理范围，回复未完成。原消息已保留；可在诊断中查看原因，修复后重试。';
  if(text.includes('agent_partial_committed'))return '本次回复未全部完成，但已有操作提交。请先核对已显示的结果和诊断记录。';
  if(text.includes('owner_life_assessment_failed'))return '本次请求未能完成判断。请查看关联诊断，确认模型服务恢复后再决定是否重新发出建议。';
  if(/agent_run_failed|provider_[a-z_]+|turn_failed/.test(text))return '本次回复未能完成。原消息已保留，请查看关联诊断；确认服务恢复后可重试。';
  return text;
}

export function chatDiagnosticTurn(state){
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
