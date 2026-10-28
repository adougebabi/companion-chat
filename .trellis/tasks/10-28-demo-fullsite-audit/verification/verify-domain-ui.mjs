import {createRequire} from 'node:module';
import assert from 'node:assert/strict';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core');
const root=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/interaction');await mkdir(root,{recursive:true});
const browser=await chromium.launch({channel:'chrome',headless:true}),results=[];
const selected=process.env.AUDIT_CASES?.split(',');
async function run(name,fn){
 if(selected&&!selected.includes(name))return;
 const context=await browser.newContext({viewport:{width:1348,height:925},locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'}),page=await context.newPage();page.setDefaultTimeout(8000);
 const record={name,kind:'production dist / controlled HTTP only',checks:[],errors:[]};page.on('pageerror',e=>record.errors.push(e.message));
 const check=(text,value)=>{assert.ok(value,text);record.checks.push(text);};
 const shot=async state=>{const file=resolve(root,name+'-'+state+'.png');await page.screenshot({path:file});(record.screenshots||=[]).push(file);};
 try{await fn({page,check,shot,context});check('no uncaught browser exception',record.errors.length===0);record.status='passed';}catch(e){record.status='failed';record.error=e.message;record.text=await page.locator('body').innerText().catch(()=>null);await shot('failure').catch(()=>{});}finally{results.push(record);console.log(name,record.status,record.error||record.checks.length);await writeFile(resolve(root,'domain-ui'+(selected?'-'+selected.join('-'):'')+'.json'),JSON.stringify(results,null,2)+'\n');await context.close();}
}
const go=async(page,route)=>{await page.goto('http://127.0.0.1:4381/#'+route,{waitUntil:'networkidle'});await page.locator('h1').first().waitFor();};
const close=async(page)=>{await page.keyboard.press('Escape');await page.getByRole('dialog').waitFor({state:'detached'});};
try{
 await run('schedule-detail-date',async({page,check,shot})=>{
  await go(page,'/c/linwan/life/schedule');await page.getByTestId('schedule-14:00').click();await page.getByRole('dialog').waitFor();await shot('detail');
  check('detail retains selected activity',(await page.getByRole('dialog').innerText()).includes('整理新到的书'));await close(page);
  await page.getByRole('button',{name:'后一天',exact:true}).click();await page.getByText('准备周末的小型读书会',{exact:true}).waitFor();check('tomorrow reads its actual arrangement',await page.locator('.lp-timeline-item').count()===1);await page.getByRole('button',{name:'后一天',exact:true}).click();await page.waitForFunction(()=>!document.querySelector('.lp-timeline-item')&&document.body.textContent.includes('10月29日'));await shot('empty-day');
  check('date is reflected by empty schedule',await page.locator('.lp-timeline-item').count()===0);
  await page.getByRole('button',{name:'今天',exact:true}).click();await page.getByTestId('schedule-14:00').waitFor();check('today restores five confirmed items',await page.locator('.lp-timeline-item').count()===5);
 });
 await run('goals-search-edit',async({page,check,shot})=>{
  await go(page,'/c/linwan/life/goals');const search=page.getByRole('textbox',{name:/搜索目标/});await search.fill('不存在的目标');await shot('no-results');check('goal search has no result',await page.locator('.lp-goal-card').count()===0);
  await search.fill('');await page.getByRole('button',{name:'添加目标',exact:true}).click();await page.getByRole('dialog').waitFor();await shot('create');
  const submit=page.getByRole('dialog').getByRole('button',{name:'保存目标',exact:true});await submit.click();check('blank target stays in editor',await page.getByRole('dialog').count()===1);check('blank target reports validation',(await page.getByRole('dialog').innerText()).includes('请'));
  await close(page);check('cancel did not add a goal',await page.locator('.lp-goal-card').count()===2);
 });
 await run('wardrobe-search-detail',async({page,check,shot})=>{
  await go(page,'/c/linwan/life/wardrobe');const search=page.getByRole('textbox',{name:'搜索衣服'});await search.fill('不存在的衣服');await shot('no-results');check('wardrobe search empty',await page.locator('.r-garment-card').count()===0);
  await search.fill('针织');await page.locator('.r-garment-image').first().click();await page.getByRole('dialog').waitFor();await shot('detail');check('detail retains exact garment',await page.getByRole('dialog').getByRole('textbox',{name:'衣服名称',exact:true}).inputValue()==='奶油色针织开衫');await close(page);
  check('closing preserves search',await search.inputValue()==='针织');await search.fill('');await page.getByRole('tab',{name:/穿搭方案/}).click();await shot('outfits');check('two saved outfits shown',await page.locator('.r-outfit-card').count()===2);
 });
 await run('diary-read-annotation',async({page,check,shot})=>{
  await go(page,'/c/linwan/life/diary');await page.getByRole('article',{name:'日记正文'}).waitFor();check('real full diary body renders',(await page.locator('.lp-paper-body').innerText()).includes('有人推门进来'));await shot('read');
  await page.getByRole('button',{name:'写一点批注',exact:true}).click();await shot('annotation-edit');const input=page.getByRole('textbox',{name:/批注/}).first();await input.fill('还未保存的批注');
  await page.getByRole('button',{name:'取消',exact:true}).click();check('cancel does not publish annotation',!(await page.locator('.lp-paper-footer').innerText()).includes('还未保存的批注'));
  await page.getByRole('tab',{name:'共同经历',exact:true}).click();await shot('shared-empty');check('shared view shows its empty state',await page.locator('.lp-paper').count()===0);
 });
 await run('memory-source-detail',async({page,check,shot})=>{
  await go(page,'/c/linwan/growth');await page.locator('.lp-memory-card').first().waitFor();check('full active memory list displayed',await page.locator('.lp-memory-card').count()>0);await page.locator('.lp-memory-card').first().click();await page.getByRole('dialog').waitFor();await shot('detail');
  check('detail includes source/history',(await page.getByRole('dialog').innerText()).includes('来源'));await close(page);
  await page.getByRole('tab',{name:/反思/}).click();await shot('reflections');check('reflection source renders',await page.getByText('让联系自然一些',{exact:true}).count()===1);
  await page.getByRole('tab',{name:/自我认知/}).click();await shot('self');check('self claims source renders',await page.getByText('我想把书店变成一个让人愿意慢下来的地方。',{exact:false}).count()>0);
 });
 await run('world-search-detail',async({page,check,shot})=>{
  await go(page,'/c/linwan/world');await page.getByTestId('world-grid').locator('.r-world-card').first().waitFor();const search=page.getByRole('searchbox',{name:'搜索世界名称或类型'});await search.fill('不存在的世界');check('world search no matches',await page.locator('.r-world-card').count()===0);await shot('no-results');
  await search.fill('青岚');await page.getByRole('button',{name:'进入世界',exact:true}).click();await page.getByRole('heading',{name:'青岚小城',exact:true}).waitFor();await shot('places');
  check('world has original place names',await page.getByRole('heading',{name:'迟山',exact:true}).count()>0);
 });
 await run('people-about-detail',async({page,check,shot})=>{
  await go(page,'/c/linwan/people');await page.getByRole('button',{name:'查看人物 陈叔',exact:true}).click();await page.getByRole('dialog').waitFor();await shot('detail');check('person exact story is retained',(await page.getByRole('dialog').innerText()).includes('修理铺'));await close(page);
  await page.getByRole('tab',{name:'关于你',exact:true}).click();await page.getByText('你想让她知道的事',{exact:true}).first().waitFor();await shot('about');check('about view reads owner background',!(await page.locator('.replica-people').innerText()).includes('资料范围不匹配'));
 });
 await run('feed-read-detail',async({page,check,shot})=>{
  await go(page,'/feed');await page.getByTestId('feed-post-window-rain').waitFor();await page.getByTestId('feed-post-window-rain').getByRole('button',{name:'查看动态详情',exact:true}).first().click();await page.getByRole('dialog').waitFor();await shot('detail');check('comments retain exact content',(await page.getByRole('dialog').innerText()).includes('路过看到你窗台上的花了'));await close(page);
 });
 await run('media-read-detail',async({page,check,shot})=>{
  await go(page,'/c/linwan/media');await page.getByRole('button',{name:'查看画面 林晚 · 规范参考',exact:true}).click();await page.getByRole('dialog').waitFor();await shot('detail');check('media detail uses actual saved image',await page.getByRole('dialog').locator('img').count()>0);await close(page);
 });
 await run('studio-resource-editor',async({page,check,shot})=>{
  await go(page,'/studio');await page.getByRole('button',{name:'打开资源 青岚小城',exact:true}).click();await page.waitForURL(/studio\/edit\//);await page.getByRole('heading',{name:'青岚小城',exact:true}).waitFor();await shot('world-editor');check('world background reads complete source',(await page.locator('body').innerText()).includes('世界背景'));
 });
 await run('inbox-read-sync',async({page,check,shot})=>{
  await page.request.get('http://127.0.0.1:4381/fixture/parity-reset');await go(page,'/inbox');await page.getByRole('button',{name:'全部标为已读',exact:true}).waitFor();await page.getByRole('button',{name:'全部标为已读',exact:true}).click();
  await page.waitForFunction(()=>document.querySelector('.replica-nav-count')===null);await shot('all-read');check('sidebar authoritative unread count cleared',await page.locator('.replica-nav-count').count()===0);
  await page.getByRole('tab',{name:/未读/}).click();await page.getByText('未读已经看完了',{exact:true}).waitFor();check('unread filter reflects confirmed writes',await page.locator('.replica-notice').count()===0);check('reading does not finish pending task',await page.getByText('日常角色照需要继续准备',{exact:true}).count()>0);
  await page.request.get('http://127.0.0.1:4381/fixture/parity-reset');
 });
}finally{await browser.close();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;
