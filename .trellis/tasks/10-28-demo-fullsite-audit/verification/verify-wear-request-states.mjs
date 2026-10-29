import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {startOwnedFixture} from './owned-fixture.mjs';
import {recordFeedbackDOM} from './feedback-dom-audit.mjs';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json'),{chromium}=require('playwright-core');
const out=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/wear-request-states-'+(process.env.AUDIT_PHASE||'pass113'));
await mkdir(out,{recursive:true});const browser=await chromium.launch({channel:'chrome',headless:true}),results=[];
try{for(const [size,width,height] of [['desktop',1348,925],['mobile',390,844]])for(const name of ['failed','denied','no-change','unknown','readback-failure']){
  if(process.env.AUDIT_CASES&&!process.env.AUDIT_CASES.split(',').includes(name))continue;
  const server=await startOwnedFixture({port:Number(process.env.WEAR_STATES_PORT||4463),env:{FLUCTLIGHT_UI_FIXTURE_WARDROBE_WORLD:'1'}});let context,page;
  const record={name,size,width,height,boundary:'production dist / controlled request and wardrobe HTTP; simulated outcomes, no real model decision',checks:[],requests:[],errors:[],screenshots:[]};
  try{
    context=await browser.newContext({viewport:{width,height},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'});page=await context.newPage();page.setDefaultTimeout(12000);
    page.on('pageerror',e=>record.errors.push(e.message));page.on('request',r=>{const path=new URL(r.url()).pathname;if(path.startsWith('/api/'))record.requests.push({path,method:r.method(),body:r.postDataJSON()});});
    const api='/api/fluctlights/fluctlight-fixture-one',get=async path=>{const r=await context.request.get(server.base+path);assert.equal(r.status(),200);return r.json();};
    const check=(label,value)=>{assert.ok(value,label);record.checks.push(label);};
    const wearing=async()=>(await get(api+'/wardrobe')).worn_items.map(i=>i.id).sort();
    const shot=async state=>{await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));});check(state+': no horizontal overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));const file=resolve(out,name+'-'+size+'-'+state+'.png');await page.screenshot({path:file,animations:'disabled'});record.screenshots.push(file);await recordFeedbackDOM(page,record,state);};
    const before=await wearing();let admission,terminal=false,firstUnknown=true,readbackFailed=false;
    await page.route('**'+api+'/inventory/requests',async route=>{
      const response=await route.fetch();admission=await response.json();
      if(name==='unknown'&&firstUnknown){firstUnknown=false;await route.fulfill({status:503,json:{code:'request_failed',message:'受控换装已受理但回执丢失'}});}else await route.fulfill({response});
    });
    await page.route('**/api/owner-requests/request-wardrobe-fixture',async route=>{
      if(['failed','denied','no-change'].includes(name)){
        terminal=true;await route.fulfill({json:{...admission.request,kind:'inventory_outfit',payload:{outfit_id:admission.request.payload.outfitId},status:name==='no-change'?'completed':name==='denied'?'rejected':'failed',decision_reason:name==='denied'?'本次换装未被采纳，继续保留当前穿着。':name==='failed'?'本次换装没有确认提交结果。':'',result:{disposition:name==='no-change'?'no_change':'not_applied',mutation_committed:false,error_code:name==='failed'?'owner_life_request_not_applied':undefined}}});
      }else{
        // Advance this controlled worker to its real fixture terminal without
        // waiting 3s polls. The fixture itself commits the selected member set.
        let response=await route.fetch(),value=await response.json();
        for(let n=0;n<2&&value.status==='pending';n++){response=await route.fetch();value=await response.json();}
        terminal=value.status==='completed';await route.fulfill({response});
      }
    });
    if(name==='readback-failure')await page.route('**'+api+'/wardrobe',async route=>{if(terminal&&!readbackFailed){readbackFailed=true;await route.fulfill({status:503,json:{code:'request_failed',message:'受控终态后穿着首次读取失败'}});}else await route.continue();});
    await page.goto(server.base+'/#/c/fluctlight-fixture-one/life/wardrobe',{waitUntil:'networkidle'});
    await page.getByRole('tab',{name:/穿搭方案/}).click();await page.getByRole('button',{name:'搭配新方案',exact:true}).click();
    const planName='换装结果核验 '+name;await page.getByRole('textbox',{name:'方案名称',exact:true}).fill(planName);
    const members=name==='no-change'?['青绿棉质上衣','宽松长裤','柔软内衣','棉质内裤','短袜','平底鞋']:['薄外套','银色项链','简约手链'];
    for(const n of members)await page.getByRole('button',{name:n,exact:true}).click();
    const saved=page.waitForResponse(r=>new URL(r.url()).pathname===api+'/inventory/outfits'&&r.request().method()==='POST');await page.getByRole('button',{name:'保存为穿搭方案',exact:true}).click();assert.equal((await saved).status(),200);
    await page.getByRole('tab',{name:/穿搭方案/}).click();
    const card=()=>page.locator('.r-outfit-card').filter({has:page.getByRole('heading',{name:planName,exact:true})});
    await card().getByRole('button',{name:/^请.*换上/}).click();await page.getByRole('alertdialog').getByRole('button',{name:'发送换装请求',exact:true}).click();
    if(name==='unknown'){
      // Admission failure keeps the original confirmation open. Dismiss only
      // that local prompt before operating the card's original-command retry.
      await page.getByRole('alertdialog').getByRole('button',{name:'取消',exact:true}).click();
      await card().getByRole('button',{name:'重试',exact:true}).waitFor();await shot('unknown');
      await card().getByRole('button',{name:'重试',exact:true}).click();
    }
    if(name==='no-change'){
      await card().getByText('已经穿着这套搭配，本次无需换装。',{exact:true}).waitFor();assert.deepEqual(await wearing(),before);record.checks.push('no-change is distinct and does not fabricate a wearing mutation');
    }else if(name==='failed'||name==='denied'){
      await card().getByRole('alert').filter({hasText:name==='failed'?'本次换装没有确认提交结果。':'本次换装未被采纳，继续保留当前穿着。'}).waitFor();assert.deepEqual(await wearing(),before);record.checks.push('failed or rejected outcome retains confirmed wearing and reason');
    }else if(name==='readback-failure'){
      await card().getByRole('button',{name:'重新读取页面结果',exact:true}).waitFor();check('terminal readback failure retains known request result',terminal&&readbackFailed);await shot('readback-failure');
      const count=record.requests.filter(r=>r.path===api+'/inventory/requests'&&r.method==='POST').length;
      await card().getByRole('button',{name:'重新读取页面结果',exact:true}).click();await card().getByRole('button',{name:'重新读取页面结果',exact:true}).waitFor({state:'detached'});
      check('readback recovery uses GET only',record.requests.filter(r=>r.path===api+'/inventory/requests'&&r.method==='POST').length===count);
    }
    if(['unknown','readback-failure'].includes(name)){
      await page.waitForFunction(()=>['薄外套','银色项链','简约手链'].every(n=>document.querySelector('.r-wearing-strip')?.textContent.includes(n)));
      assert.deepEqual(await wearing(),['clothing-6','clothing-7','clothing-8']);record.checks.push('confirmed member set is actually read back after original recovery');
    }
    const commands=record.requests.filter(r=>r.path===api+'/inventory/requests'&&r.method==='POST');assert.equal(commands.length,name==='unknown'?2:1);
    if(name==='unknown'){assert.deepEqual(commands[0].body,commands[1].body);record.checks.push('unknown retry preserves original request body/key/outfit and wearing CAS');}
    const resultVisible=()=>['failed','denied'].includes(name)?card().getByRole('button',{name:'查看原请求',exact:true}):card().getByTestId('request-status');
    await resultVisible().waitFor();await shot('result');
    const writesBefore=commands.length;await page.reload({waitUntil:'networkidle'});await page.getByRole('tab',{name:/穿搭方案/}).click();await resultVisible().waitFor();
    check('reload retains terminal result without readmission',record.requests.filter(r=>r.path===api+'/inventory/requests'&&r.method==='POST').length===writesBefore);await shot('refreshed');check('no browser exception',record.errors.length===0);record.status='passed';
  }catch(e){record.status='failed';record.error=e.stack;record.text=await page?.locator('body').innerText().catch(()=>null);await page?.screenshot({path:resolve(out,name+'-'+size+'-failure.png'),animations:'disabled'}).catch(()=>{});}
  finally{try{await context?.close();}finally{await server.stop();}}
  results.push(record);await writeFile(resolve(out,'report.json'),JSON.stringify(results,null,2)+'\n');console.log(name,size,record.status,record.error||record.checks.length);
}}finally{await browser.close();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;
