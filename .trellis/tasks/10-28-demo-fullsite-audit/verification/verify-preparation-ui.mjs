import {createRequire} from 'node:module';
import assert from 'node:assert/strict';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core');
const root=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/preparation');await mkdir(root,{recursive:true});
const browser=await chromium.launch({channel:'chrome',headless:true}),results=[];
try{for(const [scenario,port] of [['wardrobe-failure',4383],['recovery-unknown',4384],['activation-unknown',4385],['pending',4386]]){
 const context=await browser.newContext({viewport:{width:390,height:844},locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'}),page=await context.newPage();page.setDefaultTimeout(10000);
 const record={scenario,origin:'http://127.0.0.1:'+port,kind:'production dist with existing creation HTTP fixture; no live Core/LLM claim',checks:[],writes:[],errors:[]};
 page.on('pageerror',e=>record.errors.push(e.message));page.on('request',request=>{if(request.method()==='POST'&&request.url().includes('/api/'))record.writes.push({url:request.url(),body:request.postDataJSON()});});
 const check=(label,value)=>{assert.ok(value,label);record.checks.push(label);};const shot=async state=>{const path=resolve(root,scenario+'-'+state+'.png');await page.screenshot({path});(record.screenshots||=[]).push(path);};
 const stages=async()=>page.locator('[data-testid^="prep-step-"]').evaluateAll(nodes=>nodes.map(e=>({id:e.getAttribute('data-testid'),state:e.getAttribute('data-state')})));
 try{
  await page.goto(record.origin+'/#/library/create',{waitUntil:'networkidle'});await page.getByRole('tab',{name:'日常生活世界',exact:true}).click();await page.getByRole('button',{name:'使用日常生活世界',exact:true}).click();await page.getByTestId('wizard-world-next').click();await page.getByRole('button',{name:/从空白开始/}).click();await page.getByLabel('角色名字',{exact:true}).fill('林晚');await page.getByTestId('wizard-analyze').click();await page.getByTestId('wizard-activate').click();
  if(scenario==='activation-unknown'){
   await page.getByRole('button',{name:'查询或重试原激活',exact:true}).waitFor();await shot('unknown');await page.getByRole('button',{name:'查询或重试原激活',exact:true}).click();
   const commands=record.writes.filter(w=>w.url.endsWith('/activate'));check('unknown activation uses same original command',commands.length===2&&JSON.stringify(commands[0].body)===JSON.stringify(commands[1].body));
  }
  await page.waitForFunction(()=>document.querySelectorAll('[data-testid^="prep-step-"]').length===7);record.initial=await stages();await shot('seven-stages');
  check('seven independent preparation rows render',record.initial.length===7);
  if(['wardrobe-failure','recovery-unknown'].includes(scenario)){
   await page.waitForFunction(()=>document.querySelector('[data-testid="prep-step-wardrobe"]')?.getAttribute('data-state')==='失败');check('later stage waits for failed wardrobe',(await stages()).some(s=>s.state==='等待前序恢复'));await shot('wardrobe-failed');await page.getByTestId('prep-step-wardrobe').getByRole('button',{name:'恢复此环节',exact:true}).click();
   if(scenario==='recovery-unknown'){
    await page.getByRole('button',{name:'核对原恢复提交',exact:true}).waitFor();await shot('original-recovery-retained');await page.getByRole('button',{name:'核对原恢复提交',exact:true}).click();
    const commands=record.writes.filter(w=>w.url.endsWith('/inventory/preparation'));check('unknown recovery reuses exact original body',commands.length===2&&JSON.stringify(commands[0].body)===JSON.stringify(commands[1].body));
   }
  }
  if(scenario==='pending'){
   await page.waitForFunction(()=>document.querySelector('[data-testid="prep-step-wardrobe"]')?.getAttribute('data-state')==='进行中');await page.getByRole('button',{name:'重新读取进度',exact:true}).click();check('progress reread does not fabricate completion',(await stages()).some(s=>s.state==='进行中'));await shot('running');
  }else{
   await page.waitForFunction(()=>[...document.querySelectorAll('[data-testid^="prep-step-"]')].every(e=>e.getAttribute('data-state')==='已完成'));check('completion comes from all seven read sources',(await stages()).every(s=>s.state==='已完成'));await shot('complete');
   await page.reload({waitUntil:'networkidle'});await page.waitForFunction(()=>document.querySelectorAll('[data-testid^="prep-step-"]').length===7);check('refresh retains same activation',record.writes.filter(w=>w.url.endsWith('/activate')).length===(scenario==='activation-unknown'?2:1));await shot('restored');
  }
  record.final=await stages();check('no browser exception',record.errors.length===0);check('no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));record.status='passed';
 }catch(e){record.status='failed';record.error=e.message;record.text=await page.locator('body').innerText().catch(()=>null);await shot('failure').catch(()=>{});}finally{results.push(record);console.log(scenario,record.status,record.error||record.checks.length);await writeFile(resolve(root,'preparation-ui.json'),JSON.stringify(results,null,2)+'\n');await context.close();}
}}finally{await browser.close();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;
