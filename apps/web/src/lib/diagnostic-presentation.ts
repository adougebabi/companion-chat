function stringField(value: unknown, names: string[]): string {
  if (!value || typeof value !== "object" || Array.isArray(value)) return "";
  const record = value as Record<string,unknown>;
  for (const name of names) if (typeof record[name] === "string") return record[name] as string;
  for (const child of Object.values(record)) { const found=stringField(child,names); if(found)return found; }
  return "";
}
export function workflowPresentation(value: Record<string,unknown>, index = 0) {
  const workflowId=stringField(value,['workflow_id','workflowId','WorkflowId','WorkflowID']);
  const runId=stringField(value,['run_id','runId','RunId','RunID']);
  const rawStatus=stringField(value,['status','Status']).toLowerCase().replace(/^workflow_execution_status_/, '');
  const status=['running','completed','failed','cancelled','terminated','continued_as_new','timed_out'].includes(rawStatus) ? rawStatus : 'unknown';
  const startedAt=stringField(value,['start_time','started_at','startTime','StartTime']);
  const finishedAt=stringField(value,['close_time','completed_at','closeTime','CloseTime']);
  return {key:`${workflowId}:${runId || `unknown-${index}`}`,workflowId,runId,status,startedAt,finishedAt};
}
const eventWords: Record<string,string> = {lifecycle:'生命周期',adk:'运行',wake_up:'唤醒',reflection:'反思',cognition:'认知',tool:'工具',model:'模型',run:'运行',input:'输入',output:'输出',requested:'请求',result:'结果',termination:'终止',failed:'失败',activity_started:'开始执行',completed_noop:'无操作完成',provider_queued:'模型排队',trigger_released:'触发',queued:'排队',dispatched:'已派发'};
Object.assign(eventWords, {activity:'行动',completed:'已完成',cancelled:'已取消',started:'已启动',retry:'等待重试',blocked:'已阻止',unknown:'未知',superseded_by_cognition:'已由新的认知行动接替',user_quiet_period:'用户静默期',native_cognition_cycle_guarded:'已触发认知循环保护',provider_request_failed:'模型服务请求失败'});
export function eventTitle(code: string) { return code.split('.').map(part=>eventWords[part] ?? part).join(' · '); }
export function eventSummary(payload: unknown): { time: string; reason: string; stage: string } {
  const value=payload && typeof payload==='object' && !Array.isArray(payload) ? payload as Record<string,unknown> : {};
  return {time:String(value.occurred_at ?? value.last_seen_at ?? ''),reason:eventTitle(String(value.reason_code ?? value.error_code ?? '')),stage:eventTitle(String(value.stage ?? ''))};
}
