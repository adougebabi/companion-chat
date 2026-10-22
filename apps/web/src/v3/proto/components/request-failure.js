import {JsonDetails} from './records.js';

// Fixed system reasons from owner_life_requests.go settlement. Authored Chinese
// decisions remain untouched; this is presentation only, never a DTO rewrite.
const systemReasons={
 committed:'The requested change was committed',
 promise:'An intention was recorded; fulfillment requires later evidence',
 planning:'The proposed action awaits explicit Planner goal binding; no accepted promise yet',
 noChange:'The exact requested target state was already recorded; no new mutation was needed',
 notApplied:'The request did not produce a committed result',
};
export function requestDecisionText(request={}){
 const result=request.result||{},reason=request.decision_reason||result.reason||'';
 if(/[\u3400-\u9fff]/u.test(reason))return reason;
 if(request.status==='failed'){
  if(result.error_code==='owner_life_assessment_failed')return '本次请求未能完成判断。实际执行结果仍需核对，请打开关联诊断和结果来源查看。';
  if(reason===systemReasons.notApplied||result.error_code==='owner_life_request_not_applied')return '本次请求没有可确认的已提交结果，请查看原请求与实际任务。';
  return '本次请求处理失败，具体原因与实际结果请查看原请求和关联任务。';
 }
 if(reason===systemReasons.committed&&request.status==='completed')return '请求记录显示所需变更已提交，具体结果可查看实际来源。';
 if(reason===systemReasons.promise&&request.status==='accepted')return '已记录行动意向；是否履行仍需后续实际证据。';
 if(reason===systemReasons.planning&&request.status==='awaiting_goal')return '等待目标规划明确关联目标，约定尚未接受。';
 if(reason===systemReasons.noChange&&request.status==='completed'&&result.disposition==='no_change')return '所请求的目标状态已有记录，本次无需新增操作。';
 if(!reason)return '';
 return '原始处理说明已保留，实际结果请核对请求状态与关联来源。';
}
export function RequestDecisionSource(request){
 return request?JsonDetails(request,'请求原始状态、理由与时间'):'';
}
export function RequestFailureLink(request){
 if(request.status!=='failed'||request.result?.error_code!=='owner_life_assessment_failed')return '';
 return `<p><a class="btn sm" href="#/diag/events?eventType=owner.life.request.assessment_failed&q=${encodeURIComponent(request.id)}">查看本次判断失败原因</a></p><p class="small muted">服务恢复后可重新发出建议；原请求的失败记录会保留。</p>`;
}
