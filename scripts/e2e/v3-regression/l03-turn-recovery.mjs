import {runScenario,check,equal} from './runtime.mjs';
import {fixture,conversation,chat,goArgs,nodeArgs} from './domain.mjs';
await runScenario('L03','取消、断流与同身份重试',async t=>{
 const f=await t.step('fixture','Use a dedicated real direct conversation',()=>fixture(t));if(!f)return;const c=await conversation(t,f.id,'recovery');
 await t.step('disconnect','Observer disconnect and accepted Worker execution are independent',()=>t.suite('disconnect',goArgs(['TestDurableBrowserObserverDisconnectDoesNotCancelWorker']),{database:true}));
 const cancelled=await t.step('cancel','Explicitly cancel the exact turn and keep already committed messages',async()=>{
  let sent=false;const turn=await chat(t,f.id,c.id,'cancel','这是隔离的取消回归，请准备一段简短回答。',{allowError:true,onEvent:async event=>{if(!sent&&event.turnId){sent=true;await t.request(`/api/conversations/${encodeURIComponent(c.id)}/turn/${encodeURIComponent(event.turnId)}/cancel`,{method:'POST',body:{},expected:[200,202,204,409]});}}});
  const original=t.state.commands[`${t.id}:chat:cancel`],page=await t.until('cancelled or already completed turn',()=>t.get(`/api/conversations/${encodeURIComponent(c.id)}/messages?limit=100`),p=>p.messages?.some(m=>m.turnId===original.body.turnId&&['cancelled','completed','failed'].includes(m.turnStatus)));const user=page.messages.filter(m=>m.turnId===turn.turnId&&m.kind==='user');equal(user.length,1,'Cancellation must preserve one accepted user message');return {turnId:turn.turnId,user:user[0]};
 });
 await t.step('retry','Retry uses the original turn, payload, message and frozen sender clock',async()=>{
  if(!cancelled)t.blocked('CANCEL_PREREQUISITE_MISSING','The real cancellation result is required');
  if(cancelled.user.turnStatus==='completed')t.blocked('CANCEL_RACE_COMPLETED','The turn completed before cancellation; no unsafe forced retry was sent',{turnId:cancelled.turnId});
  check(cancelled.user.turnRetryable===true,'Only a server-authorized retryable turn can be replayed',true,cancelled.user);
  const result=await chat(t,f.id,c.id,'cancel','ignored because the original body is frozen');const users=result.messages.filter(m=>m.turnId===cancelled.turnId&&m.kind==='user');equal(users.length,1,'Retry must not duplicate the user message');equal(users[0].id,cancelled.user.id,'Retry must reuse the accepted message ID');equal(users[0].senderSentAt,cancelled.user.senderSentAt,'Retry must retain the original sender timestamp');return {userMessageId:users[0].id,turnId:result.turnId};
 });
 await t.step('guards','Provider failure, late reply fence, partial effects and frozen sender regression',async()=>({go:await t.suite('retry-fences',goArgs(['TestExplicitCancelIsRetryableWithoutDuplicateUserMessage','TestExplicitCancelStopsRunningProviderAndFencesLateReply','TestProviderFailureExposesRetryableTurnAndReusesUserMessage','TestNativeProseFailureDurableRetryPreservesSenderIdentity']),{database:true}),component:await t.suite('conversation-scope',nodeArgs(['conversation-scope-recovery.test.mjs']))}));
});
