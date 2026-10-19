import {check,equal} from './runtime.mjs';

function instant(value,label){
 check(typeof value==='string'&&Number.isFinite(Date.parse(value)),`${label} must be a real timestamp`,undefined,value);
 return Date.parse(value);
}
export function assertPromiseReschedule(current,after,done,requestedEnd){
 equal(instant(after.window_start,'Promise start'),instant(current.window_start,'Original start'),'Rescheduling only the end must preserve the start');
 for(const key of ['goal_id','commitment_id','intention_id'])equal(after[key],current[key],`Rescheduling must retain ${key}`);
 if(['rejected','deferred'].includes(done.status)){
  equal(instant(after.window_end,'Promise end'),instant(current.window_end,'Original end'),'Refused rescheduling cannot move the window');
  return;
 }
 equal(done.status,'completed','An unexpected terminal Owner request is not successful rescheduling');
 equal(done.mutation_committed,true,'Accepted rescheduling needs a committed effect');
 const effect=done.effects?.find(e=>e.capability_name==='intention.decide'&&e.output?.status==='rescheduled');
 check(effect,'Accepted rescheduling needs the actual rescheduled Tool receipt');
 equal(effect.output.commitment_id,after.commitment_id,'Receipt must name the original Commitment');
 equal(effect.output.intention_id,after.intention_id,'Receipt must name the original Intention');
 const end=instant(requestedEnd,'Requested end');
 equal(instant(after.window_end,'Promise end'),end,'Promise must read back the exact requested end');
 equal(instant(after.commitment?.window_end,'Commitment end'),end,'Commitment window must be updated');
 equal(instant(after.intention?.expiration,'Intention expiration'),end,'Intention expiration must match the Promise');
}
export function assertLivePromiseFulfillment(p,evidence,messages){
 equal(p.lifecycle,'fulfilled','Intention execution alone is not fulfillment');
 equal(p.fulfillment_authority,'goal_commitment_evaluation','Use the existing proof authority');
 equal(p.commitment?.status,'completed','The Promise Commitment must actually complete');
 const evaluation=p.evaluations?.find(e=>e.commitment_id===p.commitment_id&&e.impact==='completed');
 check(evaluation?.evidence_refs?.length,'A completed Promise requires a persisted Commitment evaluation with evidence');
 const committed=new Set(messages.filter(m=>m.kind==='assistant'&&m.text?.trim()).map(m=>m.id));
 const actual=evidence.filter(row=>row.status==='confirmed'&&evaluation.evidence_refs.includes(`source:${row.source?.id}`));
 check(actual.some(row=>row.source?.source_kind==='message'&&committed.has(row.source.source_id)),'The communication promise must be fulfilled by an actual persisted assistant message');
 return {promiseId:p.request_id,goalId:p.goal_id,commitmentId:p.commitment_id,evaluationId:evaluation.id,evidenceRefs:evaluation.evidence_refs};
}
