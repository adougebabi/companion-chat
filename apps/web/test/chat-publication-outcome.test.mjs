import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {proxy,snapshot,subscribe} from 'valtio/vanilla';
import {installDOM,mountReact} from './helpers/react-dom.mjs';
import {chatFailureText,chatFailureStage} from '../src/v3/chat-feedback.mjs';
import {taskStatusPresentation,taskFailureReason} from '../src/approved-ui/task-presentation.mjs';

test('chat publication errors explain the failed stage without guessing provider capacity',()=>{
 const message=chatFailureText('direct_media_delivery_unresolved');
 assert.match(message,/回复未发送.*图片发送状态/);
 assert.doesNotMatch(message,/内存|稍后再试|direct_media/);
 assert.equal(chatFailureStage('direct_media_delivery_unresolved'),'发送前核对图片状态');
 assert.match(chatFailureText('adk_run_failed'),/执行结束不表示回复已送达/);
 assert.match(chatFailureText('direct_media_delivery_unresolved',true),/回复已经保存/);
});

const failedTask={task_id:'fixture-task',kind:'cognition.processing',status:'completed',workflow_status:'completed',source:{turn_status:'failed',turn_id:'fixture-turn',reply_delivered:false,error_code:'direct_media_delivery_unresolved'},runs:1,steps:[]};
test('business outcome wins over execution completion while keeping original status',()=>{
 const view=taskStatusPresentation(failedTask);
 assert.equal(view.rawStatus,'completed');assert.equal(view.status,'failed');assert.equal(view.group,'attention');assert.equal(view.label,'回复未发送');
 assert.match(taskFailureReason(failedTask),/回复未发送.*图片发送状态/);
 assert.equal(taskStatusPresentation({...failedTask,source:{...failedTask.source,turn_status:'completed',reply_delivered:true}}).label,'回复已发送');
 assert.equal(taskStatusPresentation({...failedTask,source:{...failedTask.source,turn_status:'completed',reply_delivered:false}}).label,'本轮未发送回复');
 assert.equal(taskStatusPresentation({...failedTask,source:{...failedTask.source,reply_delivered:true}}).label,'回复已发送，后续处理失败');
});

const env=installDOM(),vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),envFile:false,appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
test.after(async()=>{await vite.close();env.restore();});
const button=text=>[...document.querySelectorAll('button')].find(node=>node.textContent.trim()===text);

test('actual chat feedback states unsent result and failure stage, preserves retry identity',async()=>{
 const {default:Feedback}=await vite.ssrLoadModule('/src/react-app/components/ChatTurnFeedback.tsx');
 const message={id:'fixture-message',turnId:'fixture-turn',turnStatus:'failed',turnErrorCode:'direct_media_delivery_unresolved',turnRetryable:true};
 const retries=[],store={sending:false,retrying:false,retryMessage:id=>retries.push(id)};
 const v=await mountReact(vite,Feedback,{props:{message,messages:[],store}});
 try{
  assert.match(v.host.textContent,/回复未发送.*图片发送状态/);
  assert.doesNotMatch(v.host.textContent,/direct_media|adk_run|node path/);
  await v.click(button('本轮详情'));
  const drawer=document.querySelector('[data-testid=turn-details]');
  assert.match(drawer.textContent,/消息结果.*本次回复未发送/);
  assert.match(drawer.textContent,/失败环节.*发送前核对图片状态/);
  await v.click(drawer.querySelector('button[aria-label="关闭"]'));
  await v.click(button('重试原回应'));assert.deepEqual(retries,['fixture-message']);
 }finally{await v.dispose();}
});

test('actual task detail shows failed chat despite a completed execution',async()=>{
 const {default:Dialog}=await vite.ssrLoadModule('/src/react-app/shared/TaskRecordDialog.tsx');
 const state=proxy({session:{actorId:'fixture'},characters:[]});
 const w={state,subscribe:fn=>subscribe(state,fn),getSnapshot:()=>snapshot(state),navigatePath(){},services:{operationsClient:{task:async()=>failedTask},diagnosticsClient:{records:async()=>({items:[],next_cursor:''})}}};
 const v=await mountReact(vite,Dialog,{workspace:w,props:{taskId:'fixture-task',onClose(){}}});
 try{
  const dialog=document.querySelector('[role=dialog]');
  assert.match(dialog.textContent,/回复未发送/);
  assert.match(dialog.textContent,/任务执行已经结束，但聊天回合失败/);
  assert.match(dialog.textContent,/图片发送状态与回复内容未能通过核对/);
  assert.doesNotMatch(dialog.textContent,/direct_media_delivery|adk_run_failed|node path/);
 }finally{await v.dispose();}
});

test('diagnostic card explains the rejected sentence and links the exact checker',async()=>{
 const {DiagnosticCard}=await vite.ssrLoadModule('/src/v3/proto/components/diagnostic-card.js');
 const html=DiagnosticCard({id:'check',kind:'events',ref:'check',occurred_at:'2026-11-23T00:00:00Z',data:{event_type:'reply.media_delivery.checked',correlation_id:'turn:fixture-turn',payload:{status:'failed',reason_code:'delivery_reference_missing',rejected_claim:'图片已经发给你了。<script>bad()</script>',committed_deliveries:[],check_correlation_id:'direct-media-check:fixture'}}});
 const host=document.createElement('div');host.innerHTML=html;
 assert.match(host.textContent,/回复发送前的图片状态检查/);
 assert.match(host.textContent,/没有找到匹配的图片交付记录/);
 assert.match(host.querySelector('blockquote').textContent,/图片已经发给你了/);
 assert.equal(host.querySelector('script'),null);
 assert.match(host.textContent,/图片交付记录：0 条/);
 assert.ok(host.querySelector('a[href*="direct-media-check%3Afixture"]'));
 assert.doesNotMatch(host.querySelector('section').textContent,/delivery_reference_missing|reply\.media_delivery\.checked/);
});
