import {createRequire} from 'node:module';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import assert from 'node:assert/strict';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json');
const {chromium}=require('playwright-core');
const phase=process.env.AUDIT_PHASE||'pass69';
const out=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/chat-inbox-avatar');
await mkdir(out,{recursive:true});
const browser=await chromium.launch({channel:'chrome',headless:true}),results=[];
async function run(name,size){
 if(process.env.AUDIT_CASES&&!process.env.AUDIT_CASES.split(',').includes(name))return;
 const base='http://127.0.0.1:'+(size==='mobile'?4413:4412),fid='fluctlight-fixture-one';
 const context=await browser.newContext({viewport:{width:size==='mobile'?390:1348,height:size==='mobile'?844:925},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'}),page=await context.newPage();
 page.setDefaultTimeout(10000);
 const record={name,size,kind:'production dist / controlled HTTP; no live model, workflow or image identity proof',checks:[],requests:[],errors:[],screenshots:[]};
 page.on('pageerror',e=>record.errors.push(e.message));
 page.on('request',r=>{if(new URL(r.url()).pathname.startsWith('/api/'))record.requests.push({path:new URL(r.url()).pathname,method:r.method(),body:r.postDataJSON()});});
 const check=(label,value)=>{assert.ok(value,label);record.checks.push(label);};
 const dialog=()=>page.getByRole('dialog').last();
 const writes=()=>record.requests.filter(r=>!['GET','HEAD'].includes(r.method));
 const go=async path=>{await page.goto(base+'/#'+path,{waitUntil:'networkidle'});await page.locator('h1').first().waitFor();};
 const shot=async state=>{
  await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));});
  const path=resolve(out,phase+'-'+name+'-'+size+'-'+state+'.png');await page.screenshot({path,animations:'disabled'});record.screenshots.push(path);
  check(state+': no document overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  if(await dialog().count()){const b=await dialog().boundingBox();check(state+': dialog within viewport',b.x>=0&&b.x+b.width<=page.viewportSize().width+.5&&b.y>=0&&b.y+b.height<=page.viewportSize().height+.5);}
 };
 try{
  if(name==='retry-original'){
   await go('/c/'+fid+'/chat');await page.getByRole('button',{name:'重试原回应',exact:true}).waitFor();await shot('failed-original');
   await page.getByRole('button',{name:'重试原回应',exact:true}).click();await page.getByText('这是隔离验收服务的模拟回复。',{exact:true}).waitFor();
   const turn=writes().filter(r=>r.path.endsWith('/turn'));check('retry preserves original turn, idempotency key and user text',turn.length===1&&turn[0].body.turnId==='turn-fixture-failed'&&turn[0].body.idempotencyKey==='key-fixture-failed'&&turn[0].body.text==='请查看回复失败时的反馈。');
   check('retry does not duplicate original user message',await page.locator('.companion-message.is-mine').count()===1);await page.reload({waitUntil:'networkidle'});await page.getByText('这是隔离验收服务的模拟回复。',{exact:true}).waitFor();check('reloaded conversation contains one original user message',await page.locator('.companion-message.is-mine').count()===1);await shot('retried-readback');
  }else if(name==='history-recovery'){
   let failed=false;await page.route('**/api/fluctlights/'+fid+'/conversations?*',async route=>{if(!failed){failed=true;await route.fulfill({status:503,json:{code:'request_failed',message:'受控会话目录读取失败'}});}else await route.continue();});
   await go('/c/'+fid+'/chat');const draft=page.getByRole('textbox',{name:'聊天消息',exact:true});await draft.fill('阅读历史时保留当前草稿');await page.getByRole('button',{name:'会话历史',exact:true}).click();await dialog().getByRole('button',{name:'重新读取历史',exact:true}).waitFor();await shot('history-failed');
   await dialog().getByRole('button',{name:'重新读取历史',exact:true}).click();await dialog().getByRole('button',{name:/隔离样本会话/}).click();await dialog().getByText('请查看回复失败时的反馈。',{exact:true}).waitFor();check('reading history does not select or send',!writes().some(r=>r.path.endsWith('/select')||r.path.endsWith('/turn')));
   await dialog().getByRole('textbox',{name:'搜索全部会话',exact:true}).fill('不存在的历史关键词');await dialog().getByRole('button',{name:'搜索',exact:true}).click();await dialog().getByText('当前范围没有匹配消息。',{exact:true}).waitFor();await shot('history-no-match');
   await dialog().getByRole('button',{name:/隔离样本会话/}).click();await dialog().getByRole('button',{name:'继续这段对话',exact:true}).click();await page.getByRole('dialog').waitFor({state:'detached'});check('only explicit continue selects one original conversation',writes().filter(r=>r.path.endsWith('/select')).length===1);check('continuing history preserves current draft',await draft.inputValue()==='阅读历史时保留当前草稿');await shot('continued');
  }else if(name==='stop-failure'){
   const pending=async route=>{const r=await route.fetch(),v=await r.json();v.messages=v.messages.filter(m=>m.kind==='user'||m.id==='message-fixture-one').map(m=>m.kind==='user'?{...m,turnStatus:'running',turnRetryable:false}:m);await route.fulfill({response:r,json:v});};
   await page.route('**/api/fluctlights/'+fid+'/conversation',pending);await page.route('**/api/conversations/conversation-fixture/messages?*',pending);
   await page.route('**/api/conversations/conversation-fixture/turn/turn-fixture-failed/cancel',route=>route.fulfill({status:503,json:{code:'request_failed',message:'受控取消回执暂未确认'}}));
   await go('/c/'+fid+'/chat');await page.getByRole('button',{name:'停止本次生成',exact:true}).click();await page.getByRole('alert').filter({hasText:'取消请求结果尚未确认，请刷新进度核对。原消息已保留。'}).waitFor();
   check('stop uses the current original turn URL',writes().filter(r=>r.path.endsWith('/cancel')).length===1);check('stop failure preserves original user message',await page.getByText('请查看回复失败时的反馈。',{exact:true}).count()===1);check('unconfirmed stop does not send another turn',!writes().some(r=>r.path.endsWith('/turn')));await shot('unconfirmed-stop');
  }else if(name==='inbox-source'){
   await go('/inbox');await page.getByRole('button').filter({has:page.getByRole('heading',{name:'日常准备需要继续',exact:true})}).click();await dialog().getByRole('heading',{name:'日常准备需要继续',exact:true}).waitFor();await shot('notification-detail');await dialog().getByRole('button',{name:'返回通知',exact:true}).click();await page.getByRole('dialog').waitFor({state:'detached'});
   await page.getByRole('button').filter({has:page.getByRole('heading',{name:'日常准备需要继续',exact:true})}).click();await dialog().getByRole('button',{name:'查看原始来源',exact:true}).click();await page.waitForURL(/diag\/task\/task-fixture\?returnTo=%23%2Finbox/);await page.getByRole('button',{name:'重试任务',exact:true}).waitFor();await page.getByRole('button',{name:'重试任务',exact:true}).click();await dialog().getByRole('textbox',{name:'操作原因',exact:true}).fill('从通知核对原任务，仅查看后取消');await shot('original-task-review');await page.keyboard.press('Escape');await page.getByRole('dialog').waitFor({state:'detached'});await page.getByRole('button',{name:'返回来源页',exact:true}).click();await page.waitForURL(/#\/inbox$/);check('notification source and task return preserve original context',true);check('reviewing original failed task never restarts it',!writes().some(r=>r.path.startsWith('/api/workflow-controls')));
  }else if(name==='inbox-back'){
   await go('/inbox');await page.getByRole('button',{name:'和林岚 · 验收样本聊聊',exact:true}).click();await page.waitForURL(/fluctlight-fixture-one\/chat/);check('return to daily chat targets current character',true);check('return link never sends a chat turn',!writes().some(r=>r.path.endsWith('/turn')));await shot('back-to-chat');
  }else if(name==='inbox-source-errors'){
   await page.route('**/api/notifications?*',async route=>{const r=await route.fetch(),v=await r.json();v.items=[{...v.items[0],link:'https://invalid.fixture.test/private',read_at:null}];await route.fulfill({response:r,json:v});});
   await page.route('**/api/notifications/*/read',route=>route.fulfill({status:503,json:{message:'受控已读写入失败'}}));
   await go('/inbox');await page.getByRole('button').filter({has:page.getByRole('heading',{name:'日常准备需要继续',exact:true})}).click();await page.getByText(/通知可继续查看，已读状态暂未确认/).waitFor();await dialog().getByRole('button',{name:'查看原始来源',exact:true}).click();await page.getByText('这项记录没有可访问的来源链接，请重新读取或前往运行诊断核对。',{exact:true}).waitFor();check('invalid source leaves user in notification dialog',page.url().endsWith('#/inbox')&&await dialog().isVisible());check('failed mark retains unread marker',await page.locator('.replica-unread-dot').count()===1);await shot('invalid-source-and-read-failure');
  }else if(name==='inbox-groups'){
   await page.route('**/api/pending?*',async route=>{const r=await route.fetch(),v=await r.json();v.groups=v.groups.map(g=>g.kind==='capability'?{...g,status:'unavailable',message:'受控能力来源待读取'}:g.kind==='owner_request'?{...g,count:1,items:[{id:'owner-pending',title:'原请求正在处理',status:'running',fluctlight_id:fid,href:'/diag/request/owner-pending'}]}:g);await route.fulfill({response:r,json:v});});
   await go('/inbox');await page.getByRole('heading',{name:'正在处理的请求',exact:true}).waitFor();await page.getByRole('heading',{name:'服务状态',exact:true}).waitFor();await page.getByText('受控能力来源待读取',{exact:true}).waitFor();check('source failure is labelled unread, not zero',await page.getByText('尚未读全',{exact:true}).count()>0);check('running request stays separate from decisions',await page.getByRole('button',{name:/原请求正在处理/}).count()===1);await page.getByRole('button',{name:'重新读取待处理来源',exact:true}).click();await page.getByRole('button',{name:/原请求正在处理/}).waitFor();await shot('separate-source-categories');
  }else if(name==='avatar-all'){
   await go('/c/'+fid+'/media?tab=avatar');await page.getByRole('button',{name:/选择一张媒体图片/}).click();await page.getByRole('button',{name:'选择头像 午后的窗边',exact:true}).click();const inline=page.locator('[data-slot="workspace-command-inline"]');await inline.getByRole('textbox',{name:'选择原因',exact:true}).fill('核对同一真实目录头像在各展示位置的来源');await inline.getByRole('button',{name:'保存显示头像',exact:true}).click();await page.getByText('显示头像已保存并读回。',{exact:true}).waitFor();
   for(const [route,selector] of [['/library','.library-card'],['/c/'+fid+'/chat','.companion-chat-heading'],['/c/'+fid+'/profile','.replica-profile'],['/c/'+fid+'/people?tab=relationship','.companion-relationship-avatars'],['/feed?fid='+fid,'.feed-author-avatar']]){
    await go(route);const images=page.locator(selector+' img[src="/api/media/replica-landscape"]');await images.first().waitFor();check('saved avatar is loaded in '+route,await images.first().evaluate(e=>e.complete&&e.naturalWidth>0));if(route.endsWith('/chat')){await page.locator('.replica-message-avatar img[src="/api/media/replica-landscape"]').first().waitFor();check('assistant messages use same avatar as chat header',true);}await shot('surface-'+route.replace(/[^a-z]+/g,'-'));
   }
   await go('/c/'+fid+'/chat');if(size==='mobile')await page.getByRole('button',{name:'打开导航',exact:true}).click();await page.locator('.replica-workspace-switch img[src="/api/media/replica-landscape"]').waitFor();check('sidebar uses same saved directory avatar',true);if(size==='mobile')await page.getByRole('button',{name:'关闭导航',exact:true}).click();
   await page.route('**/api/media/replica-landscape',route=>route.fulfill({status:503,body:''}));await page.reload({waitUntil:'networkidle'});await page.locator('.companion-chat-heading [data-slot=avatar-fallback]').waitFor();check('broken avatar has original first-character fallback',await page.locator('.companion-chat-heading [data-slot=avatar-fallback]').innerText()==='林');check('failed image never hides current character name',await page.locator('.companion-chat-person').innerText().then(t=>t.includes('林岚')));await shot('image-failure-fallback');
  }
  check('no uncaught browser exception',record.errors.length===0);record.status='passed';
 }catch(e){record.status='failed';record.error=e.message;record.text=await page.locator('body').innerText().catch(()=>null);await shot('failure').catch(()=>{});}
 finally{console.log(name,size,record.status,record.error||record.checks.length);results.push(record);await writeFile(resolve(out,'report-'+phase+'.json'),JSON.stringify(results,null,2)+'\n');await context.close();}
}
try{for(const size of ['desktop','mobile'])for(const name of ['retry-original','history-recovery','stop-failure','inbox-source','inbox-back','inbox-source-errors','inbox-groups','avatar-all'])await run(name,size);}finally{await browser.close();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;
