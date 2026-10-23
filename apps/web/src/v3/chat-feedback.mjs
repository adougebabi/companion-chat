import {errorMessage} from './domain-presentation.mjs';
// A scheduled activity is a plan, even when its scheduled time is now.
export function chatContextText(context){
 const text=[context?.activity,context?.location].filter(Boolean).join(' · ');
 return text&&context?.source==='schedule'?'安排 · '+text:text;
}
// Presentation only: the conversation store retains the original failure and retry identity.
export function chatFailureText(value) {
  const text=String(value || '');
  if(!text)return '';
  if(/agent_partial_committed/.test(text))return '这次回应没有完成，有些行动可能已经发生。请先看看她的近况。';
  if(/gateway_timeout|\b524\b/.test(text))return '模型中转服务等待超时，没能收到回应。原消息已保留，可以检查模型通道后再试。';
  if(/budget_exceeded/.test(text))return '她暂时没能回应：需要读取的资料超过模型输入上限，请核对本次调用的有效预算。';
  if(/timeout|timed.out|deadline exceeded/.test(text))return '她暂时没能回应：模型服务响应超时。原消息已保留，可以稍后重试。';
  if(/request_cancelled|context canceled|reply_cancelled/.test(text))return '本次处理被中断，消息已保留。这不一定是手动取消，可查看本次运行记录。';
  if(/unavailable|http_error|request_failed/.test(text))return '她暂时没能回应：模型服务连接失败，请检查模型通道后重试。';
  if(/已取消|cancelled|canceled/.test(text))return '已停止等待这次回应。';
  return '她暂时没能回应。可以稍后再试一次。';
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
 if(!state.sending&&!state.hasPendingTurn&&!state.hasSettlingTurn)return '';
 const seconds=Math.max(0,Math.floor((now-startedAt)/1000));
 const elapsed=seconds<60?`${seconds} 秒`:`${Math.floor(seconds/60)} 分 ${seconds%60} 秒`;
 return `${state.hasSettlingTurn?'正在结束本次生成':state.sending?'正在连接或等待回复':'后台正在处理回复'} · 已等待 ${elapsed}。${seconds>=60?'她还没能回应，可以先看看她的生活，稍后回来。':seconds>=30?'正在等待她回应，稍后可以离开页面再回来查看。':''}`;
}
