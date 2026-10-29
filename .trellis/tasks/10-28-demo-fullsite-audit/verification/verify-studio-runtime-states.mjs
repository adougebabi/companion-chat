// Actual production UI; each viewport/scenario owns a fresh in-memory server.
// Does not contact Core/PG/Worker/model. Never attach to an existing listener.
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import net from 'node:net';
import {mkdir,writeFile} from 'node:fs/promises';
import {resolve} from 'node:path';
const require=createRequire('/private/tmp/fluctlight-demo-audit-20261028/browser-tools/package.json');
const {chromium}=require('playwright-core');
const port=Number(process.env.STUDIO_STATES_PORT||4457),base=`http://127.0.0.1:${port}`;
assert.ok(Number.isSafeInteger(port)&&port>=1024&&port<=65535);
const out=resolve('.trellis/tasks/10-28-demo-fullsite-audit/evidence/studio-runtime-states-'+(process.env.AUDIT_PHASE||'pass107'));
const fid='fluctlight-fixture-one',resource='studio-world';
const catalog='/api/fluctlights/'+fid+'/world/catalog',source='/api/studio/resources/'+resource;
await mkdir(out,{recursive:true});
async function stop(child){
  if(!child?.pid||child.exitCode!==null||child.signalCode!==null)return;
  const ended=once(child,'exit');child.kill('SIGTERM');await ended;
}
async function start(scenario){
  const probe=net.createServer();
  await new Promise((ok,no)=>{probe.once('error',no);probe.listen(port,'127.0.0.1',ok);});
  await new Promise(ok=>probe.close(ok));
  const child=spawn(process.execPath,['apps/web/test/fixtures/ui-quality-server.mjs'],{cwd:process.cwd(),env:{...Object.fromEntries(Object.entries(process.env).filter(([k])=>!k.startsWith('FLUCTLIGHT_UI_FIXTURE_'))),FLUCTLIGHT_UI_FIXTURE_PORT:String(port),FLUCTLIGHT_UI_FIXTURE_WORLD:'normal',FLUCTLIGHT_UI_FIXTURE_STUDIO:'runtime-'+scenario,FLUCTLIGHT_UI_FIXTURE_STUDIO_RUNTIME:'1'},stdio:['ignore','pipe','pipe']});
  try{
    await new Promise((ok,no)=>{
      let stdout='',stderr='';const timer=setTimeout(()=>no(Error('fixture startup timeout: '+stdout+stderr)),10000);
      const fail=e=>{clearTimeout(timer);no(e);};child.once('error',fail);child.once('exit',(code,signal)=>fail(Error('fixture exit '+(signal||code)+': '+stderr)));
      child.stderr.on('data',v=>{stderr+=v;});child.stdout.on('data',v=>{stdout+=v;if(stdout.split('\n').includes('Isolated UI fixture: '+base)){clearTimeout(timer);ok();}});
    });return child;
  }catch(e){await stop(child);throw e;}
}
const selected=process.env.AUDIT_CASES?.split(','),results=[];
const browser=await chromium.launch({channel:'chrome',headless:true});
try{for(const [size,width,height] of [['desktop',1348,925],['mobile',390,844]])for(const scenario of ['rejected','unknown','pending']){
  if(selected&&!selected.includes(scenario))continue;
  const child=await start(scenario);let context,page,releaseResponse;
  const record={scenario,size,width,height,dpr:1,boundary:'production dist / controlled HTTP runtime / no live Core, persistence or model claim',checks:[],requests:[],errors:[],screenshots:[]};
  const check=(label,value)=>{assert.ok(value,label);record.checks.push(label);};
  try{
    context=await browser.newContext({viewport:{width,height},deviceScaleFactor:1,locale:'zh-CN',timezoneId:'Asia/Shanghai',reducedMotion:'reduce'});
    page=await context.newPage();page.setDefaultTimeout(10000);
    page.on('pageerror',e=>record.errors.push(e.message));
    page.on('request',r=>{const path=new URL(r.url()).pathname;if(path.startsWith('/api/'))record.requests.push({path,method:r.method(),body:r.postDataJSON()});});
    const get=async path=>{const response=await context.request.get(base+path);assert.equal(response.status(),200,path);return response.json();};
    const dialog=()=>page.getByRole('dialog').last();
    const writes=()=>record.requests.filter(r=>r.path===source+'/apply'&&r.method==='POST');
    const shot=async(label,modal=true)=>{
      await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(document.getAnimations().filter(a=>a.effect?.getComputedTiming().iterations!==Infinity).map(a=>a.finished.catch(()=>{})));});
      check(label+': no document overflow',await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
      if(modal){const b=await dialog().boundingBox();check(label+': bounded dialog',b&&b.x>=0&&b.y>=0&&b.x+b.width<=width+.5&&b.y+b.height<=height+.5);}
      const file=resolve(out,scenario+'-'+size+'-'+label+'.png');await page.screenshot({path:file,animations:'disabled'});record.screenshots.push(file);
    };
    const before=await get(catalog),otherBefore=await get('/api/fluctlights/fluctlight-fixture-two/world/catalog');
    const sourceBefore=await get(source),publication=await get(source+'/versions/draft%3A1');
    await page.goto(base+'/#/studio/edit/'+resource,{waitUntil:'networkidle'});
    await page.getByRole('button',{name:'应用到角色',exact:true}).click();
    await dialog().getByRole('combobox',{name:'目标角色',exact:true}).click();
    await page.getByRole('option').filter({hasText:'林岚'}).click();
    const previewResponse=page.waitForResponse(r=>new URL(r.url()).pathname===source+'/apply-preview');
    await dialog().getByRole('button',{name:'读取应用差异',exact:true}).click();
    const preview=await(await previewResponse).json();check('fresh preview creates inactive runtime from exact publication',preview.create_required===true&&preview.runtime_revision===0);
    let pendingReady,pendingDone;
    if(scenario==='pending'){
      // Keep the real delayed server response undelivered until its state has
      // been inspected. This is transport gating, never a synthetic success.
      const gate=new Promise(ok=>{releaseResponse=ok;});
      pendingReady=new Promise(ok=>{pendingDone=ok;});
      await page.route('**'+source+'/apply',async route=>{const response=await route.fetch();pendingDone(response.status());await gate;await route.fulfill({response});});
      record.transport='fixture delays receipt 1200ms; browser delivery additionally gated for deterministic pending screenshot';
    }
    let responsePromise=page.waitForResponse(r=>new URL(r.url()).pathname===source+'/apply'&&r.request().method()==='POST');
    await dialog().getByRole('button',{name:'确认应用所选字段',exact:true}).click();
    if(scenario==='pending'){
      assert.equal(await pendingReady,200);
      check('pending disables duplicate confirmation',await dialog().getByRole('button',{name:'确认应用所选字段',exact:true}).isDisabled());
      check('pending disables target and preview',await dialog().getByRole('combobox',{name:'目标角色',exact:true}).isDisabled()&&await dialog().getByRole('button',{name:'读取应用差异',exact:true}).isDisabled());
      check('pending has no receipt or target result action',await page.getByRole('heading',{name:'原应用回执已核对',exact:true}).count()===0&&await page.getByRole('button',{name:'查看角色中的结果',exact:true}).count()===0);
      const pendingCatalog=await get(catalog),rows=pendingCatalog.items.filter(r=>r.source_resource_id===resource);
      check('actual controlled target exists once before receipt delivery',rows.length===1&&rows[0].revision===1&&!rows[0].active);
      record.pendingRuntimeId=rows[0].id;await shot('pending');releaseResponse();releaseResponse=null;
    }
    let response=await responsePromise;
    assert.equal(response.status(),scenario==='rejected'?422:scenario==='unknown'?503:200);
    if(scenario==='rejected'){
      await dialog().getByRole('alert').waitFor();
      check('explicit rejection never shows acknowledged success',await page.getByRole('heading',{name:'原应用回执已核对',exact:true}).count()===0);
      assert.deepEqual(await get(catalog),before);assert.deepEqual((await get(source)).referenced_by,sourceBefore.referenced_by);record.checks.push('rejected command leaves catalogue and source references unchanged');
      check('rejected preview remains available',await dialog().getByRole('button',{name:'确认应用所选字段',exact:true}).isEnabled());
      await shot('rejected');await dialog().getByRole('button',{name:'取消',exact:true}).click();
      check('known rejection releases unknown recovery identity',await page.getByRole('button',{name:'重试原提交',exact:true}).count()===0);
      await page.reload({waitUntil:'networkidle'});assert.deepEqual(await get(catalog),before);
    }else{
      if(scenario==='unknown'){
        await dialog().getByRole('alert').waitFor();
        check('lost receipt does not show success',await page.getByRole('heading',{name:'原应用回执已核对',exact:true}).count()===0);
        const unknownRows=(await get(catalog)).items.filter(r=>r.source_resource_id===resource);
        check('unknown committed once before retry',unknownRows.length===1&&unknownRows[0].revision===1);record.unknownRuntimeId=unknownRows[0].id;
        await shot('unknown');await dialog().getByRole('button',{name:'取消',exact:true}).click();
        await page.getByRole('button',{name:'重试原提交',exact:true}).waitFor();
        await page.getByRole('textbox',{name:'资源名称',exact:true}).fill('回执未知之后的新草稿');
        await page.reload({waitUntil:'networkidle'});
        check('new draft survives unknown remount',await page.getByRole('textbox',{name:'资源名称',exact:true}).inputValue()==='回执未知之后的新草稿');
        await shot('recovery',false);
        responsePromise=page.waitForResponse(r=>new URL(r.url()).pathname===source+'/apply'&&r.request().method()==='POST');
        await page.getByRole('button',{name:'重试原提交',exact:true}).click();response=await responsePromise;assert.equal(response.status(),200);
        await page.getByText('已确认原应用回执，请到目标记录核对当前值。',{exact:true}).waitFor();
        check('new draft survives original acknowledgement',await page.getByRole('textbox',{name:'资源名称',exact:true}).inputValue()==='回执未知之后的新草稿');
        assert.equal(writes().length,2);assert.deepEqual(writes()[0].body,writes()[1].body);record.checks.push('retry preserves exact original body/key/CAS/choices despite newer draft');
      }
      const receipt=await response.json();
      if(scenario==='unknown'){
        // Recovery is on the editor after remount. Opening a NEW application
        // deliberately clears lastApplication; follow its actual source link.
        await page.getByRole('link',{name:'查看实际记录',exact:true}).waitFor();await shot('receipt',false);
      }else{
        await dialog().getByRole('heading',{name:'原应用回执已核对',exact:true}).waitFor();await shot('receipt');
      }
      const after=await get(catalog),rows=after.items.filter(r=>r.source_resource_id===resource);
      check('one original runtime and unchanged active binding',rows.length===1&&!rows[0].active&&JSON.stringify(after.active_binding)===JSON.stringify(before.active_binding));
      const actual=await get(catalog+'/'+receipt.id);
      check('exact applied snapshot remains revision one',rows[0].id===receipt.id&&actual.world.revision===1&&actual.world.source_version===publication.version&&actual.world.background===publication.body.background);
      check('framework created once',actual.places.filter(p=>p.name==='书房').length===1);
      check('source references same runtime',(await get(source)).referenced_by.some(ref=>ref.fluctlight_id===fid&&ref.runtime_id===receipt.id&&ref.running_version===publication.version));
      check('receipt refers to originally observed target',receipt.id===(record.pendingRuntimeId||record.unknownRuntimeId));
      if(scenario==='unknown')await page.getByRole('link',{name:'查看实际记录',exact:true}).click();
      else await dialog().getByRole('button',{name:'查看角色中的结果',exact:true}).click();
      await page.waitForURL('**/#/c/'+fid+'/worlds');
      const card=page.getByTestId('world-card-'+receipt.id);await card.waitFor();
      await card.getByRole('button',{name:'进入世界 '+publication.name,exact:true}).click();
      await page.getByRole('tab',{name:'世界设定',exact:true}).click();
      await page.getByRole('tabpanel',{name:'世界设定',exact:true}).getByText(publication.body.background,{exact:true}).waitFor();await shot('target',false);
      await page.reload({waitUntil:'networkidle'});await page.getByTestId('world-card-'+receipt.id).waitFor();
      check('target survives actual page refresh',!!(await get(catalog)).items.find(r=>r.id===receipt.id));
    }
    check('other role unaffected',JSON.stringify(otherBefore)===JSON.stringify(await get('/api/fluctlights/fluctlight-fixture-two/world/catalog')));
    check('expected apply count only',writes().length===(scenario==='unknown'?2:1));check('no browser exception',record.errors.length===0);record.status='passed';
  }catch(e){record.status='failed';record.error=e.stack;record.text=await page?.locator('body').innerText().catch(()=>null);if(page)await page.screenshot({path:resolve(out,scenario+'-'+size+'-failure.png'),animations:'disabled'}).catch(()=>{});}
  finally{releaseResponse?.();try{await context?.close();}finally{await stop(child);}}
  results.push(record);await writeFile(resolve(out,'report.json'),JSON.stringify(results,null,2)+'\n');console.log(scenario,size,record.status,record.error||record.checks.length);
}}finally{await browser.close();}
if(results.some(r=>r.status!=='passed'))process.exitCode=1;
