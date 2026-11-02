import assert from 'node:assert/strict';
import {mkdir,writeFile} from 'node:fs/promises';
import {startOwnedFixture} from '../10-28-demo-fullsite-audit/verification/owned-fixture.mjs';
import {chromium} from '/tmp/r22-browser/node_modules/playwright-core/index.mjs';

const before=process.argv.includes('--baseline'),shots='/tmp/r25-shots',results=[],errors=[];
await mkdir(shots,{recursive:true});
const fixture=await startOwnedFixture({port:4285,env:{FLUCTLIGHT_UI_FIXTURE_MANAGEMENT:'normal'}});
const browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true,args:['--no-sandbox']});
const fid='fluctlight-fixture-one';
try {
 for(const [size,width,height] of [['desktop',1920,1113],['mobile',390,844]]){
  const context=await browser.newContext({viewport:{width,height},locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'}),page=await context.newPage();
  page.setDefaultTimeout(10000);page.on('pageerror',error=>errors.push({size,message:error.message}));
  await page.route(/\/api\/diagnostic-records\/workflow-intents(?:\?|$)/,route=>route.fulfill({json:{items:[{ref:'wake-up',occurred_at:'2026-11-02T08:41:00Z',data:{id:'wake-up',title:'日常唤醒',intent_type:'wake_up.current',fluctlight_id:fid,status:'pending'}}],next_cursor:''}}));
  await page.route(/\/api\/fluctlights\/[^/]+\/diary(?:\?|$)/,route=>route.fulfill({json:{items:[],next_cursor:''}}));
  await page.route(/\/api\/notifications(?:\?|$)/,route=>route.fulfill({json:{items:[],as_of:'2026-11-02T00:00:00Z',next_cursor:''}}));
  const go=async path=>{await page.goto(fixture.base+'/#'+path,{waitUntil:'networkidle'});await page.locator('h1').first().waitFor();};
  const shot=async name=>{await page.evaluate(()=>document.fonts.ready);await page.screenshot({path:`${shots}/${before?'before-':''}${size}-${name}.png`,fullPage:true,animations:'disabled'});};
  const check=async(item,run)=>{console.log('CHECK '+size+' '+item);try{const evidence=await run();assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'no horizontal overflow');results.push({size,item,status:'passed',...evidence});}catch(error){results.push({size,item,status:'failed',message:error.message});console.log('FAIL '+size+' '+item+' '+error.message);if(!before)throw error;}};
  await check('P1-4',async()=>{
   await go('/diag?view=tasks');const row=page.locator('.replica-task-row').first();await row.waitFor();
   const title=await row.locator('h3').innerText(),metadata=await row.locator('p').innerText(),badge=await row.locator('[data-slot=badge]').evaluate(n=>({font:parseFloat(getComputedStyle(n).fontSize),height:n.getBoundingClientRect().height}));await shot('diagnostics');
   assert.equal(title,'日常唤醒');assert.doesNotMatch(metadata,/日常唤醒/);assert.match(metadata,/16:41/);assert.ok(badge.font>=12&&badge.height>=28,JSON.stringify(badge));return {title,metadata,badge};
  });
  await check('P2-2',async()=>{
   if(size==='mobile')await page.getByRole('button',{name:'打开导航',exact:true}).click();
   const trigger=page.locator('[aria-label=切换当前角色]');await trigger.click();const menu=page.getByRole('menu');await menu.waitFor();await menu.evaluate(async n=>Promise.all(n.getAnimations({subtree:true}).map(a=>a.finished)));
   const a=await trigger.boundingBox(),b=await menu.boundingBox();await shot('character-menu');assert.ok(Math.abs(a.x-b.x)<=1.5&&Math.abs(a.width-b.width)<=1.5,JSON.stringify({a,b}));
   const reopenNavigation=async()=>{if(size==='mobile'){await trigger.waitFor({state:'hidden'});await page.getByRole('button',{name:'打开导航',exact:true}).click();}};
   const firstName=await trigger.innerText();await menu.getByRole('menuitem').nth(1).click();await reopenNavigation();assert.notEqual(await trigger.innerText(),firstName,'actual workspace role selection updates the trigger');
   await trigger.click();await menu.getByRole('menuitem').first().click();await reopenNavigation();assert.equal(await trigger.innerText(),firstName);await trigger.focus();await page.keyboard.press('ArrowDown');await menu.waitFor();await page.keyboard.press('Escape');if(size==='mobile')await page.keyboard.press('Escape');return {trigger:a,menu:b,selection:'actual workspace switched and restored'};
  });
  await check('P2-1',async()=>{
   await go('/settings?section='+encodeURIComponent('角色自主与主动联系'));const toggle=page.getByRole('switch',{name:'启用安静时段',exact:true});await toggle.waitFor();await toggle.scrollIntoViewIfNeeded();
   const geometry=await toggle.evaluate(n=>{const row=n.closest('[data-slot=field]'),r=row.getBoundingClientRect(),s=getComputedStyle(row),label=row.querySelector('label').getBoundingClientRect(),select=row.parentElement.querySelector('[role=combobox]').getBoundingClientRect();return {borders:[s.borderTopWidth,s.borderRightWidth,s.borderBottomWidth,s.borderLeftWidth],right:r.right-n.getBoundingClientRect().right,left:label.x-r.x,alignment:Math.abs(r.bottom-select.bottom),stacked:select.top>=r.bottom};});await shot('quiet-hours');await page.locator('.replica-quiet-card').screenshot({path:`${shots}/${before?'before-':''}${size}-quiet-hours-detail.png`,animations:'disabled'});
   assert.deepEqual(geometry.borders,['0px','0px','0px','0px']);assert.ok(geometry.right<=2&&geometry.left<=2&&(size==='desktop'?geometry.alignment<=2:geometry.stacked),JSON.stringify(geometry));
   const checked=await toggle.getAttribute('aria-checked');await toggle.click();assert.notEqual(await toggle.getAttribute('aria-checked'),checked);await toggle.click();assert.equal(await toggle.getAttribute('aria-checked'),checked);return {geometry};
  });
  await check('P2-3',async()=>{
   await go('/c/'+fid+'/life/diary');await page.getByRole('heading',{name:'日记本还是空的',exact:true}).waitFor();assert.equal(await page.getByText('没有找到这段回忆。',{exact:true}).count(),0);assert.equal(await page.locator('[data-slot=empty]').count(),1);assert.equal(await page.locator('[data-slot=empty] .lucide-leaf').count(),1);await shot('diary-empty');
   await page.locator('[aria-label=搜索日记]').fill('不存在的回忆');assert.equal(await page.locator('[data-slot=empty]').count(),1);assert.equal(await page.getByText('没有找到这段回忆。',{exact:true}).count(),0);return {emptyCount:1};
  });
  await check('P2-4',async()=>{
   await go('/inbox');const panel=page.locator('.replica-notice-panel'),empty=panel.locator('[data-slot=empty]');await empty.waitFor();assert.equal(await empty.locator('.lucide-leaf').count(),1);
   const borders=await empty.evaluate(n=>[n,n.parentElement].map(el=>({slot:el.dataset.slot,border:getComputedStyle(el).borderTopWidth,style:getComputedStyle(el).borderTopStyle})));await shot('notifications-empty');assert.ok(borders.every(b=>parseFloat(b.border)===0),JSON.stringify(borders));
   await page.getByRole('tab',{name:/未读/}).click();await page.getByRole('button',{name:'查看全部通知',exact:true}).click();await page.getByRole('heading',{name:'暂时没有通知',exact:true}).waitFor();return {borders};
  });
  await check('P2-5',async()=>{
   await go('/library/create');await page.getByTestId('wizard-world-next').waitFor();await shot('create-world');assert.equal(await page.locator('.replica-eyebrow').first().innerText(),'新的相识');
   await page.locator('label.creation-daily').click({position:{x:30,y:30}});await page.getByTestId('wizard-world-next').click();await page.locator('.creation-aside').waitFor({state:'attached'});await shot('create-character');
   const sidebar=await page.locator('.creation-aside .replica-eyebrow').textContent();assert.equal(sidebar,'新的篇章');assert.doesNotMatch(await page.locator('.replica-creation').innerText(),/A NEW BEGINNING|YOUR NEW CHAPTER/);return {sidebar};
  });
  await check('P2-8',async()=>{
   await go('/data');const upload=page.locator('.replica-data-upload');await upload.waitFor();await upload.scrollIntoViewIfNeeded();const frames=await upload.evaluate(n=>[n,...n.querySelectorAll('*')].filter(el=>getComputedStyle(el).display!=='none'&&parseFloat(getComputedStyle(el).borderTopWidth)>0).map(el=>({tag:el.tagName,slot:el.dataset.slot,style:getComputedStyle(el).borderTopStyle})));await shot('data-import');await page.locator('.replica-data-import').screenshot({path:`${shots}/${before?'before-':''}${size}-data-import-detail.png`,animations:'disabled'});
   assert.equal(frames.length,1,JSON.stringify(frames));assert.equal(frames[0].style,'dashed');
   const picker=upload.locator('input[type=file]:not([data-domain-carrier])');await picker.setInputFiles({name:'r25-settings.json',mimeType:'application/json',buffer:Buffer.from('{}')});await upload.getByText('r25-settings.json',{exact:true}).waitFor();assert.equal(await page.getByRole('textbox',{name:'设定文件内容（JSON）',exact:true}).inputValue(),'{}');return {frames,selection:'forwarded to actual domain controller'};
  });
  await context.close();
 }
 assert.deepEqual(errors,[]);await writeFile(`${shots}/${before?'baseline':'browser-results'}.json`,JSON.stringify({status:results.every(r=>r.status==='passed')?'passed':'failed',boundary:'current production dist + isolated HTTP fixture, never the live API',results,errors},null,2)+'\n');console.log(JSON.stringify(results,null,2));
}finally{await browser.close();await fixture.stop();}
