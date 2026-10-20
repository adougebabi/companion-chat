export function requestDecisionText(request){
 if(request.status==='failed'&&request.result?.error_code==='owner_life_assessment_failed')return '本次请求未能完成判断，未产生已确认的执行结果。请打开关联诊断查看当时的服务错误。';
 return request.decision_reason || '';
}
export function RequestFailureLink(request){
 if(request.status!=='failed'||request.result?.error_code!=='owner_life_assessment_failed')return '';
 return `<p><a class="btn sm" href="#/diag/events?eventType=owner.life.request.assessment_failed&q=${encodeURIComponent(request.id)}">查看本次判断失败原因</a></p><p class="small muted">服务恢复后可重新发出建议；原请求的失败记录会保留。</p>`;
}
