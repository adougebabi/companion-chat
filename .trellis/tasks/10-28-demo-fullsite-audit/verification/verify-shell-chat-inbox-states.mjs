import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {startOwnedFixture} from './owned-fixture.mjs';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core');
const out=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/shell-chat-inbox-states-'+(process.env.AUDIT_PHASE||'pass117'));
await mkdir(out,{recursive:true});const browser=await chromium.launch({channel:'chrome',headless:true}),results=[];
try{for(const [size,width,height] of [['desktop',1348,925],['mobile',390,844]])for(const name of ['logout-failure','quickjump-empty','inbox-bulk','inbox-no-role','chat-composition','chat-states']){
  if(process.env.AUDIT_CASES&&!process.env.AUDIT_CASES.split(',').includes(name))continue;
  const server=await startOwnedFixture({port:Number(process.env.SHELL_STATES_PORT||4465),env:{FLUCTLIGHT_UI_FIXTURE_MANAGEMENT:'normal',FLUCTLIGHT_UI_FIXTURE_CHAT:'normal'}});let context,page,release;
  const record={name,size,width,height,boundary:'actual production components / fresh controlled HTTP / no real logout, chat/model or notification mutation',checks:[],requests:[],errors:[],screenshots:[]};
  try{
    context=await browser.newContext({viewport:{width,height},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'});page=await context.newPage();page.setDefaultTimeout(10000);
    page.on('pageerror',e=>record.errors.push(e.message));page.on('request',r=>{const path=new URL(r.url()).pathname;if(path.startsWith('/api/')||path.startsWith('/auth/'))record.requests.push({path,method:r.method(),body:r.postDataJSON()});});
    const check=(label,value)=>{assert.ok(value,label);record.checks.push(label);};
    const shot=async state=>{await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));});check(state+': no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));const file=resolve(out,name+'-'+size+'-'+state+'.png');await page.screenshot({path:file,animations:'disabled'});record.screenshots.push(file);};
    const go=async path=>{await page.goto(server.base+'/#'+path,{waitUntil:'networkidle'});await page.locator('h1').first().waitFor();};
    const writes=()=>record.requests.filter(r=>!['GET','HEAD'].includes(r.method));
    if(name==='logout-failure'){
      const gate=new Promise(ok=>{release=ok;});let first=true;
      await page.route(server.base+'/auth/logout',async route=>{if(first){first=false;await gate;await route.fulfill({status:503,json:{code:'request_failed',message:'受控退出暂未确认'}});}else await route.continue();});
      await go('/library');if(size==='mobile')await page.getByRole('button',{name:'打开导航',exact:true}).click();
      const account=()=>page.getByRole('button',{name:'账户菜单',exact:true});await account().click();await page.getByRole('menuitem',{name:'退出登录',exact:true}).click();
      await account().click();await page.getByRole('menuitem',{name:'正在退出…',exact:true}).waitFor();check('busy disables another logout',await page.getByRole('menuitem',{name:'正在退出…',exact:true}).isDisabled());await shot('busy');release();release=null;
      await page.locator('[data-sonner-toast]').filter({hasText:/受控退出|暂时|失败/}).first().waitFor();check('failed logout retains private workspace',await page.getByTestId('replica-library').count()===1);await page.keyboard.press('Escape');await account().click();await page.getByRole('menuitem',{name:'退出登录',exact:true}).waitFor();await shot('failure');
      await page.getByRole('menuitem',{name:'退出登录',exact:true}).click();await page.getByRole('heading',{name:'欢迎回来',exact:true}).waitFor();check('explicit retry exits after authoritative acknowledgement',writes().filter(r=>r.path==='/auth/logout').length===2);await shot('retried');
    }else if(name==='quickjump-empty'){
      await go('/library');const trigger=page.getByRole('button',{name:/快速前往/});await trigger.click();const input=page.getByRole('textbox',{name:'搜索页面或角色名字',exact:true});await input.fill('不存在的页面xyz');await page.getByRole('status').filter({hasText:'没有找到匹配的页面或角色。'}).waitFor();check('no-result leaves query editable',await input.inputValue()==='不存在的页面xyz');await shot('empty');await page.keyboard.press('Escape');await page.getByRole('dialog').waitFor({state:'detached'});check('Escape returns to original quickjump trigger',await trigger.evaluate(e=>e===document.activeElement));await trigger.click();check('reopening clears dismissed query',await input.inputValue()==='');
    }else if(name==='inbox-no-role'){
      await page.route(server.base+'/api/fluctlights',route=>route.fulfill({json:[]}));await go('/inbox');await page.locator('.replica-inbox-toolbar').waitFor();check('empty directory hides scoped chat return',await page.locator('.replica-inbox-note').count()===0&&await page.getByRole('button',{name:/^和.*聊聊/}).count()===0);await shot('no-role');check('global inbox never sends to an invented role',!writes().some(r=>r.path.endsWith('/turn')));
    }else if(name==='inbox-bulk'){
      const gate=new Promise(ok=>{release=ok;});await page.route('**/api/notifications/read-all',async route=>{await gate;await route.continue();});
      await go('/inbox');await page.getByRole('button',{name:'全部标为已读',exact:true}).waitFor();await page.getByRole('button',{name:'全部标为已读',exact:true}).click();await page.getByRole('button',{name:'正在确认已读…',exact:true}).waitFor();check('bulk busy prevents duplicate mutation',await page.getByRole('button',{name:'正在确认已读…',exact:true}).isDisabled());await shot('busy');release();release=null;
      await page.getByRole('button',{name:'全部标为已读',exact:true}).waitFor();const commands=writes().filter(r=>r.path==='/api/notifications/read-all');check('one frozen workspace read snapshot only',commands.length===1&&typeof commands[0].body.before==='string'&&commands[0].body.before.length>0);await shot('confirmed');
    }else if(name==='chat-composition'){
      await go('/c/fluctlight-fixture-one/chat');const input=page.getByRole('textbox',{name:'聊天消息',exact:true});await input.fill('输入法尚在组词');
      await input.evaluate(e=>e.dispatchEvent(new CompositionEvent('compositionstart',{bubbles:true,data:'尚在组词'})));await input.press('Enter');check('composition Enter does not submit a turn',!writes().some(r=>r.path.endsWith('/turn')));await shot('composing');
      await input.evaluate(e=>e.dispatchEvent(new CompositionEvent('compositionend',{bubbles:true,data:'尚在组词'})));const response=page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname.endsWith('/turn'));await input.press('Enter');assert.equal((await response).status(),200);await page.getByText('这是隔离验收服务的模拟回复。',{exact:true}).waitFor();check('after composition exactly one original text is sent',writes().filter(r=>r.path.endsWith('/turn')).length===1&&writes().find(r=>r.path.endsWith('/turn')).body.text==='输入法尚在组词');await shot('completed');
    }else{
      let state='running';
      const project=async route=>{const response=await route.fetch(),value=await response.json();value.messages=value.messages.map(m=>m.kind==='user'?{...m,turnStatus:state==='settling'?'failed':state,turnRetryable:state==='cancelled',turnErrorCode:state==='cancelled'?'reply_cancelled':state==='settling'?'agent_run_failed':undefined,createdAt:'2026-10-01T01:00:00Z'}:m);await route.fulfill({response,json:value});};
      await page.route('**/api/fluctlights/fluctlight-fixture-one/conversation',project);await page.route('**/api/conversations/conversation-fixture/messages?*',project);
      await go('/c/fluctlight-fixture-one/chat');await page.getByText('林岚 · 验收样本还在想…',{exact:true}).waitFor();check('long running turn exposes diagnostics and honest waiting',await page.getByRole('button',{name:'停止本次生成',exact:true}).count()===1);await shot('slow');
      state='settling';await page.reload({waitUntil:'networkidle'});await page.getByRole('button',{name:'查看状态',exact:true}).waitFor();check('settling cannot offer model retry or cancellation',await page.getByRole('button',{name:'重试原回应',exact:true}).count()===0&&await page.getByRole('button',{name:'停止本次生成',exact:true}).count()===0);await shot('settling');
      state='cancelled';await page.reload({waitUntil:'networkidle'});await page.getByRole('button',{name:'重试原回应',exact:true}).waitFor();await page.getByRole('alert').filter({hasText:'本次处理被中断，消息已保留。这不一定是手动取消，可查看本次运行记录。'}).waitFor();check('actual cancelled turn retains original user bubble',await page.getByText('请查看回复失败时的反馈。',{exact:true}).count()===1);await shot('cancelled');check('read-only status rotation never creates a turn',!writes().some(r=>r.path.endsWith('/turn')));
    }
    check('no browser exception',record.errors.length===0);record.status='passed';
  }catch(e){record.status='failed';record.error=e.stack;record.text=await page?.locator('body').innerText().catch(()=>null);await page?.screenshot({path:resolve(out,name+'-'+size+'-failure.png'),animations:'disabled'}).catch(()=>{});}
  finally{release?.();try{await context?.close();}finally{await server.stop();}}
  results.push(record);await writeFile(resolve(out,'report.json'),JSON.stringify(results,null,2)+'\n');console.log(name,size,record.status,record.error||record.checks.length);
}}finally{await browser.close();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;
