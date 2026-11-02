import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'vite';
import {fileURLToPath} from 'node:url';
import {mountReact,installDOM,act} from './helpers/react-dom.mjs';
const env=installDOM();
const vite=await createServer({root:fileURLToPath(new URL('../',import.meta.url)),appType:'custom',logLevel:'silent',server:{middlewareMode:true}});
const load=path=>vite.ssrLoadModule('/src/'+path);
test.after(async()=>{await vite.close();env.restore();});
const button=(root,text)=>[...root.querySelectorAll('button')].find(node=>node.textContent.trim()===text);

test('studio notification and pending task labels localize the same actual kind',async()=>{
 const {domainName}=await load('v3/domain-presentation.mjs');
 const {notificationTitle}=await load('approved-ui/notification-title.mjs');
 assert.equal(domainName('studio.generate'),'创作内容生成');
 assert.equal(domainName('studio.generate.failed'),'创作内容生成 · 执行失败');
 assert.equal(notificationTitle({title:'studio.generate'}),'创作内容生成');
 const {default:Inbox}=await load('react-app/pages/InboxPage.jsx');
 const w={state:{session:{actorId:'r34-notice'},selectedId:'A',characters:[{id:'A',name:'青岚'}],notificationCount:{count:0}},subscribe:()=>()=>{},getSnapshot:()=>w.state,current:{value:{id:'A',name:'青岚'}},isCurrent:()=>true,services:{operationsClient:{notifications:async()=>({items:[],as_of:'now',next_cursor:''}),pending:async()=>({groups:[{kind:'failed_task',status:'ok',count:1,items:[{id:'studio',title:'studio.generate',status:'failed',fluctlight_id:'A',href:'/diag/task/studio'}],next_cursor:''}]})}},client:{},navigatePath(){}};
 const view=await mountReact(vite,Inbox,{workspace:w});
 try{assert.match(view.host.querySelector('.replica-pending-card').textContent,/创作内容生成/);assert.doesNotMatch(view.host.textContent,/studio\.generate/);}finally{await view.dispose();}
});

test('chat technical details open a right Sheet and raw JSON is collapsed; inspection never retries',async()=>{
 const {default:TurnFeedback}=await load('react-app/components/ChatTurnFeedback.tsx');
 const message={id:'r34-message',turnId:'r34-turn',turnStatus:'failed',turnRetryable:true,turnErrorCode:'request_timeout'};
 let retries=0,reads=0;
 const view=await mountReact(vite,TurnFeedback,{props:{message,messages:[],store:{sending:false,retrying:false,error:'',retryMessage(){retries++;},refreshActiveTurn(){reads++;}}}});
 try{
  const trigger=button(view.host,'查看技术详情');trigger.focus();
  assert.equal(view.host.querySelector('pre'),null);
  await view.click(trigger);
  const sheet=document.querySelector('[data-testid="chat-turn-details"]');
  assert.equal(sheet.dataset.side,'right');assert.equal(sheet.getAttribute('role'),'dialog');
  assert.match(sheet.textContent,/回应检查结果.*执行失败/s);assert.match(sheet.textContent,/r34-turn/);
  assert.equal(sheet.querySelector('pre'),null,'raw JSON is absent until explicit disclosure');
  await view.click(button(sheet,'原始 JSON'));
  assert.match(sheet.querySelector('pre').textContent,/request_timeout/);
  assert.match(sheet.querySelector('a').href,/correlationId=turn%3Ar34-turn/);
  assert.equal(retries,0);assert.equal(reads,0);
  await act(async()=>sheet.dispatchEvent(new window.KeyboardEvent('keydown',{key:'Escape',bubbles:true})));
  await view.flush();assert.equal(document.querySelector('[data-testid="chat-turn-details"]'),null);
  assert.equal(document.activeElement,trigger,'closing restores the original message action');
 }finally{await view.dispose();}
});

test('Moment author uses publisher identity even inside another character scope',async()=>{
 const {momentAuthor}=await load('approved-ui/feed-presentation.mjs');
 assert.equal(momentAuthor({owner_fluctlight_id:'A',author_actor_id:'owner',author_type:'user',author_name:'青岚'}),'我');
 assert.equal(momentAuthor({owner_fluctlight_id:'A',author_actor_id:'A',author_type:'fluctlight',author_name:'青岚'}),'青岚');
 const {default:FeedPost}=await load('react-app/shared/FeedPost.jsx');
 const model={w:{state:{session:{actorId:'r34-feed'},characters:[{id:'A',name:'青岚'}]},subscribe:()=>()=>{},getSnapshot:()=>model.w.state,current:{value:{id:'A'}},isCurrent:()=>true,services:{mediaClient:{url:id=>'/api/media/'+id}}},service:{comments:async()=>({items:[],next_cursor:''})}};
 const base={id:'owner-post',owner_fluctlight_id:'A',author_actor_id:'owner',author_type:'user',author_name:'青岚',text:'杭州下雨天，一杯桂花乌龙。',status:'visible',revision:1,comment_count:0,media:[{id:'portrait',kind:'image',readable:true}]};
 const view=await mountReact(vite,FeedPost,{workspace:model.w,props:{post:base,model,onAction(){},onOpen(){}}});
 try{
  assert.equal(view.host.querySelector('header strong').textContent,'我');
  assert.equal(view.host.querySelector('img.feed-image').getAttribute('src'),'/api/media/portrait');
  await view.update({post:{...base,author_type:'fluctlight',author_actor_id:'A'},model,onAction(){},onOpen(){}});
  assert.equal(view.host.querySelector('header strong').textContent,'青岚');
 }finally{await view.dispose();}
});
