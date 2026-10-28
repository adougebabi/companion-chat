import {createRequire} from 'node:module';
import assert from 'node:assert/strict';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json');
const {chromium}=require('playwright-core');
const root=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/interaction');
await mkdir(root,{recursive:true});
const browser=await chromium.launch({channel:'chrome',headless:true}),results=[];
async function run(name,fn,{width=1348,height=925}={}){
 const context=await browser.newContext({viewport:{width,height},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'}),page=await context.newPage(),errors=[];
 page.setDefaultTimeout(8000);
 page.on('pageerror',e=>errors.push(e.message));
 const record={name,viewport:{width,height},kind:'real production dist with controlled HTTP DTOs, not live Core',checks:[],errors};
 const check=(label,value)=>{assert.ok(value,label);record.checks.push(label);};
 const shot=async state=>{const path=resolve(root,name+'-'+state+'.png');await page.screenshot({path});(record.screenshots||=[]).push(path);};
 try{await fn({page,check,shot});check('no uncaught browser exception',errors.length===0);record.status='passed';}
 catch(error){record.status='failed';record.error=error.message;record.focus=await page.evaluate(()=>document.activeElement?.outerHTML.slice(0,500));await shot('failure').catch(()=>{});}
 finally{results.push(record);console.log(name,record.status,record.error||record.checks.length);await writeFile(resolve(root,'workspace-ui.json'),JSON.stringify(results,null,2)+'\n');await context.close();}
}
const go=async(page,route)=>{await page.goto('http://127.0.0.1:4381/#'+route,{waitUntil:'networkidle'});await page.locator('h1').first().waitFor();};
try{
 await run('library-search-view-back',async({page,check,shot})=>{
  await go(page,'/library');await page.locator('.library-chips span').first().waitFor();
  check('full directory has three cards',await page.locator('.library-card').count()===3);
  const input=page.getByRole('searchbox',{name:'搜索名字、职业或标签'});
  await input.fill('不存在的名字');check('search produces real empty result',await page.locator('.library-card').count()===0);await shot('no-results');
  await page.getByRole('button',{name:'清除搜索',exact:true}).click();await page.getByRole('button',{name:'列表视图',exact:true}).click();
  check('list mode selected',await page.getByRole('button',{name:'列表视图',exact:true}).getAttribute('aria-pressed')==='true');await shot('list');
  await input.fill('林晚');await page.locator('.library-card').getByRole('button',{name:'角色档案',exact:true}).click();await page.getByRole('button',{name:'编辑资料',exact:true}).waitFor();
  await page.evaluate(()=>history.back());await page.waitForURL(/#\/library$/);await page.locator('.library-card').waitFor();
  check('filter survives profile and back',(await page.getByRole('searchbox',{name:'搜索名字、职业或标签'}).inputValue())==='林晚');
  check('view survives profile and back',await page.getByRole('button',{name:'列表视图',exact:true}).getAttribute('aria-pressed')==='true');
 });
 await run('shell-keyboard-navigation',async({page,check,shot})=>{
  await go(page,'/library');await page.getByRole('button',{name:'收起导航',exact:true}).click();
  check('desktop sidebar collapses',await page.locator('[data-collapsible="icon"]').count()===1);await shot('collapsed');
  await page.keyboard.press('Control+k');await page.getByRole('dialog').waitFor();
  const input=page.getByRole('textbox',{name:'搜索页面或角色名字'});check('quickjump receives keyboard focus',await input.evaluate(e=>e===document.activeElement));
  await input.fill('新的相识');check('creation entry found',await page.getByRole('dialog').getByRole('button',{name:'新的相识'}).count()===1);await shot('quickjump');
  await page.keyboard.press('Escape');await page.getByRole('dialog').waitFor({state:'detached'});check('Escape closes quickjump',await page.getByRole('dialog').count()===0);
  await page.getByRole('button',{name:'快速前往'}).click();await page.getByRole('textbox',{name:'搜索页面或角色名字'}).fill('日程');await page.getByRole('dialog').getByRole('button',{name:'日程',exact:true}).click();await page.waitForURL(/life\/schedule/);
  await page.getByRole('heading',{name:'日程',exact:true}).waitFor();
  check('quickjump navigates to real page',await page.locator('h1').textContent()==='日程');
 });
 await run('chat-input-image-history',async({page,check,shot})=>{
  await go(page,'/c/linwan/chat');await page.getByText('给你看看窗边这个小角落。',{exact:false}).waitFor();
  const input=page.getByRole('textbox',{name:'聊天消息'}),send=page.getByRole('button',{name:'发送',exact:true});
  check('empty draft disables send',await send.isDisabled());await input.fill('  \n ');check('whitespace disables send',await send.isDisabled());
  await input.fill('还未发送的草稿');check('text enables send',await send.isEnabled());await input.press('Shift+Enter');check('Shift Enter adds newline',(await input.inputValue()).includes('\n'));await shot('draft');
  const trigger=page.getByRole('button',{name:'查看对话附件',exact:true});await trigger.click();await page.getByRole('dialog').waitFor();check('attachment detail uses actual media URL',(await page.getByRole('dialog').locator('img').getAttribute('src')).includes('parity-bookshop'));await shot('attachment');
  await page.keyboard.press('Escape');await page.waitForFunction(()=>!document.querySelector('[role=dialog]'));check('attachment dismissal restores source focus',await trigger.evaluate(e=>e===document.activeElement));
  await page.getByRole('button',{name:'会话历史',exact:true}).click();await page.getByRole('dialog').getByText('窗边的一场小雨',{exact:true}).waitFor();await shot('history');
  await page.keyboard.press('Escape');await page.waitForFunction(()=>!document.querySelector('[role=dialog]'));check('history dismissal restores source focus',await page.getByRole('button',{name:'会话历史',exact:true}).evaluate(e=>e===document.activeElement));
  check('draft survived details',(await input.inputValue()).startsWith('还未发送的草稿'));
  await page.getByTestId('chat-preview-toggle').click();check('preview folds',await page.locator('.companion-chat-layout.without-preview').count()===1);
  await page.getByTestId('chat-preview-toggle').click();check('preview unfolds',await page.locator('.companion-moment-preview').count()===1);
 });
 await run('chat-image-error-recovery',async({page,check,shot})=>{
  let fail=true;await page.route('**/api/media/parity-bookshop',route=>fail?route.fulfill({status:503,contentType:'text/plain',body:'test image read failure'}):route.continue());
  await go(page,'/c/linwan/chat');await page.getByText('图片暂时无法读取',{exact:true}).waitFor();await shot('image-failure');
  fail=false;await page.getByRole('button',{name:'重新读取图片',exact:true}).click();await page.locator('.companion-message-image[data-state="done"]').waitFor();
  check('retry reads original image',await page.locator('.companion-message-image img').evaluate(e=>e.complete&&e.naturalWidth>0));await shot('image-recovered');
 });
 await run('shell-mobile-drawer',async({page,check,shot})=>{
  await go(page,'/c/linwan/chat');check('mobile document has no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  await page.getByRole('button',{name:'打开导航',exact:true}).click();const drawer=page.getByRole('dialog');await drawer.waitFor();await shot('navigation');
  await drawer.getByRole('button',{name:'日程',exact:true}).click();await page.waitForURL(/life\/schedule/);await page.waitForFunction(()=>!document.querySelector('[role=dialog]'));
  check('navigation dismisses after route choice',await drawer.count()===0);check('schedule remains within mobile viewport',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
  await page.getByRole('button',{name:'打开导航',exact:true}).click();await page.keyboard.press('Escape');await page.waitForFunction(()=>!document.querySelector('[role=dialog]'));
  await page.waitForFunction(()=>document.activeElement?.getAttribute('aria-label')==='打开导航');
  check('Escape restores navigation trigger focus',await page.getByRole('button',{name:'打开导航',exact:true}).evaluate(e=>e===document.activeElement));
 },{width:390,height:844});
 await run('profile-edit-cancel',async({page,check,shot})=>{
  await go(page,'/c/linwan/profile');await page.getByRole('button',{name:'编辑资料',exact:true}).click();const dialog=page.getByRole('dialog');await dialog.waitFor();await shot('edit');
  const name=dialog.getByLabel('名字',{exact:true});await name.fill('尚未保存的名字');await page.keyboard.press('Escape');await page.getByRole('alertdialog').waitFor();await shot('discard-confirm');
  await page.getByRole('alertdialog').getByRole('button',{name:/放弃|不保存|丢弃/}).click();await page.waitForFunction(()=>!document.querySelector('[role=dialog],[role=alertdialog]'));
  check('cancel preserved saved profile',(await page.locator('.companion-profile-intro h2').textContent()).includes('林晚'));check('unsaved name did not leak',!(await page.locator('.companion-profile-intro h2').textContent()).includes('尚未保存'));
 });
}finally{await browser.close();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;
